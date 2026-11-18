package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"math"
	"strconv"
	"time"
)

var activityShanghai = time.FixedZone("Asia/Shanghai", 8*60*60)

var ErrActivityIneligible = errors.New("activity requires a successful billed API call")
var ErrActivityDisabled = errors.New("activity is disabled")

type ActivityService struct {
	db           *sql.DB
	billingCache *BillingCacheService
	userService  *UserService
}

func NewActivityService(db *sql.DB, billingCache *BillingCacheService, userService *UserService) *ActivityService {
	return &ActivityService{db: db, billingCache: billingCache, userService: userService}
}

type ActivityStatus struct {
	CheckinEligible bool       `json:"checkin_eligible"`
	CheckinMinSpend float64    `json:"checkin_min_spend"`
	Eligible        bool       `json:"eligible"`
	FirstCallAt     *time.Time `json:"first_call_at,omitempty"`
	CheckinEnabled  bool       `json:"checkin_enabled"`
	CheckedToday    bool       `json:"checked_today"`
	Streak          int        `json:"streak"`
	NextMilestone   int        `json:"next_milestone"`
	MonthReward     float64    `json:"month_reward"`
	MonthCap        float64    `json:"month_cap"`
	CheckedDays     []string   `json:"checked_days"`
	SecondEnabled   bool       `json:"second_recharge_enabled"`
	SecondEligible  bool       `json:"second_recharge_eligible"`
	SecondClaimed   bool       `json:"second_recharge_claimed"`
	SecondOffer     bool       `json:"second_recharge_offer"`
	SecondReward    float64    `json:"second_recharge_reward"`
}

func activityDay(now time.Time) (string, string) {
	local := now.In(activityShanghai)
	return local.Format("2006-01-02"), local.Format("2006-01")
}

func activityCheckinCap(configured int, multiplier float64) int {
	limit := int(math.Floor(1500 * normalizeBalanceRechargeMultiplier(multiplier)))
	if limit < configured {
		return limit
	}
	return configured
}

func activityRemainingBudget(reward int, total, spent, userCap, userSpent int64) int {
	if int64(reward) > total-spent {
		reward = int(total - spent)
	}
	if int64(reward) > userCap-userSpent {
		reward = int(userCap - userSpent)
	}
	if reward < 0 {
		return 0
	}
	return reward
}

