package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/repository/activitystore"
	"log/slog"
	"math"
	"strconv"
	"time"
)

var activityShanghai = time.FixedZone("Asia/Shanghai", 8*60*60)

var ErrActivityIneligible = errors.New("activity requires a successful billed API call")
var ErrActivityDisabled = errors.New("activity is disabled")
var ErrActivityWalletEmpty = errors.New("no activity reward balance available to withdraw")
var ErrActivityNoReward = errors.New("当前签到奖励额度已用完，未占用今日签到，请在额度恢复后重试")
var ErrActivityReviewBlocked = errors.New("rewards are awaiting refund or settlement review")

type ActivityService struct {
	worker       *activityWorker
	db           *sql.DB
	billingCache *BillingCacheService
	userService  *UserService
}

func NewActivityService(db *sql.DB, billingCache *BillingCacheService, userService *UserService) *ActivityService {
	return &ActivityService{db: db, billingCache: billingCache, userService: userService}
}

type ActivityWalletLedgerItem struct {
	ID               int64     `json:"id"`
	Direction        string    `json:"direction"`
	Kind             string    `json:"kind"`
	AmountCents      int64     `json:"amount_cents"`
	WalletAfterCents int64     `json:"wallet_after_cents"`
	UserBalanceAfter string    `json:"user_balance_after,omitempty"`
	CreatedAt        time.Time `json:"created_at"`
}

type ActivityWalletSummary struct {
	Visible             bool                       `json:"visible"`
	AvailableCents      int64                      `json:"available_cents"`
	Available           float64                    `json:"available"`
	ReservedCents       int64                      `json:"reserved_cents"`
	Reserved            float64                    `json:"reserved"`
	TotalGiftCents      int64                      `json:"total_gift_cents"`
	TotalGift           float64                    `json:"total_gift"`
	TotalEarnedCents    int64                      `json:"total_earned_cents"`
	TotalEarned         float64                    `json:"total_earned"`
	TotalWithdrawnCents int64                      `json:"total_withdrawn_cents"`
	TotalWithdrawn      float64                    `json:"total_withdrawn"`
	UnabsorbedGift      string                     `json:"unabsorbed_gift"`
	RecoveryDebtCents   int64                      `json:"recovery_debt_cents"`
	WithdrawBlockReason string                     `json:"withdraw_block_reason,omitempty"`
	WithdrawBlocked     bool                       `json:"withdraw_blocked"`
	WithdrawnCents      int64                      `json:"withdrawn_cents,omitempty"`
	Withdrawn           float64                    `json:"withdrawn,omitempty"`
	UserBalanceAfter    string                     `json:"user_balance_after,omitempty"`
	Ledger              []ActivityWalletLedgerItem `json:"ledger,omitempty"`
}