func (s *ActivityService) effectiveCheckinCap(ctx context.Context, configured int) (int, error) {
	var raw string
	err := s.db.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=$1`, SettingBalanceRechargeMult).Scan(&raw)
	if errors.Is(err, sql.ErrNoRows) {
		return activityCheckinCap(configured, defaultBalanceRechargeMultiplier), nil
	}
	if err != nil {
		return 0, err
	}
	multiplier, err := strconv.ParseFloat(raw, 64)
	if err != nil {
		return 0, err
	}
	return activityCheckinCap(configured, multiplier), nil
}

func (s *ActivityService) eligible(ctx context.Context, userID int64) (bool, *time.Time, error) {
	var first time.Time
	err := s.db.QueryRowContext(ctx, `SELECT occurred_at FROM activity_first_calls WHERE user_id=$1`, userID).Scan(&first)
	if err == nil {
		return true, &first, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return false, nil, err
	}
	// Historical non-cyber billed usage makes existing users eligible, without
	// backdating the post-launch first-call event or its conversion metric.
	err = s.db.QueryRowContext(ctx, `SELECT created_at FROM usage_logs WHERE user_id=$1 AND actual_cost>0 AND COALESCE(request_type,0) <> 4 ORDER BY created_at LIMIT 1`, userID).Scan(&first)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}
	return true, &first, nil
}

func (s *ActivityService) Status(ctx context.Context, userID int64) (*ActivityStatus, error) {
	eligible, first, err := s.eligible(ctx, userID)
	if err != nil {
		return nil, err
	}
	day, month := activityDay(time.Now())
	out := &ActivityStatus{Eligible: eligible, FirstCallAt: first, CheckedDays: []string{}}
	var cap int
	var checkinRollout int
	var minSpend int64
	var secondReward int
	var secondBudget int64
	var totalBudget, userCap int64
	err = s.db.QueryRowContext(ctx, `SELECT checkin_enabled,checkin_rollout_percent,second_recharge_enabled,checkin_monthly_cap_cents,second_recharge_reward_cents,second_recharge_total_budget_cents,total_budget_cents,user_lifetime_cap_cents,checkin_min_spend_cents FROM activity_config WHERE id=1`).Scan(&out.CheckinEnabled, &checkinRollout, &out.SecondEnabled, &cap, &secondReward, &secondBudget, &totalBudget, &userCap, &minSpend)
	if err != nil {
		return nil, err
	}
	out.CheckinMinSpend = float64(minSpend) / 100
	out.CheckinEligible, err = checkinSpendEligible(ctx, s.db, userID, minSpend)
	if err != nil {
		return nil, err
	}
	out.CheckinEnabled = out.CheckinEnabled && int(userID%100) < checkinRollout
	cap, err = s.effectiveCheckinCap(ctx, cap)
	if err != nil {
		return nil, err
	}
	out.MonthCap = float64(cap) / 100
	if out.SecondEnabled {
		var spent, totalSpent, userSpent int64
		if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0) FROM activity_reward_ledger WHERE kind='second_recharge'`).Scan(&spent); err != nil {
			return nil, err
		}
		if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0)+activity_reserved_cents() FROM activity_reward_ledger`).Scan(&totalSpent); err != nil {
			return nil, err
		}
		if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0)+activity_reserved_cents($1) FROM activity_reward_ledger WHERE user_id=$1`, userID).Scan(&userSpent); err != nil {
			return nil, err
		}
		out.SecondEnabled = secondReward > 0 && spent+int64(secondReward) <= secondBudget && totalSpent+int64(secondReward) <= totalBudget && userSpent+int64(secondReward) <= userCap
		out.SecondReward = float64(secondReward) / 100
	}
	rows, err := s.db.QueryContext(ctx, `SELECT local_day::text, streak, reward_cents FROM activity_checkins WHERE user_id=$1 AND to_char(local_day,'YYYY-MM')=$2 ORDER BY local_day`, userID, month)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var d string
		var streak, cents int
		if err := rows.Scan(&d, &streak, &cents); err != nil {
			return nil, err
		}
		out.CheckedDays = append(out.CheckedDays, d)
		out.MonthReward += float64(cents) / 100
		if d == day {
			out.CheckedToday = true
			out.Streak = streak
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return nil, err
	}
	rows.Close()
	if !out.CheckedToday {
		_ = s.db.QueryRowContext(ctx, `SELECT streak FROM activity_checkins WHERE user_id=$1 AND local_day=($2::date - 1)`, userID, day).Scan(&out.Streak)
	}
	out.NextMilestone = (out.Streak/7 + 1) * 7
	if eligible && out.SecondEnabled {
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM activity_reward_ledger WHERE user_id=$1 AND kind='second_recharge')`, userID).Scan(&out.SecondClaimed); err != nil {
			return nil, err
		}
		out.SecondEligible, err = s.secondEligible(ctx, userID)
		if err != nil {
			return nil, err
		}
		if !out.SecondClaimed && !out.SecondEligible {
			out.SecondOffer, err = s.secondOffer(ctx, userID)
			if err != nil {
				return nil, err
			}
		}
	}
	return out, nil
}

func (s *ActivityService) Checkin(ctx context.Context, userID int64) (*ActivityStatus, error) {
	day, month := activityDay(time.Now())
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var active bool
	if err := tx.QueryRowContext(ctx, `SELECT status='active' FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, userID).Scan(&active); err != nil {
		return nil, err
	}
	if !active {
		return nil, ErrActivityIneligible
	}
	var enabled bool
	var daily, milestone, cap int
	var rollout int
	var budget int64
	var minSpend int64
	var totalBudget, userCap int64
	if err := tx.QueryRowContext(ctx, `SELECT checkin_enabled,checkin_rollout_percent,checkin_daily_cents,checkin_milestone_cents,checkin_monthly_cap_cents,checkin_monthly_budget_cents,total_budget_cents,user_lifetime_cap_cents,checkin_min_spend_cents FROM activity_config WHERE id=1 FOR UPDATE`).Scan(&enabled, &rollout, &daily, &milestone, &cap, &budget, &totalBudget, &userCap, &minSpend); err != nil {
		return nil, err
	}
	if !enabled || int(userID%100) >= rollout {
		return nil, ErrActivityDisabled
	}
	var rawMultiplier string
	multiplier := defaultBalanceRechargeMultiplier
	err = tx.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=$1 FOR SHARE`, SettingBalanceRechargeMult).Scan(&rawMultiplier)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		multiplier, err = strconv.ParseFloat(rawMultiplier, 64)
		if err != nil {
			return nil, err
		}
	}
	cap = activityCheckinCap(cap, multiplier)
	var already bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM activity_checkins WHERE user_id=$1 AND local_day=$2::date)`, userID, day).Scan(&already); err != nil {
		return nil, err
	}
	if already {
		_ = tx.Rollback()
		return s.Status(ctx, userID)
	}
	eligible, err := checkinSpendEligible(ctx, tx, userID, minSpend)
	if err != nil {
		return nil, err
	}
	if !eligible {
		return nil, ErrActivityIneligible
	}
	var streak int
	_ = tx.QueryRowContext(ctx, `SELECT streak FROM activity_checkins WHERE user_id=$1 AND local_day=($2::date - 1)`, userID, day).Scan(&streak)
	streak++
	var spent int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(reward_cents),0) FROM activity_checkins WHERE user_id=$1 AND to_char(local_day,'YYYY-MM')=$2`, userID, month).Scan(&spent); err != nil {
		return nil, err
	}
	var globalSpent int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0) FROM activity_reward_ledger WHERE kind='checkin' AND to_char(created_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM')=$1`, month).Scan(&globalSpent); err != nil {
		return nil, err
	}
	reward := calculateCheckinReward(streak, daily, milestone, cap, spent, budget, globalSpent)
	var totalSpent, userSpent int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0)+activity_reserved_cents() FROM activity_reward_ledger`).Scan(&totalSpent); err != nil {
		return nil, err
	}
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0)+activity_reserved_cents($1) FROM activity_reward_ledger WHERE user_id=$1`, userID).Scan(&userSpent); err != nil {
		return nil, err
	}
	reward = activityRemainingBudget(reward, totalBudget, totalSpent, userCap, userSpent)
	if _, err := tx.ExecContext(ctx, `INSERT INTO activity_checkins(user_id,local_day,streak,reward_cents) VALUES($1,$2::date,$3,$4)`, userID, day, streak, reward); err != nil {
		return nil, err
	}
	if reward > 0 {
		if _, err := tx.ExecContext(ctx, `INSERT INTO activity_reward_ledger(user_id,kind,source_id,amount_cents) VALUES($1,'checkin',$2,$3)`, userID, fmt.Sprintf("%d:%s", userID, day), reward); err != nil {
			return nil, err
		}
		if _, err := tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1::numeric/100,updated_at=NOW() WHERE id=$2`, reward, userID); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	if reward > 0 {
		s.invalidateBalance(ctx, userID)
	}
	return s.Status(ctx, userID)
}

// Use the same exact numeric comparison for status and the reward transaction.
// Cyber requests do not contribute. Zero explicitly removes the usage requirement.
type activityQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func checkinSpendEligible(ctx context.Context, db activityQueryer, userID, minimumCents int64) (bool, error) {
	if minimumCents == 0 {
		return true, nil
	}
	var eligible bool
	err := db.QueryRowContext(ctx, `SELECT COALESCE(SUM(actual_cost),0) >= $2::numeric/100 FROM usage_logs WHERE user_id=$1 AND actual_cost>0 AND COALESCE(request_type,0)<>4`, userID, minimumCents).Scan(&eligible)
	return eligible, err
}

func calculateCheckinReward(streak, daily, milestone, cap, spent int, budget, globalSpent int64) int {
	reward := daily
	if streak%7 == 0 {
		reward += milestone
	}
	if reward > cap-spent {
		reward = cap - spent
	}
	if int64(reward) > budget-globalSpent {
		reward = int(budget - globalSpent)
	}
	if reward < 0 {
		return 0
	}
	return reward
}

func (s *ActivityService) invalidateBalance(ctx context.Context, userID int64) {
	if s.userService != nil && s.userService.authCacheInvalidator != nil {
		s.userService.authCacheInvalidator.InvalidateAuthCacheByUserID(ctx, userID)
	}
	if s.billingCache != nil {
		if err := s.billingCache.InvalidateUserBalance(ctx, userID); err != nil {
			slog.Error("activity balance cache invalidation failed", "user_id", userID, "error", err)
		}
	}
}

type activityPaidOrder struct {
	id       int64
	payCents int
	paidAt   time.Time
	status   string
	currency string
}

func (s *ActivityService) secondOrders(ctx context.Context, userID int64) ([]activityPaidOrder, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,ROUND(pay_amount*100)::int,COALESCE(paid_at,completed_at),status,
		COALESCE(NULLIF(provider_snapshot->>'currency',''),'CNY')
		FROM payment_orders WHERE user_id=$1 AND order_type='balance' AND paid_at IS NOT NULL
		ORDER BY paid_at,id LIMIT 3`, userID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	orders := make([]activityPaidOrder, 0, 3)
	for rows.Next() {
		var order activityPaidOrder
		if err := rows.Scan(&order.id, &order.payCents, &order.paidAt, &order.status, &order.currency); err != nil {
			return nil, err
		}
		orders = append(orders, order)
	}
	return orders, rows.Err()
}

func (s *ActivityService) secondEligible(ctx context.Context, userID int64) (bool, error) {
	if userID%2 != 0 {
		return false, nil
	}
	orders, err := s.secondOrders(ctx, userID)
	if err != nil || len(orders) < 2 {
		return false, err
	}
	first, second := orders[0], orders[1]
	var minPay int
	if err := s.db.QueryRowContext(ctx, `SELECT second_recharge_min_pay_cents FROM activity_config WHERE id=1`).Scan(&minPay); err != nil {
		return false, err
	}
	if first.status != "COMPLETED" || second.status != "COMPLETED" || first.currency != "CNY" || second.currency != "CNY" || first.payCents < minPay || second.payCents < minPay || second.paidAt.Sub(first.paidAt) > 30*24*time.Hour {
		return false, nil
	}
	var used bool
	err = s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM usage_logs WHERE user_id=$1 AND actual_cost>0 AND COALESCE(request_type,0) <> 4 AND created_at>$2 AND created_at<$3)`, userID, first.paidAt, second.paidAt).Scan(&used)
	return used, err
}

func (s *ActivityService) secondOffer(ctx context.Context, userID int64) (bool, error) {
	if userID%2 != 0 {
		return false, nil
	}
	orders, err := s.secondOrders(ctx, userID)
	if err != nil || len(orders) != 1 {
		return false, err
	}
	var minPay int
	if err = s.db.QueryRowContext(ctx, `SELECT second_recharge_min_pay_cents FROM activity_config WHERE id=1`).Scan(&minPay); err != nil {
		return false, err
	}
	first := orders[0]
	if first.status != "COMPLETED" || first.currency != "CNY" || first.payCents < minPay || time.Since(first.paidAt) > 30*24*time.Hour {
		return false, nil
	}
	var used bool
	err = s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM usage_logs WHERE user_id=$1 AND actual_cost>0 AND COALESCE(request_type,0)<>4 AND created_at>$2)`, userID, first.paidAt).Scan(&used)
	return used, err
}