type ActivityStatus struct {
	ServerDay              string     `json:"server_day"`
	CheckinBlockReason     string     `json:"checkin_block_reason,omitempty"`
	NextRewardCents        int        `json:"next_reward_cents"`
	TodayRewardCents       int        `json:"today_reward_cents"`
	CheckinVisible         bool       `json:"checkin_visible"`
	CheckinEligible        bool       `json:"checkin_eligible"`
	CheckinRequireRecharge bool       `json:"checkin_require_recharge"`
	CheckinHasRecharge     bool       `json:"checkin_has_recharge"`
	CheckinMinSpend        float64    `json:"checkin_min_spend"`
	Eligible               bool       `json:"eligible"`
	FirstCallAt            *time.Time `json:"first_call_at,omitempty"`
	CheckinEnabled         bool       `json:"checkin_enabled"`
	CheckedToday           bool       `json:"checked_today"`
	DailyReward            float64    `json:"daily_reward"`
	MilestoneReward        float64    `json:"milestone_reward"`
	Streak                 int        `json:"streak"`
	NextMilestone          int        `json:"next_milestone"`
	MonthReward            float64    `json:"month_reward"`
	MonthCap               float64    `json:"month_cap"`
	CheckedDays            []string   `json:"checked_days"`
	WalletBalanceCents     int64      `json:"wallet_balance_cents"`
	WalletBalance          float64    `json:"wallet_balance"`
	WalletTotalEarned      float64    `json:"wallet_total_earned"`
	WalletTotalWithdrawn   float64    `json:"wallet_total_withdrawn"`
	WithdrawBlocked        bool       `json:"withdraw_blocked"`
	WithdrawBlockReason    string     `json:"withdraw_block_reason,omitempty"`
	SecondEnabled          bool       `json:"second_recharge_enabled"`
	SecondEligible         bool       `json:"second_recharge_eligible"`
	SecondClaimed          bool       `json:"second_recharge_claimed"`
	SecondOffer            bool       `json:"second_recharge_offer"`
	SecondReward           float64    `json:"second_recharge_reward"`
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

func activityRemainingUserCap(reward int, userCap, userSpent int64) int {
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
	first, err = activitystore.FirstEligibleUsageAt(ctx, s.db, userID)
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil, nil
	}
	if err != nil {
		return false, nil, err
	}
	return true, &first, nil
}

func (s *ActivityService) loadWalletSummary(ctx context.Context, userID int64, includeLedger bool) (ActivityWalletSummary, error) {
	summary := ActivityWalletSummary{UnabsorbedGift: "0", Ledger: []ActivityWalletLedgerItem{}}
	err := s.db.QueryRowContext(ctx, `SELECT available_cents,total_earned_cents,total_withdrawn_cents,unabsorbed_gift::text,recovery_debt_cents FROM activity_wallets WHERE user_id=$1`, userID).
		Scan(&summary.AvailableCents, &summary.TotalEarnedCents, &summary.TotalWithdrawnCents, &summary.UnabsorbedGift, &summary.RecoveryDebtCents)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return summary, err
	}
	var audienceVisible bool
	if err := s.db.QueryRowContext(ctx, `SELECT activity_reserved_cents($1), activity_audience_allowed('checkin',$1) OR activity_audience_allowed('consumption',$1) OR activity_audience_allowed('lottery',$1)`, userID).
		Scan(&summary.ReservedCents, &audienceVisible); err != nil {
		return summary, err
	}
	summary.Available = float64(summary.AvailableCents) / 100
	summary.Reserved = float64(summary.ReservedCents) / 100
	summary.TotalGiftCents = summary.AvailableCents + summary.ReservedCents
	summary.TotalGift = float64(summary.TotalGiftCents) / 100
	summary.TotalEarned = float64(summary.TotalEarnedCents) / 100
	summary.TotalWithdrawn = float64(summary.TotalWithdrawnCents) / 100
	summary.Visible = audienceVisible || summary.AvailableCents > 0 || summary.ReservedCents > 0 || summary.RecoveryDebtCents > 0

	var awardsEnabled bool
	if err := s.db.QueryRowContext(ctx, `SELECT awards_enabled FROM activity_runtime WHERE id=1`).Scan(&awardsEnabled); err != nil {
		return summary, err
	}
	reviewBlocked, err := activitystore.WalletReviewBlocked(ctx, s.db, userID)
	if err != nil {
		return summary, err
	}
	summary.WithdrawBlocked = !awardsEnabled || reviewBlocked || summary.RecoveryDebtCents > 0
	switch {
	case !awardsEnabled:
		summary.WithdrawBlockReason = "awards_paused"
	case summary.RecoveryDebtCents > 0:
		summary.WithdrawBlockReason = "recovery_debt"
	case reviewBlocked:
		summary.WithdrawBlockReason = "refund_review"
	}

	if includeLedger {
		rows, err := s.db.QueryContext(ctx, `SELECT id,direction,kind,amount_cents,wallet_after_cents,COALESCE(user_balance_after::text,''),created_at FROM activity_wallet_ledger WHERE user_id=$1 ORDER BY id DESC LIMIT 20`, userID)
		if err != nil {
			return summary, err
		}
		defer rows.Close()
		for rows.Next() {
			var item ActivityWalletLedgerItem
			if err := rows.Scan(&item.ID, &item.Direction, &item.Kind, &item.AmountCents, &item.WalletAfterCents, &item.UserBalanceAfter, &item.CreatedAt); err != nil {
				return summary, err
			}
			summary.Ledger = append(summary.Ledger, item)
		}
		if err := rows.Err(); err != nil {
			return summary, err
		}
	}
	return summary, nil
}