type ActivityConfig struct {
	CheckinMinSpendCents           int64 `json:"checkin_min_spend_cents"`
	CheckinEnabled                 bool  `json:"checkin_enabled"`
	CheckinRolloutPercent          int   `json:"checkin_rollout_percent"`
	SecondRechargeEnabled          bool  `json:"second_recharge_enabled"`
	CheckinDailyCents              int   `json:"checkin_daily_cents"`
	CheckinMilestoneCents          int   `json:"checkin_milestone_cents"`
	CheckinMonthlyCapCents         int   `json:"checkin_monthly_cap_cents"`
	CheckinMonthlyBudgetCents      int64 `json:"checkin_monthly_budget_cents"`
	SecondRechargeRewardCents      int   `json:"second_recharge_reward_cents"`
	SecondRechargeTotalBudgetCents int64 `json:"second_recharge_total_budget_cents"`
	SecondRechargeMinPayCents      int   `json:"second_recharge_min_pay_cents"`
	TotalBudgetCents               int64 `json:"total_budget_cents"`
	UserLifetimeCapCents           int64 `json:"user_lifetime_cap_cents"`
	TotalSpentCents                int64 `json:"total_spent_cents"`
	CheckinMonthSpentCents         int64 `json:"checkin_month_spent_cents"`
	SecondTotalSpentCents          int64 `json:"second_total_spent_cents"`
}