func (s *ActivityService) WalletStatus(ctx context.Context, userID int64) (*ActivityWalletSummary, error) {
	summary, err := s.loadWalletSummary(ctx, userID, true)
	if err != nil {
		return nil, err
	}
	return &summary, nil
}

func creditActivityWalletTx(ctx context.Context, tx *sql.Tx, userID int64, kind, sourceID string, amountCents int64) error {
	if amountCents <= 0 {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO activity_reward_ledger(user_id,kind,source_id,amount_cents) VALUES($1,$2,$3,$4)`, userID, kind, sourceID, amountCents); err != nil {
		return err
	}
	var walletAfter int64
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO activity_wallets (user_id, available_cents, total_earned_cents, updated_at)
		VALUES ($1, $2, $2, NOW())
		ON CONFLICT (user_id) DO UPDATE
		SET available_cents = activity_wallets.available_cents + EXCLUDED.available_cents,
		    total_earned_cents = activity_wallets.total_earned_cents + EXCLUDED.total_earned_cents,
		    updated_at = NOW()
		RETURNING available_cents
	`, userID, amountCents).Scan(&walletAfter); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `
		INSERT INTO activity_wallet_ledger (user_id, direction, kind, source_id, amount_cents, wallet_after_cents)
		VALUES ($1, 'credit', $2, $3, $4, $5)
	`, userID, kind, sourceID, amountCents, walletAfter)
	return err
}

// syncUserRefundReviewsTx performs a read-only check on payment_orders for real
// in-flight or completed refunds (excluding REFUND_FAILED, where no principal left).
func syncUserRefundReviewsTx(ctx context.Context, tx *sql.Tx, userID int64) error {
	orders, err := activitystore.RefundOrdersForReview(ctx, tx, userID)
	if err != nil {
		return err
	}

	flagged := false
	for _, o := range orders {
		var changed bool
		err := tx.QueryRowContext(ctx, `
			INSERT INTO activity_refund_reviews (order_id, user_id, order_status, refund_amount, review_status, updated_at)
			VALUES ($1, $2, $3, $4::numeric, 'pending', NOW())
			ON CONFLICT (order_id) DO UPDATE
			SET order_status = EXCLUDED.order_status,
			    refund_amount = EXCLUDED.refund_amount,
			    review_status = 'pending',
			    updated_at = NOW()
			WHERE activity_refund_reviews.order_status IS DISTINCT FROM EXCLUDED.order_status OR activity_refund_reviews.refund_amount IS DISTINCT FROM EXCLUDED.refund_amount
			RETURNING TRUE
		`, o.ID, userID, o.Status, o.RefundAmount).Scan(&changed)
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return err
		}
		if err == nil && changed {
			flagged = true
		}
	}
	if flagged {
		if _, err := tx.ExecContext(ctx, `UPDATE activity_consumption_entitlements SET state='review' WHERE user_id=$1 AND state='reserved'`, userID); err != nil {
			return err
		}
	}
	return nil
}