type ActivityRefundReview struct {
	OrderID      int64     `json:"order_id"`
	UserID       int64     `json:"user_id"`
	OrderStatus  string    `json:"order_status"`
	ReviewStatus string    `json:"review_status"`
	CreatedAt    time.Time `json:"created_at"`
}

func (s *ActivityService) RefundReviews(ctx context.Context) ([]ActivityRefundReview, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT order_id,user_id,order_status,review_status,created_at FROM activity_refund_reviews WHERE review_status='pending' ORDER BY created_at DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := make([]ActivityRefundReview, 0)
	for rows.Next() {
		var review ActivityRefundReview
		if err := rows.Scan(&review.OrderID, &review.UserID, &review.OrderStatus, &review.ReviewStatus, &review.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, review)
	}
	return out, rows.Err()
}

func (s *ActivityService) MarkRefundReviewed(ctx context.Context, orderID int64) error {
	result, err := s.db.ExecContext(ctx, `UPDATE activity_refund_reviews SET review_status='reviewed',updated_at=NOW() WHERE order_id=$1 AND review_status='pending'`, orderID)
	if err != nil {
		return err
	}
	count, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if count == 0 {
		return fmt.Errorf("refund review not found")
	}
	return nil
}

func (s *ActivityService) Config(ctx context.Context) (*ActivityConfig, error) {
	c := &ActivityConfig{}
	err := s.db.QueryRowContext(ctx, `SELECT checkin_enabled,checkin_rollout_percent,second_recharge_enabled,checkin_daily_cents,checkin_milestone_cents,checkin_monthly_cap_cents,checkin_monthly_budget_cents,second_recharge_reward_cents,second_recharge_total_budget_cents,second_recharge_min_pay_cents,total_budget_cents,user_lifetime_cap_cents,checkin_min_spend_cents FROM activity_config WHERE id=1`).Scan(&c.CheckinEnabled, &c.CheckinRolloutPercent, &c.SecondRechargeEnabled, &c.CheckinDailyCents, &c.CheckinMilestoneCents, &c.CheckinMonthlyCapCents, &c.CheckinMonthlyBudgetCents, &c.SecondRechargeRewardCents, &c.SecondRechargeTotalBudgetCents, &c.SecondRechargeMinPayCents, &c.TotalBudgetCents, &c.UserLifetimeCapCents, &c.CheckinMinSpendCents)
	if err != nil {
		return nil, err
	}
	_, month := activityDay(time.Now())
	if err = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0) FROM activity_reward_ledger WHERE kind='checkin' AND to_char(created_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM')=$1`, month).Scan(&c.CheckinMonthSpentCents); err != nil {
		return nil, err
	}
	if err = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0) FROM activity_reward_ledger WHERE kind='second_recharge'`).Scan(&c.SecondTotalSpentCents); err != nil {
		return nil, err
	}
	if err = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0)+activity_reserved_cents() FROM activity_reward_ledger`).Scan(&c.TotalSpentCents); err != nil {
		return nil, err
	}
	return c, nil
}

func (s *ActivityService) UpdateConfig(ctx context.Context, c ActivityConfig) error {
	if c.SecondRechargeEnabled {
		return fmt.Errorf("second recharge rewards have been replaced by consumption campaigns")
	}
	if c.CheckinMinSpendCents < 0 || c.CheckinRolloutPercent < 0 || c.CheckinRolloutPercent > 100 || c.CheckinDailyCents < 0 || c.CheckinMilestoneCents < 0 || c.CheckinMonthlyCapCents < 0 || c.CheckinMonthlyCapCents > 1500 || c.CheckinMonthlyBudgetCents < 0 || c.SecondRechargeRewardCents < 0 || c.SecondRechargeTotalBudgetCents < 0 || c.SecondRechargeMinPayCents < 0 || c.TotalBudgetCents < 0 || c.UserLifetimeCapCents < 0 {
		return fmt.Errorf("invalid activity amounts")
	}
	if c.CheckinEnabled && (c.CheckinRolloutPercent <= 0 || c.CheckinMonthlyBudgetCents <= 0 || c.CheckinDailyCents <= 0 || c.CheckinMonthlyCapCents <= 0) {
		return fmt.Errorf("checkin reward and budget are required before enabling")
	}
	if c.SecondRechargeEnabled && (c.SecondRechargeRewardCents <= 0 || c.SecondRechargeTotalBudgetCents < int64(c.SecondRechargeRewardCents) || c.SecondRechargeMinPayCents <= 0) {
		return fmt.Errorf("second recharge reward, budget and minimum payment are required before enabling")
	}
	if (c.CheckinEnabled || c.SecondRechargeEnabled) && (c.TotalBudgetCents <= 0 || c.UserLifetimeCapCents <= 0) {
		return fmt.Errorf("total budget and user lifetime cap are required before enabling")
	}
	if c.SecondRechargeEnabled && int64(c.SecondRechargeRewardCents) > c.UserLifetimeCapCents {
		return fmt.Errorf("second reward exceeds user lifetime cap")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var lock int
	if err = tx.QueryRowContext(ctx, `SELECT id FROM activity_config WHERE id=1 FOR UPDATE`).Scan(&lock); err != nil {
		return err
	}
	var committed int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0)+activity_reserved_cents() FROM activity_reward_ledger`).Scan(&committed); err != nil {
		return err
	}
	if c.TotalBudgetCents < committed {
		return fmt.Errorf("total budget cannot be lower than spent and reserved rewards")
	}
	var maximumUserCommitment int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(spent),0) FROM (SELECT u.id,COALESCE(SUM(l.amount_cents),0)+activity_reserved_cents(u.id) spent FROM users u LEFT JOIN activity_reward_ledger l ON l.user_id=u.id GROUP BY u.id) x`).Scan(&maximumUserCommitment); err != nil {
		return err
	}
	if c.UserLifetimeCapCents < maximumUserCommitment {
		return fmt.Errorf("user cap cannot be lower than existing spent and reserved rewards")
	}
	_, err = tx.ExecContext(ctx, `UPDATE activity_config SET checkin_enabled=$1,checkin_rollout_percent=$2,second_recharge_enabled=$3,checkin_daily_cents=$4,checkin_milestone_cents=$5,checkin_monthly_cap_cents=$6,checkin_monthly_budget_cents=$7,second_recharge_reward_cents=$8,second_recharge_total_budget_cents=$9,second_recharge_min_pay_cents=$10,total_budget_cents=$11,user_lifetime_cap_cents=$12,checkin_min_spend_cents=$13,updated_at=NOW() WHERE id=1`, c.CheckinEnabled, c.CheckinRolloutPercent, c.SecondRechargeEnabled, c.CheckinDailyCents, c.CheckinMilestoneCents, c.CheckinMonthlyCapCents, c.CheckinMonthlyBudgetCents, c.SecondRechargeRewardCents, c.SecondRechargeTotalBudgetCents, c.SecondRechargeMinPayCents, c.TotalBudgetCents, c.UserLifetimeCapCents, c.CheckinMinSpendCents)
	if err != nil {
		return err
	}
	return tx.Commit()
}