func (s *ActivityService) WithdrawWallet(ctx context.Context, userID int64) (*ActivityWalletSummary, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if err := activitystore.LockWallet(ctx, tx, userID); err != nil {
		return nil, err
	}

	if err := activityAwardsAllowed(ctx, tx); err != nil {
		return nil, err
	}
	if err := syncUserRefundReviewsTx(ctx, tx, userID); err != nil {
		return nil, err
	}
	blocked, err := activitystore.WalletReviewBlocked(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	var debt int64
	if err := tx.QueryRowContext(ctx, `SELECT recovery_debt_cents FROM activity_wallets WHERE user_id=$1`, userID).Scan(&debt); err != nil {
		return nil, err
	}
	if blocked || debt > 0 {
		return nil, ErrActivityReviewBlocked
	}

	var availableCents int64
	err = tx.QueryRowContext(ctx, `SELECT available_cents FROM activity_wallets WHERE user_id=$1 FOR UPDATE`, userID).Scan(&availableCents)
	if errors.Is(err, sql.ErrNoRows) || availableCents <= 0 {
		if err != nil && !errors.Is(err, sql.ErrNoRows) {
			return nil, err
		}
		return nil, ErrActivityWalletEmpty
	}
	if err != nil {
		return nil, err
	}

	newBalance, ledgerID, err := activitystore.TransferWallet(ctx, tx, userID, availableCents)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrActivityIneligible
	}
	if err != nil {
		return nil, err
	}

	if err := consumptionAudit(ctx, tx, userID, "wallet_withdraw", ledgerID, map[string]any{
		"amount_cents":       availableCents,
		"user_balance_after": newBalance,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}

	s.invalidateBalance(ctx, userID)

	summary, err := s.loadWalletSummary(ctx, userID, true)
	if err != nil {
		return nil, err
	}
	summary.WithdrawnCents = availableCents
	summary.Withdrawn = float64(availableCents) / 100
	summary.UserBalanceAfter = newBalance
	return &summary, nil
}

func (s *ActivityService) Status(ctx context.Context, userID int64) (*ActivityStatus, error) {
	var audienceMode string
	var visible bool
	if err := s.db.QueryRowContext(ctx, `SELECT mode, mode='all' OR $1=ANY(user_ids) FROM activity_audiences WHERE kind='checkin'`, userID).Scan(&audienceMode, &visible); err != nil {
		return nil, err
	}
	if !visible {
		return &ActivityStatus{CheckedDays: []string{}}, nil
	}

	eligible, first, err := s.eligible(ctx, userID)
	if err != nil {
		return nil, err
	}
	day, month := activityDay(time.Now())
	out := &ActivityStatus{ServerDay: day, CheckinVisible: true, Eligible: eligible, FirstCallAt: first, CheckedDays: []string{}}

	wallet, err := s.loadWalletSummary(ctx, userID, false)
	if err != nil {
		return nil, err
	}
	out.WalletBalanceCents = wallet.AvailableCents
	out.WalletBalance = wallet.Available
	out.WalletTotalEarned = wallet.TotalEarned
	out.WalletTotalWithdrawn = wallet.TotalWithdrawn
	out.WithdrawBlocked = wallet.WithdrawBlocked
	out.WithdrawBlockReason = wallet.WithdrawBlockReason

	var cap, dailyCents, milestoneCents int
	var checkinRollout int
	var minSpend, monthlyBudget int64
	var secondReward int
	var secondBudget int64
	var userCap int64
	err = s.db.QueryRowContext(ctx, `SELECT checkin_enabled,checkin_require_recharge,checkin_rollout_percent,second_recharge_enabled,checkin_daily_cents,checkin_milestone_cents,checkin_monthly_cap_cents,second_recharge_reward_cents,second_recharge_total_budget_cents,user_lifetime_cap_cents,checkin_min_spend_cents,checkin_monthly_budget_cents FROM activity_config WHERE id=1`).Scan(&out.CheckinEnabled, &out.CheckinRequireRecharge, &checkinRollout, &out.SecondEnabled, &dailyCents, &milestoneCents, &cap, &secondReward, &secondBudget, &userCap, &minSpend, &monthlyBudget)
	if err != nil {
		return nil, err
	}
	out.DailyReward = float64(dailyCents) / 100
	out.MilestoneReward = float64(milestoneCents) / 100
	var awardsEnabled bool
	if err = s.db.QueryRowContext(ctx, `SELECT awards_enabled FROM activity_runtime WHERE id=1`).Scan(&awardsEnabled); err != nil {
		return nil, err
	}
	out.CheckinEnabled = out.CheckinEnabled && awardsEnabled
	out.SecondEnabled = out.SecondEnabled && awardsEnabled
	out.CheckinMinSpend = float64(minSpend) / 100
	out.CheckinHasRecharge, err = checkinRechargeEligible(ctx, s.db, userID, out.CheckinRequireRecharge)
	if err != nil {
		return nil, err
	}
	spendEligible, err := checkinSpendEligible(ctx, s.db, userID, minSpend)
	if err != nil {
		return nil, err
	}
	out.CheckinEligible = out.CheckinHasRecharge && spendEligible
	// Allowlisted test users in 'selected' mode bypass percentage rollout modulo checks.
	if audienceMode == "all" {
		out.CheckinEnabled = out.CheckinEnabled && int(userID%100) < checkinRollout
	}
	cap, err = s.effectiveCheckinCap(ctx, cap)
	if err != nil {
		return nil, err
	}
	out.MonthCap = float64(cap) / 100
	if out.SecondEnabled {
		var spent, userSpent int64
		if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0) FROM activity_reward_ledger WHERE kind='second_recharge'`).Scan(&spent); err != nil {
			return nil, err
		}
		if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0)+activity_reserved_cents($1) FROM activity_reward_ledger WHERE user_id=$1 AND kind<>'lottery'`, userID).Scan(&userSpent); err != nil {
			return nil, err
		}
		out.SecondEnabled = secondReward > 0 && spent+int64(secondReward) <= secondBudget && userSpent+int64(secondReward) <= userCap
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
			out.TodayRewardCents = cents
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
	var siteSpent, personalSpent int64
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0) FROM activity_reward_ledger WHERE kind='checkin' AND to_char(created_at AT TIME ZONE 'Asia/Shanghai','YYYY-MM')=$1`, month).Scan(&siteSpent); err != nil {
		return nil, err
	}
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0)+activity_reserved_cents($1) FROM activity_reward_ledger WHERE user_id=$1 AND kind<>'lottery'`, userID).Scan(&personalSpent); err != nil {
		return nil, err
	}
	out.NextRewardCents = activityRemainingUserCap(calculateCheckinReward(out.Streak+1, dailyCents, milestoneCents, cap, int(math.Round(out.MonthReward*100)), monthlyBudget, siteSpent), userCap, personalSpent)
	switch {
	case !out.CheckinEnabled:
		out.CheckinBlockReason = "disabled"
	case out.CheckedToday:
		out.CheckinBlockReason = "checked"
	case wallet.WithdrawBlocked:
		out.CheckinBlockReason = "refund_review"
		if wallet.WithdrawBlockReason == "awards_paused" {
			out.CheckinBlockReason = "awards_paused"
		}
		out.CheckinEligible = false
	case !out.CheckinHasRecharge:
		out.CheckinBlockReason = "recharge_required"
	case !spendEligible:
		out.CheckinBlockReason = "spend_required"
	case personalSpent >= userCap:
		out.CheckinBlockReason = "user_cap"
	case out.MonthReward*100 >= float64(cap):
		out.CheckinBlockReason = "monthly_cap"
	case siteSpent >= monthlyBudget:
		out.CheckinBlockReason = "budget_exhausted"
	case out.NextRewardCents <= 0:
		out.CheckinBlockReason = "no_reward"
	}
	if !out.CheckedToday && out.NextRewardCents <= 0 {
		out.CheckinEligible = false
	}

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

	if err := activitystore.LockWallet(ctx, tx, userID); err != nil {
		return nil, err
	}

	active, err := activitystore.UserActive(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	if !active {
		return nil, ErrActivityIneligible
	}
	audienceMode, err := activityAudienceModeAllowed(ctx, tx, "checkin", userID)
	if err != nil {
		return nil, err
	}
	if err := activityAwardsAllowed(ctx, tx); err != nil {
		return nil, err
	}
	if err := syncUserRefundReviewsTx(ctx, tx, userID); err != nil {
		return nil, err
	}
	blocked, err := activitystore.CheckinReviewBlocked(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	if blocked {
		return nil, ErrActivityReviewBlocked
	}

	var enabled, requireRecharge bool
	var daily, milestone, cap int
	var rollout int
	var budget int64
	var minSpend int64
	var userCap int64
	if err := tx.QueryRowContext(ctx, `SELECT checkin_enabled,checkin_require_recharge,checkin_rollout_percent,checkin_daily_cents,checkin_milestone_cents,checkin_monthly_cap_cents,checkin_monthly_budget_cents,user_lifetime_cap_cents,checkin_min_spend_cents FROM activity_config WHERE id=1 FOR UPDATE`).Scan(&enabled, &requireRecharge, &rollout, &daily, &milestone, &cap, &budget, &userCap, &minSpend); err != nil {
		return nil, err
	}
	if !enabled || (audienceMode == "all" && int(userID%100) >= rollout) {
		return nil, ErrActivityDisabled
	}
	var rawMultiplier string
	multiplier := defaultBalanceRechargeMultiplier
	err = tx.QueryRowContext(ctx, `SELECT value FROM settings WHERE key=$1`, SettingBalanceRechargeMult).Scan(&rawMultiplier)
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
	hasRecharge, err := checkinRechargeEligible(ctx, tx, userID, requireRecharge)
	if err != nil {
		return nil, err
	}
	if !hasRecharge {
		return nil, ErrActivityIneligible
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
	var userSpent int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0)+activity_reserved_cents($1) FROM activity_reward_ledger WHERE user_id=$1 AND kind<>'lottery'`, userID).Scan(&userSpent); err != nil {
		return nil, err
	}
	reward = activityRemainingUserCap(reward, userCap, userSpent)
	if reward <= 0 {
		return nil, ErrActivityNoReward
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO activity_checkins(user_id,local_day,streak,reward_cents) VALUES($1,$2::date,$3,$4)`, userID, day, streak, reward); err != nil {
		return nil, err
	}
	if reward > 0 {
		if err := creditActivityWalletTx(ctx, tx, userID, "checkin", fmt.Sprintf("%d:%s", userID, day), int64(reward)); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.Status(ctx, userID)
}

func checkinRechargeEligible(ctx context.Context, db activitystore.Tx, userID int64, requireRecharge bool) (bool, error) {
	if !requireRecharge {
		return true, nil
	}
	return activitystore.HasCompletedRecharge(ctx, db, userID)
}

func checkinSpendEligible(ctx context.Context, db activitystore.Tx, userID, minimumCents int64) (bool, error) {
	if minimumCents == 0 {
		return true, nil
	}
	return activitystore.EligibleSpendAtLeast(ctx, db, userID, minimumCents)
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
	orders, err := activitystore.PaidBalanceOrders(ctx, s.db, userID, 3)
	if err != nil {
		return nil, err
	}
	return convertActivityPaidOrders(orders), nil
}

func convertActivityPaidOrders(orders []activitystore.PaidOrder) []activityPaidOrder {
	out := make([]activityPaidOrder, 0, len(orders))
	for _, order := range orders {
		out = append(out, activityPaidOrder{
			id:       order.ID,
			payCents: order.PayCents,
			paidAt:   order.PaidAt,
			status:   order.Status,
			currency: order.Currency,
		})
	}
	return out
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
	return activitystore.EligibleUsageBetween(ctx, s.db, userID, first.paidAt, second.paidAt)
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
	return activitystore.EligibleUsageAfter(ctx, s.db, userID, first.paidAt)
}

type ActivityConfig struct {
	CheckinRequireRecharge         bool  `json:"checkin_require_recharge"`
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
	_ = s.SyncRefundReviews(ctx)
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

func (s *ActivityService) MarkRefundReviewed(ctx context.Context, orderID int64, actors ...int64) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var uid int64
	if err := tx.QueryRowContext(ctx, `SELECT user_id FROM activity_refund_reviews WHERE order_id=$1 AND review_status='pending'`, orderID).Scan(&uid); err != nil {
		return err
	}
	if err := activitystore.LockWallet(ctx, tx, uid); err != nil {
		return err
	}
	if err := settleUserRefundRewardsTx(ctx, tx, uid); err != nil {
		return err
	}
	inflight, err := activitystore.HasPendingBalanceRefund(ctx, tx, uid)
	if err != nil {
		return err
	}
	if inflight {
		return ErrActivityReviewBlocked
	}
	if err := collectActivityRecoveryDebt(ctx, tx, uid); err != nil {
		return err
	}
	var negative bool
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(delta_tickets),0)<0 FROM activity_lottery_ticket_ledger WHERE user_id=$1`, uid).Scan(&negative); err != nil {
		return err
	}
	if negative {
		return ErrActivityRecoveryReview
	}
	if _, err := tx.ExecContext(ctx, `UPDATE activity_refund_reviews SET review_status='reviewed',updated_at=NOW() WHERE order_id=$1`, orderID); err != nil {
		return err
	}
	actor := int64(0)
	if len(actors) > 0 {
		actor = actors[0]
	}
	if err := consumptionAudit(ctx, tx, actor, "refund_review_settled", orderID, map[string]any{"user_id": uid}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.invalidateBalance(ctx, uid)
	return nil
}

func (s *ActivityService) Config(ctx context.Context) (*ActivityConfig, error) {
	c := &ActivityConfig{}
	err := s.db.QueryRowContext(ctx, `SELECT checkin_enabled,checkin_require_recharge,checkin_rollout_percent,second_recharge_enabled,checkin_daily_cents,checkin_milestone_cents,checkin_monthly_cap_cents,checkin_monthly_budget_cents,second_recharge_reward_cents,second_recharge_total_budget_cents,second_recharge_min_pay_cents,total_budget_cents,user_lifetime_cap_cents,checkin_min_spend_cents FROM activity_config WHERE id=1`).Scan(&c.CheckinEnabled, &c.CheckinRequireRecharge, &c.CheckinRolloutPercent, &c.SecondRechargeEnabled, &c.CheckinDailyCents, &c.CheckinMilestoneCents, &c.CheckinMonthlyCapCents, &c.CheckinMonthlyBudgetCents, &c.SecondRechargeRewardCents, &c.SecondRechargeTotalBudgetCents, &c.SecondRechargeMinPayCents, &c.TotalBudgetCents, &c.UserLifetimeCapCents, &c.CheckinMinSpendCents)
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
	if (c.CheckinEnabled || c.SecondRechargeEnabled) && c.UserLifetimeCapCents <= 0 {
		return fmt.Errorf("user lifetime cap is required before enabling")
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
	var maximumUserCommitment int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(MAX(spent),0) FROM (SELECT user_id,SUM(amount) spent FROM (SELECT user_id,amount_cents::bigint amount FROM activity_reward_ledger WHERE kind<>'lottery' UNION ALL SELECT e.user_id,e.reward_cents::bigint FROM activity_consumption_entitlements e JOIN activity_consumption_campaigns c ON c.id=e.campaign_id WHERE e.state IN ('reserved','review') AND e.claimed_at IS NULL AND (e.state='review' OR c.claim_until>NOW())) commitments GROUP BY user_id) x`).Scan(&maximumUserCommitment); err != nil {
		return err
	}
	if c.UserLifetimeCapCents < maximumUserCommitment {
		return fmt.Errorf("user cap cannot be lower than existing spent and reserved rewards")
	}
	_, err = tx.ExecContext(ctx, `UPDATE activity_config SET checkin_enabled=$1,checkin_require_recharge=$2,checkin_rollout_percent=$3,second_recharge_enabled=$4,checkin_daily_cents=$5,checkin_milestone_cents=$6,checkin_monthly_cap_cents=$7,checkin_monthly_budget_cents=$8,second_recharge_reward_cents=$9,second_recharge_total_budget_cents=$10,second_recharge_min_pay_cents=$11,total_budget_cents=$12,user_lifetime_cap_cents=$13,checkin_min_spend_cents=$14,updated_at=NOW() WHERE id=1`, c.CheckinEnabled, c.CheckinRequireRecharge, c.CheckinRolloutPercent, c.SecondRechargeEnabled, c.CheckinDailyCents, c.CheckinMilestoneCents, c.CheckinMonthlyCapCents, c.CheckinMonthlyBudgetCents, c.SecondRechargeRewardCents, c.SecondRechargeTotalBudgetCents, c.SecondRechargeMinPayCents, c.TotalBudgetCents, c.UserLifetimeCapCents, c.CheckinMinSpendCents)
	if err != nil {
		return err
	}
	return tx.Commit()
}