// ClaimSecondRecharge checks the immutable second paid order and credits once.
// The handler may retry this after an interrupted payment callback.
func (s *ActivityService) ClaimSecondRecharge(ctx context.Context, userID int64) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var active bool
	if err = tx.QueryRowContext(ctx, `SELECT status='active' FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, userID).Scan(&active); err != nil {
		return false, err
	}
	if !active {
		return false, ErrActivityIneligible
	}
	var enabled bool
	var reward int
	var budget int64
	var minPay int
	var totalBudget, userCap int64
	if err = tx.QueryRowContext(ctx, `SELECT second_recharge_enabled,second_recharge_reward_cents,second_recharge_total_budget_cents,second_recharge_min_pay_cents,total_budget_cents,user_lifetime_cap_cents FROM activity_config WHERE id=1 FOR UPDATE`).Scan(&enabled, &reward, &budget, &minPay, &totalBudget, &userCap); err != nil {
		return false, err
	}
	if !enabled || reward <= 0 || budget <= 0 {
		return false, ErrActivityDisabled
	}
	var claimed bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM activity_reward_ledger WHERE user_id=$1 AND kind='second_recharge')`, userID).Scan(&claimed); err != nil {
		return false, err
	}
	if claimed {
		return false, nil
	}
	if userID%2 != 0 {
		return false, ErrActivityIneligible
	}
	rows, err := tx.QueryContext(ctx, `SELECT id,ROUND(pay_amount*100)::int,paid_at,status,COALESCE(NULLIF(provider_snapshot->>'currency',''),'CNY')
		FROM payment_orders WHERE user_id=$1 AND order_type='balance' AND paid_at IS NOT NULL
		ORDER BY paid_at,id LIMIT 3 FOR UPDATE`, userID)
	if err != nil {
		return false, err
	}
	orders := make([]activityPaidOrder, 0, 3)
	for rows.Next() {
		var o activityPaidOrder
		if err = rows.Scan(&o.id, &o.payCents, &o.paidAt, &o.status, &o.currency); err != nil {
			rows.Close()
			return false, err
		}
		orders = append(orders, o)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return false, err
	}
	if len(orders) < 2 {
		return false, ErrActivityIneligible
	}
	first, second := orders[0], orders[1]
	if first.status != "COMPLETED" || second.status != "COMPLETED" || first.currency != "CNY" || second.currency != "CNY" || first.payCents < minPay || second.payCents < minPay || second.paidAt.Sub(first.paidAt) > 30*24*time.Hour {
		return false, ErrActivityIneligible
	}
	var used bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM usage_logs WHERE user_id=$1 AND actual_cost>0 AND COALESCE(request_type,0)<>4 AND created_at>$2 AND created_at<$3)`, userID, first.paidAt, second.paidAt).Scan(&used); err != nil {
		return false, err
	}
	if !used {
		return false, ErrActivityIneligible
	}
	var spent int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0) FROM activity_reward_ledger WHERE kind='second_recharge'`).Scan(&spent); err != nil {
		return false, err
	}
	if spent+int64(reward) > budget {
		return false, ErrActivityDisabled
	}
	var totalSpent, userSpent int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0)+activity_reserved_cents() FROM activity_reward_ledger`).Scan(&totalSpent); err != nil {
		return false, err
	}
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0)+activity_reserved_cents($1) FROM activity_reward_ledger WHERE user_id=$1`, userID).Scan(&userSpent); err != nil {
		return false, err
	}
	if totalSpent+int64(reward) > totalBudget || userSpent+int64(reward) > userCap {
		return false, ErrActivityDisabled
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO activity_reward_ledger(user_id,kind,source_id,amount_cents) VALUES($1,'second_recharge',$2,$3)`, userID, fmt.Sprintf("%d", orders[1].id), reward); err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE users SET balance=balance+$1::numeric/100,updated_at=NOW() WHERE id=$2`, reward, userID); err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	s.invalidateBalance(ctx, userID)
	return true, nil
}