func (s *ActivityService) ClaimSecondRecharge(ctx context.Context, userID int64) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `INSERT INTO activity_wallets(user_id) VALUES($1) ON CONFLICT DO NOTHING`, userID); err != nil {
		return false, err
	}
	var lockedUser int64
	if err := tx.QueryRowContext(ctx, `SELECT user_id FROM activity_wallets WHERE user_id=$1 FOR UPDATE`, userID).Scan(&lockedUser); err != nil {
		return false, err
	}
	active, err := activitystore.UserActive(ctx, tx, userID)
	if err != nil {
		return false, err
	}
	if !active {
		return false, ErrActivityIneligible
	}
	if err := activityAwardsAllowed(ctx, tx); err != nil {
		return false, err
	}
	var enabled bool
	var reward int
	var budget int64
	var minPay int
	var userCap int64
	if err = tx.QueryRowContext(ctx, `SELECT second_recharge_enabled,second_recharge_reward_cents,second_recharge_total_budget_cents,second_recharge_min_pay_cents,user_lifetime_cap_cents FROM activity_config WHERE id=1 FOR UPDATE`).Scan(&enabled, &reward, &budget, &minPay, &userCap); err != nil {
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
	paidOrders, err := activitystore.PaidBalanceOrders(ctx, tx, userID, 3)
	if err != nil {
		return false, err
	}
	orders := convertActivityPaidOrders(paidOrders)
	if len(orders) < 2 {
		return false, ErrActivityIneligible
	}
	first, second := orders[0], orders[1]
	if first.status != "COMPLETED" || second.status != "COMPLETED" || first.currency != "CNY" || second.currency != "CNY" || first.payCents < minPay || second.payCents < minPay || second.paidAt.Sub(first.paidAt) > 30*24*time.Hour {
		return false, ErrActivityIneligible
	}
	used, err := activitystore.EligibleUsageBetween(ctx, tx, userID, first.paidAt, second.paidAt)
	if err != nil {
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
	var userSpent int64
	if err = tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0)+activity_reserved_cents($1) FROM activity_reward_ledger WHERE user_id=$1 AND kind<>'lottery'`, userID).Scan(&userSpent); err != nil {
		return false, err
	}
	if userSpent+int64(reward) > userCap {
		return false, ErrActivityDisabled
	}
	if err := creditActivityWalletTx(ctx, tx, userID, "second_recharge", fmt.Sprintf("%d", orders[1].id), int64(reward)); err != nil {
		return false, err
	}
	if err = tx.Commit(); err != nil {
		return false, err
	}
	return true, nil
}
