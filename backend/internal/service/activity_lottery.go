package service

import (
	"context"
	"crypto/rand"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"math/big"
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

var (
	ErrLotteryInsufficientTickets     = errors.New("抽奖次数不足")
	ErrLotteryBudgetInsufficient      = errors.New("奖池补货中，您的抽奖次数已安全保留")
	ErrLotteryBatchBudgetInsufficient = errors.New("当前奖池剩余预算不足以支持十连抽，请先使用单抽，您的次数已安全保留")
	ErrLotteryInvalidDraw             = errors.New("无效的抽奖请求参数")
)

const lotteryTierCount = 8

type LotteryTier struct {
	Position    int     `json:"position"`
	RewardCents int     `json:"reward_cents"`
	Reward      float64 `json:"reward"`
	Weight      int     `json:"weight"`
}

type LotteryPublicTier struct {
	Probability float64 `json:"probability"`
	Position    int     `json:"position"`
	RewardCents int     `json:"reward_cents"`
	Reward      float64 `json:"reward"`
}

type LotteryStatus struct {
	Visible              bool                `json:"visible"`
	Enabled              bool                `json:"enabled"`
	AwardsEnabled        bool                `json:"awards_enabled"`
	UnitPayCents         int                 `json:"unit_pay_cents"`
	UnitPayYuan          float64             `json:"unit_pay_yuan"`
	TicketDebt           int                 `json:"ticket_debt"`
	RemainingDraws       int                 `json:"remaining_draws"`
	TotalWonCents        int64               `json:"total_won_cents"`
	TotalWon             float64             `json:"total_won"`
	TodayWonCents        int64               `json:"today_won_cents"`
	TodayWon             float64             `json:"today_won"`
	DrawBlocked          bool                `json:"draw_blocked"`
	PoolPaused           bool                `json:"pool_paused"`
	TenDrawPaused        bool                `json:"ten_draw_paused"`
	WalletBalanceCents   int64               `json:"wallet_balance_cents"`
	WalletBalance        float64             `json:"wallet_balance"`
	WalletTotalEarned    float64             `json:"wallet_total_earned"`
	WalletTotalWithdrawn float64             `json:"wallet_total_withdrawn"`
	WithdrawBlocked      bool                `json:"withdraw_blocked"`
	WithdrawBlockReason  string              `json:"withdraw_block_reason,omitempty"`
	Tiers                []LotteryPublicTier `json:"tiers"`
}

type LotteryDrawItem struct {
	ID           int64      `json:"id"`
	BatchKey     string     `json:"batch_key"`
	DrawCount    int        `json:"draw_count"`
	DrawIndex    int        `json:"draw_index"`
	TierPosition int        `json:"tier_position"`
	RewardCents  int        `json:"reward_cents"`
	Reward       float64    `json:"reward"`
	CreatedAt    time.Time  `json:"created_at"`
	RevokedAt    *time.Time `json:"revoked_at,omitempty"`
}

type LotteryDrawResponse struct {
	Items            []LotteryDrawItem `json:"items"`
	TotalRewardCents int64             `json:"total_reward_cents"`
	TotalReward      float64           `json:"total_reward"`
	Status           *LotteryStatus    `json:"status"`
}

type LotteryHistoryResponse struct {
	Items    []LotteryDrawItem `json:"items"`
	Total    int64             `json:"total"`
	Page     int               `json:"page"`
	PageSize int               `json:"page_size"`
}

type AdminLotteryConfig struct {
	Enabled                bool          `json:"enabled"`
	UnitPayCents           int           `json:"unit_pay_cents"`
	BudgetCents            int64         `json:"budget_cents"`
	SpentCents             int64         `json:"spent_cents"`
	OutstandingTickets     int64         `json:"outstanding_tickets"`
	ExpectedLiabilityCents float64       `json:"expected_liability_cents"`
	TiersLocked            bool          `json:"tiers_locked"`
	ExpectedCents          float64       `json:"expected_cents"`
	Tiers                  []LotteryTier `json:"tiers"`
}

type LotteryTicketLedgerItem struct {
	ID           int64     `json:"id"`
	SourceType   string    `json:"source_type"`
	SourceID     string    `json:"source_id"`
	OrderID      *int64    `json:"order_id,omitempty"`
	DeltaTickets int       `json:"delta_tickets"`
	CreatedAt    time.Time `json:"created_at"`
}

type AdminLotteryUserSummary struct {
	UserID         int64                     `json:"user_id"`
	NetTickets     int                       `json:"net_tickets"`
	RemainingDraws int                       `json:"remaining_draws"`
	TotalWonCents  int64                     `json:"total_won_cents"`
	TotalWon       float64                   `json:"total_won"`
	DrawBlocked    bool                      `json:"draw_blocked"`
	Wallet         ActivityWalletSummary     `json:"wallet"`
	Tickets        []LotteryTicketLedgerItem `json:"tickets"`
	Draws          []LotteryDrawItem         `json:"draws"`
}

func defaultLotteryTiers() []LotteryTier {
	raw := []LotteryTier{
		{Position: 1, RewardCents: 50, Weight: 8738},
		{Position: 2, RewardCents: 100, Weight: 961},
		{Position: 3, RewardCents: 200, Weight: 150},
		{Position: 4, RewardCents: 300, Weight: 100},
		{Position: 5, RewardCents: 500, Weight: 40},
		{Position: 6, RewardCents: 1000, Weight: 9},
		{Position: 7, RewardCents: 2000, Weight: 1},
		{Position: 8, RewardCents: 5000, Weight: 1},
	}
	for i := range raw {
		raw[i].Reward = float64(raw[i].RewardCents) / 100
	}
	return raw
}

func parseAndValidateLotteryTiers(tiers []LotteryTier) ([]LotteryTier, int, int64, float64, error) {
	if len(tiers) != lotteryTierCount {
		return nil, 0, 0, 0, fmt.Errorf("奖池必须固定包含 %d 个奖位", lotteryTierCount)
	}
	normalized := make([]LotteryTier, lotteryTierCount)
	var totalWeight int64
	var weightedSum float64
	maxReward := 0
	for i, t := range tiers {
		if t.Position != i+1 {
			return nil, 0, 0, 0, fmt.Errorf("奖位序号必须为 1 至 %d", lotteryTierCount)
		}
		if t.RewardCents <= 0 || t.RewardCents > 10_000_000 {
			return nil, 0, 0, 0, fmt.Errorf("第 %d 格奖金必须大于 0", t.Position)
		}
		if t.Weight < 0 || t.Weight > 1_000_000 {
			return nil, 0, 0, 0, fmt.Errorf("第 %d 格权重必须为 0 至 1000000 的整数，0 表示暂未投放", t.Position)
		}
		normalized[i] = LotteryTier{
			Position:    t.Position,
			RewardCents: t.RewardCents,
			Reward:      float64(t.RewardCents) / 100,
			Weight:      t.Weight,
		}
		totalWeight += int64(t.Weight)
		weightedSum += float64(t.RewardCents) * float64(t.Weight)
		if t.Weight > 0 && t.RewardCents > maxReward {
			maxReward = t.RewardCents
		}
	}
	if totalWeight <= 0 {
		return nil, 0, 0, 0, fmt.Errorf("总权重必须大于 0")
	}
	return normalized, maxReward, totalWeight, weightedSum / float64(totalWeight), nil
}

func calculateLotteryOrderTickets(payCents int64, unitPayCents int) int {
	if payCents <= 0 || unitPayCents <= 0 {
		return 0
	}
	n := payCents / int64(unitPayCents)
	if n > math.MaxInt32 {
		return math.MaxInt32
	}
	return int(n)
}

func calculateLotteryRefundRemainingTickets(status string, orderAmount, payAmount, refundAmount float64, unitPayCents, currentEffective int) (int, int64) {
	if currentEffective <= 0 || unitPayCents <= 0 {
		return 0, 0
	}
	payCents := decimal.NewFromFloat(payAmount).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
	if status == "REFUNDED" || orderAmount <= 0 || refundAmount >= orderAmount-0.01 {
		return 0, payCents
	}
	refundPay := calculateGatewayRefundAmount(orderAmount, payAmount, refundAmount, "CNY")
	refundPayCents := decimal.NewFromFloat(refundPay).Mul(decimal.NewFromInt(100)).Round(0).IntPart()
	if refundPayCents > payCents {
		refundPayCents = payCents
	}
	if refundPayCents < 0 {
		refundPayCents = 0
	}
	remainingCents := payCents - refundPayCents
	allowed := calculateLotteryOrderTickets(remainingCents, unitPayCents)
	if allowed > currentEffective {
		allowed = currentEffective
	}
	return allowed, refundPayCents
}

func pickWeightedLotteryTier(tiers []LotteryTier, totalWeight int64, randInt func(max int64) (int64, error)) (LotteryTier, error) {
	if len(tiers) == 0 || totalWeight <= 0 {
		return LotteryTier{}, fmt.Errorf("invalid lottery pool")
	}
	r, err := randInt(totalWeight)
	if err != nil {
		return LotteryTier{}, err
	}
	var cursor int64
	for _, tier := range tiers {
		cursor += int64(tier.Weight)
		if r < cursor {
			return tier, nil
		}
	}
	return tiers[len(tiers)-1], nil
}

func cryptoLotteryRandInt(max int64) (int64, error) {
	n, err := rand.Int(rand.Reader, big.NewInt(max))
	if err != nil {
		return 0, err
	}
	return n.Int64(), nil
}

type lotteryWindow struct {
	id           int64
	unitPayCents int
	openedAt     time.Time
	closedAt     sql.NullTime
}

func matchLotteryWindow(windows []lotteryWindow, settledAt time.Time) (int, bool) {
	for i := len(windows) - 1; i >= 0; i-- {
		w := windows[i]
		if settledAt.Before(w.openedAt) {
			continue
		}
		if w.closedAt.Valid && !settledAt.Before(w.closedAt.Time) {
			continue
		}
		return w.unitPayCents, true
	}
	return 0, false
}

func loadLotteryWindowsTx(ctx context.Context, tx *sql.Tx) ([]lotteryWindow, error) {
	rows, err := tx.QueryContext(ctx, `SELECT id, unit_pay_cents, opened_at, closed_at FROM activity_lottery_windows ORDER BY id ASC`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []lotteryWindow
	for rows.Next() {
		var w lotteryWindow
		if err := rows.Scan(&w.id, &w.unitPayCents, &w.openedAt, &w.closedAt); err != nil {
			return nil, err
		}
		out = append(out, w)
	}
	return out, rows.Err()
}

// syncUserLotteryTx idempotently grants tickets for completed CNY balance orders
// settled within active lottery windows, and deducts tickets for refunded orders.
func syncUserLotteryTx(ctx context.Context, tx *sql.Tx, userID int64) error {
	if err := syncUserRefundReviewsTx(ctx, tx, userID); err != nil {
		return err
	}

	var allowed bool
	if err := tx.QueryRowContext(ctx, `SELECT mode='all' OR $1=ANY(user_ids) FROM activity_audiences WHERE kind='lottery' FOR SHARE`, userID).Scan(&allowed); err != nil {
		return err
	}

	if allowed {
		windows, err := loadLotteryWindowsTx(ctx, tx)
		if err != nil {
			return err
		}
		if len(windows) > 0 {
			earliest := windows[0].openedAt
			rows, err := tx.QueryContext(ctx, `
				SELECT o.id, ROUND(o.pay_amount * 100)::bigint, COALESCE(o.completed_at, o.paid_at)
				FROM payment_orders o
				WHERE o.user_id = $1
				  AND o.order_type = 'balance'
				  AND o.status IN ('COMPLETED','PARTIALLY_REFUNDED')
				  AND COALESCE(o.completed_at, o.paid_at) >= $2
				  AND COALESCE(NULLIF(UPPER(TRIM(o.provider_snapshot->>'currency')), ''), 'CNY') = 'CNY'
				  AND NOT EXISTS (SELECT 1 FROM activity_lottery_orders lo WHERE lo.order_id = o.id)
				ORDER BY o.id ASC
			`, userID, earliest)
			if err != nil {
				return err
			}
			type candidate struct {
				id        int64
				payCents  int64
				settledAt time.Time
			}
			var candidates []candidate
			for rows.Next() {
				var c candidate
				if err := rows.Scan(&c.id, &c.payCents, &c.settledAt); err != nil {
					rows.Close()
					return err
				}
				candidates = append(candidates, c)
			}
			if err := rows.Err(); err != nil {
				rows.Close()
				return err
			}
			rows.Close()

			for _, c := range candidates {
				unitPayCents, ok := matchLotteryWindow(windows, c.settledAt)
				if !ok {
					continue
				}
				tickets := calculateLotteryOrderTickets(c.payCents, unitPayCents)
				var inserted bool
				err := tx.QueryRowContext(ctx, `
					INSERT INTO activity_lottery_orders (order_id, user_id, unit_pay_cents, granted_tickets, effective_tickets)
					VALUES ($1, $2, $3, $4, $4)
					ON CONFLICT (order_id) DO NOTHING
					RETURNING TRUE
				`, c.id, userID, unitPayCents, tickets).Scan(&inserted)
				if err != nil && !errors.Is(err, sql.ErrNoRows) {
					return err
				}
				if err == nil && inserted && tickets > 0 {
					if _, err := tx.ExecContext(ctx, `
						INSERT INTO activity_lottery_ticket_ledger (user_id, source_type, source_id, order_id, delta_tickets)
						VALUES ($1, 'order_grant', $2, $3, $4)
						ON CONFLICT (source_type, source_id) DO NOTHING
					`, userID, fmt.Sprintf("%d", c.id), c.id, tickets); err != nil {
						return err
					}
				}
			}
		}
	}

	// Reconcile finalized refunds on previously granted orders.
	refundRows, err := tx.QueryContext(ctx, `
		SELECT lo.order_id, lo.unit_pay_cents, lo.effective_tickets,
		       o.status, COALESCE(o.amount, 0)::float8, COALESCE(o.pay_amount, 0)::float8, COALESCE(o.refund_amount, 0)::float8
		FROM activity_lottery_orders lo
		JOIN payment_orders o ON o.id = lo.order_id
		WHERE lo.user_id = $1
		  AND lo.effective_tickets > 0
		  AND o.status IN ('PARTIALLY_REFUNDED', 'REFUNDED')
		FOR UPDATE OF lo
	`, userID)
	if err != nil {
		return err
	}
	type refundCandidate struct {
		orderID          int64
		unitPayCents     int
		effectiveTickets int
		status           string
		orderAmount      float64
		payAmount        float64
		refundAmount     float64
	}
	var refunds []refundCandidate
	for refundRows.Next() {
		var r refundCandidate
		if err := refundRows.Scan(&r.orderID, &r.unitPayCents, &r.effectiveTickets, &r.status, &r.orderAmount, &r.payAmount, &r.refundAmount); err != nil {
			refundRows.Close()
			return err
		}
		refunds = append(refunds, r)
	}
	if err := refundRows.Err(); err != nil {
		refundRows.Close()
		return err
	}
	refundRows.Close()

	for _, r := range refunds {
		newEffective, refundCents := calculateLotteryRefundRemainingTickets(r.status, r.orderAmount, r.payAmount, r.refundAmount, r.unitPayCents, r.effectiveTickets)
		if newEffective >= r.effectiveTickets {
			continue
		}
		delta := newEffective - r.effectiveTickets
		if _, err := tx.ExecContext(ctx, `
			UPDATE activity_lottery_orders
			SET effective_tickets = $2, last_refund_cents = $3, updated_at = NOW()
			WHERE order_id = $1
		`, r.orderID, newEffective, refundCents); err != nil {
			return err
		}
		sourceID := fmt.Sprintf("%d:%d", r.orderID, newEffective)
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO activity_lottery_ticket_ledger (user_id, source_type, source_id, order_id, delta_tickets)
			VALUES ($1, 'order_refund', $2, $3, $4)
			ON CONFLICT (source_type, source_id) DO NOTHING
		`, userID, sourceID, r.orderID, delta); err != nil {
			return err
		}
	}
	return nil
}

func (s *ActivityService) SyncLotteryOrders(ctx context.Context) error {
	var processing bool
	if err := s.db.QueryRowContext(ctx, `SELECT processing_enabled FROM activity_runtime WHERE id=1`).Scan(&processing); err != nil {
		return err
	}
	if !processing {
		return nil
	}
	pauseTx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	if err := pauseDepletedLotteryTx(ctx, pauseTx); err != nil {
		_ = pauseTx.Rollback()
		return err
	}
	if err := pauseTx.Commit(); err != nil {
		return err
	}

	var earliest sql.NullTime
	if err := s.db.QueryRowContext(ctx, `SELECT MIN(opened_at) FROM activity_lottery_windows`).Scan(&earliest); err != nil {
		return err
	}
	if !earliest.Valid {
		return nil
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT DISTINCT user_id FROM (
			SELECT o.user_id
			FROM payment_orders o
			WHERE o.order_type = 'balance'
			  AND o.status IN ('COMPLETED','PARTIALLY_REFUNDED')
			  AND COALESCE(o.completed_at, o.paid_at) >= $1
			  AND COALESCE(NULLIF(UPPER(TRIM(o.provider_snapshot->>'currency')), ''), 'CNY') = 'CNY'
			  AND activity_audience_allowed('lottery', o.user_id)
			  AND NOT EXISTS (SELECT 1 FROM activity_lottery_orders lo WHERE lo.order_id = o.id)
			  AND EXISTS (
			      SELECT 1 FROM activity_lottery_windows w
			      WHERE COALESCE(o.completed_at, o.paid_at) >= w.opened_at
			        AND (w.closed_at IS NULL OR COALESCE(o.completed_at, o.paid_at) < w.closed_at)
			  )
			LIMIT 50
		) pending_grants
		UNION
		SELECT DISTINCT lo.user_id
		FROM activity_lottery_orders lo
		JOIN payment_orders o ON o.id = lo.order_id
		WHERE lo.effective_tickets > 0
		  AND o.status IN ('PARTIALLY_REFUNDED', 'REFUNDED')
		LIMIT 50
	`, earliest.Time)
	if err != nil {
		return err
	}
	var userIDs []int64
	for rows.Next() {
		var uid int64
		if err := rows.Scan(&uid); err != nil {
			rows.Close()
			return err
		}
		userIDs = append(userIDs, uid)
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return err
	}
	rows.Close()

	for _, uid := range userIDs {
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if err := lockActivityWalletTx(ctx, tx, uid); err != nil {
			_ = tx.Rollback()
			return err
		}

		if err := syncUserLotteryTx(ctx, tx, uid); err != nil {
			_ = tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}
	}
	return nil
}

func loadLotteryConfigRow(ctx context.Context, q activityQueryer) (bool, int, int64, []LotteryTier, int, int64, float64, error) {
	var enabled bool
	var unitPayCents int
	var budgetCents int64
	var rawTiers []byte
	if err := q.QueryRowContext(ctx, `SELECT enabled, unit_pay_cents, budget_cents, tiers FROM activity_lottery_config WHERE id=1`).Scan(&enabled, &unitPayCents, &budgetCents, &rawTiers); err != nil {
		return false, 0, 0, nil, 0, 0, 0, err
	}
	var tiers []LotteryTier
	if len(rawTiers) == 0 {
		tiers = defaultLotteryTiers()
	} else if err := json.Unmarshal(rawTiers, &tiers); err != nil {
		return false, 0, 0, nil, 0, 0, 0, err
	}
	normalized, maxReward, totalWeight, expected, err := parseAndValidateLotteryTiers(tiers)
	if err != nil {
		return false, 0, 0, nil, 0, 0, 0, err
	}
	return enabled, unitPayCents, budgetCents, normalized, maxReward, totalWeight, expected, nil
}

func isUserLotteryDrawBlocked(ctx context.Context, q activityQueryer, userID int64) (bool, error) {
	var blocked bool
	err := q.QueryRowContext(ctx, `
		SELECT EXISTS(SELECT 1 FROM activity_consumption_entitlements WHERE user_id=$1 AND state='review')
		    OR EXISTS(SELECT 1 FROM activity_refund_reviews WHERE user_id=$1 AND review_status='pending')
		    OR EXISTS(SELECT 1 FROM activity_wallets WHERE user_id=$1 AND recovery_debt_cents>0)
		OR EXISTS(SELECT 1 FROM payment_orders WHERE user_id=$1 AND order_type='balance' AND status IN ('REFUND_REQUESTED','REFUNDING','REFUND_PENDING'))
	`, userID).Scan(&blocked)
	return blocked, err
}

func (s *ActivityService) LotteryStatus(ctx context.Context, userID int64) (*LotteryStatus, error) {
	var allowed bool
	if err := s.db.QueryRowContext(ctx, `SELECT mode='all' OR $1=ANY(user_ids) FROM activity_audiences WHERE kind='lottery'`, userID).Scan(&allowed); err != nil {
		return nil, err
	}
	if !allowed {
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM activity_lottery_orders WHERE user_id=$1)`, userID).Scan(&allowed); err != nil {
			return nil, err
		}
		if !allowed {
			return &LotteryStatus{Tiers: []LotteryPublicTier{}}, nil
		}
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	if err := lockActivityWalletTx(ctx, tx, userID); err != nil {
		return nil, err
	}
	if err := pauseDepletedLotteryTx(ctx, tx); err != nil {
		return nil, err
	}

	if err := syncUserLotteryTx(ctx, tx, userID); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	return s.readLotteryStatusSnapshot(ctx, userID)
}

func (s *ActivityService) readLotteryStatusSnapshot(ctx context.Context, userID int64) (*LotteryStatus, error) {
	enabled, unitPayCents, lotteryBudget, tiers, maxReward, totalWeight, _, err := loadLotteryConfigRow(ctx, s.db)
	if err != nil {
		return nil, err
	}
	var awardsEnabled bool
	if err := s.db.QueryRowContext(ctx, `SELECT awards_enabled FROM activity_runtime WHERE id=1`).Scan(&awardsEnabled); err != nil {
		return nil, err
	}

	var netTickets int
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(delta_tickets), 0)::int FROM activity_lottery_ticket_ledger WHERE user_id=$1`, userID).Scan(&netTickets); err != nil {
		return nil, err
	}
	remainingDraws := netTickets
	if remainingDraws < 0 {
		remainingDraws = 0
	}

	publicTiers := make([]LotteryPublicTier, len(tiers))
	for i, t := range tiers {
		publicTiers[i] = LotteryPublicTier{
			Position: t.Position, RewardCents: t.RewardCents, Reward: t.Reward, Probability: float64(t.Weight) / float64(totalWeight),
		}
	}

	visible := (enabled && awardsEnabled) || remainingDraws > 0 || netTickets < 0
	if !visible {
		if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM activity_lottery_draws WHERE user_id=$1)`, userID).Scan(&visible); err != nil {
			return nil, err
		}
	}
	if !visible {
		return &LotteryStatus{
			Visible:       false,
			Enabled:       enabled && awardsEnabled,
			AwardsEnabled: awardsEnabled,
			UnitPayCents:  unitPayCents,
			UnitPayYuan:   float64(unitPayCents) / 100,
			Tiers:         publicTiers,
		}, nil
	}

	day, _ := activityDay(time.Now())
	var totalWonCents, todayWonCents int64
	if err := s.db.QueryRowContext(ctx, `
		SELECT COALESCE(SUM(reward_cents), 0),
		       COALESCE(SUM(reward_cents) FILTER (WHERE (created_at AT TIME ZONE 'Asia/Shanghai')::date = $2::date), 0)
		FROM activity_lottery_draws
		WHERE user_id = $1 AND revoked_at IS NULL
	`, userID, day).Scan(&totalWonCents, &todayWonCents); err != nil {
		return nil, err
	}

	drawBlocked, err := isUserLotteryDrawBlocked(ctx, s.db, userID)
	if err != nil {
		return nil, err
	}

	var lotterySpent int64
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents), 0) FROM activity_reward_ledger WHERE kind='lottery'`).Scan(&lotterySpent); err != nil {
		return nil, err
	}
	effectiveRemaining := lotteryBudget - lotterySpent

	wallet, err := s.loadWalletSummary(ctx, userID, false)
	if err != nil {
		return nil, err
	}

	return &LotteryStatus{
		Visible:              true,
		Enabled:              enabled && awardsEnabled,
		AwardsEnabled:        awardsEnabled,
		UnitPayCents:         unitPayCents,
		UnitPayYuan:          float64(unitPayCents) / 100,
		RemainingDraws:       remainingDraws,
		TicketDebt:           max(0, -netTickets),
		TotalWonCents:        totalWonCents,
		TotalWon:             float64(totalWonCents) / 100,
		TodayWonCents:        todayWonCents,
		TodayWon:             float64(todayWonCents) / 100,
		DrawBlocked:          !awardsEnabled || drawBlocked,
		PoolPaused:           effectiveRemaining < int64(maxReward),
		TenDrawPaused:        effectiveRemaining < 10*int64(maxReward),
		WalletBalanceCents:   wallet.AvailableCents,
		WalletBalance:        wallet.Available,
		WalletTotalEarned:    wallet.TotalEarned,
		WalletTotalWithdrawn: wallet.TotalWithdrawn,
		WithdrawBlocked:      wallet.WithdrawBlocked,
		WithdrawBlockReason:  wallet.WithdrawBlockReason,
		Tiers:                publicTiers,
	}, nil
}

func (s *ActivityService) DrawLottery(ctx context.Context, userID int64, count int, idempotencyKey string) (*LotteryDrawResponse, error) {
	idempotencyKey = strings.TrimSpace(idempotencyKey)
	if (count != 1 && count != 10) || idempotencyKey == "" || len(idempotencyKey) > 64 {
		return nil, ErrLotteryInvalidDraw
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	if err := lockActivityWalletTx(ctx, tx, userID); err != nil {
		return nil, err
	}

	var active bool
	if err := tx.QueryRowContext(ctx, `SELECT status='active' FROM users WHERE id=$1 AND deleted_at IS NULL`, userID).Scan(&active); err != nil {
		return nil, err
	}
	if !active {
		return nil, ErrActivityIneligible
	}

	if err := activityAudienceAllowed(ctx, tx, "lottery", userID); err != nil {
		var earned bool
		if scanErr := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM activity_lottery_orders WHERE user_id=$1)`, userID).Scan(&earned); scanErr != nil {
			return nil, scanErr
		}
		if !earned {
			return nil, err
		}
	}

	// Idempotent replay: return existing batch if this key was already committed.
	existingRows, err := tx.QueryContext(ctx, `
		SELECT id, batch_key, draw_count, draw_index, tier_position, reward_cents, created_at, revoked_at
		FROM activity_lottery_draws
		WHERE user_id = $1 AND batch_key = $2
		ORDER BY draw_index ASC
	`, userID, idempotencyKey)
	if err != nil {
		return nil, err
	}
	var existing []LotteryDrawItem
	var existingTotal int64
	for existingRows.Next() {
		var item LotteryDrawItem
		if err := existingRows.Scan(&item.ID, &item.BatchKey, &item.DrawCount, &item.DrawIndex, &item.TierPosition, &item.RewardCents, &item.CreatedAt, &item.RevokedAt); err != nil {
			existingRows.Close()
			return nil, err
		}
		item.Reward = float64(item.RewardCents) / 100
		existingTotal += int64(item.RewardCents)
		existing = append(existing, item)
	}
	if err := existingRows.Err(); err != nil {
		existingRows.Close()
		return nil, err
	}
	existingRows.Close()

	if len(existing) > 0 {
		if existing[0].DrawCount != count {
			return nil, ErrLotteryInvalidDraw
		}
		_ = tx.Rollback()
		status, err := s.readLotteryStatusSnapshot(ctx, userID)
		if err != nil {
			return nil, err
		}
		return &LotteryDrawResponse{
			Items:            existing,
			TotalRewardCents: existingTotal,
			TotalReward:      float64(existingTotal) / 100,
			Status:           status,
		}, nil
	}

	if err := activityAwardsAllowed(ctx, tx); err != nil {
		return nil, err
	}
	if err := syncUserLotteryTx(ctx, tx, userID); err != nil {
		return nil, err
	}

	blocked, err := isUserLotteryDrawBlocked(ctx, tx, userID)
	if err != nil {
		return nil, err
	}
	if blocked {
		return nil, ErrActivityReviewBlocked
	}

	var netTickets int
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(delta_tickets), 0)::int FROM activity_lottery_ticket_ledger WHERE user_id=$1`, userID).Scan(&netTickets); err != nil {
		return nil, err
	}
	if netTickets < count {
		return nil, ErrLotteryInsufficientTickets
	}

	var lotteryBudget int64
	var rawTiers []byte
	if err := tx.QueryRowContext(ctx, `SELECT budget_cents, tiers FROM activity_lottery_config WHERE id=1 FOR UPDATE`).Scan(&lotteryBudget, &rawTiers); err != nil {
		return nil, err
	}

	var rawList []LotteryTier
	if err := json.Unmarshal(rawTiers, &rawList); err != nil {
		return nil, err
	}
	tiers, maxReward, totalWeight, _, err := parseAndValidateLotteryTiers(rawList)
	if err != nil {
		return nil, err
	}

	var lotterySpent int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents), 0) FROM activity_reward_ledger WHERE kind='lottery'`).Scan(&lotterySpent); err != nil {
		return nil, err
	}
	effectiveRemaining := lotteryBudget - lotterySpent
	if effectiveRemaining < int64(maxReward) {
		return nil, ErrLotteryBudgetInsufficient
	}
	if count == 10 && effectiveRemaining < 10*int64(maxReward) {
		return nil, ErrLotteryBatchBudgetInsufficient
	}

	sourceID := fmt.Sprintf("%d:%s", userID, idempotencyKey)
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO activity_lottery_ticket_ledger (user_id, source_type, source_id, delta_tickets)
		VALUES ($1, 'draw', $2, $3)
	`, userID, sourceID, -count); err != nil {
		return nil, err
	}

	if err := attributeLotteryDraws(ctx, tx, userID); err != nil {
		return nil, err
	}
	items := make([]LotteryDrawItem, 0, count)
	var totalRewardCents int64
	for idx := 1; idx <= count; idx++ {
		chosen, err := pickWeightedLotteryTier(tiers, totalWeight, cryptoLotteryRandInt)
		if err != nil {
			return nil, err
		}
		var item LotteryDrawItem
		item.BatchKey = idempotencyKey
		item.DrawCount = count
		item.DrawIndex = idx
		item.TierPosition = chosen.Position
		item.RewardCents = chosen.RewardCents
		item.Reward = chosen.Reward
		if err := tx.QueryRowContext(ctx, `
			INSERT INTO activity_lottery_draws (user_id, batch_key, draw_count, draw_index, tier_position, reward_cents)
			VALUES ($1, $2, $3, $4, $5, $6)
			RETURNING id, created_at
		`, userID, idempotencyKey, count, idx, chosen.Position, chosen.RewardCents).Scan(&item.ID, &item.CreatedAt); err != nil {
			return nil, err
		}
		if err := allocateLotteryDraw(ctx, tx, userID, item.ID); err != nil {
			return nil, err
		}
		totalRewardCents += int64(chosen.RewardCents)
		items = append(items, item)
	}

	if err := creditActivityWalletTx(ctx, tx, userID, "lottery", sourceID, totalRewardCents); err != nil {
		return nil, err
	}

	if err := pauseDepletedLotteryTx(ctx, tx); err != nil {
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	status, err := s.readLotteryStatusSnapshot(ctx, userID)
	if err != nil {
		return nil, err
	}
	return &LotteryDrawResponse{
		Items:            items,
		TotalRewardCents: totalRewardCents,
		TotalReward:      float64(totalRewardCents) / 100,
		Status:           status,
	}, nil
}

func (s *ActivityService) LotteryHistory(ctx context.Context, userID int64, page, pageSize int) (*LotteryHistoryResponse, error) {
	if page < 1 {
		page = 1
	}
	if pageSize < 1 || pageSize > 50 {
		pageSize = 20
	}
	offset := (page - 1) * pageSize

	var total int64
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM activity_lottery_draws WHERE user_id=$1`, userID).Scan(&total); err != nil {
		return nil, err
	}

	rows, err := s.db.QueryContext(ctx, `
		SELECT id, batch_key, draw_count, draw_index, tier_position, reward_cents, created_at, revoked_at
		FROM activity_lottery_draws
		WHERE user_id = $1
		ORDER BY created_at DESC, id DESC
		LIMIT $2 OFFSET $3
	`, userID, pageSize, offset)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	items := make([]LotteryDrawItem, 0)
	for rows.Next() {
		var item LotteryDrawItem
		if err := rows.Scan(&item.ID, &item.BatchKey, &item.DrawCount, &item.DrawIndex, &item.TierPosition, &item.RewardCents, &item.CreatedAt, &item.RevokedAt); err != nil {
			return nil, err
		}
		item.Reward = float64(item.RewardCents) / 100
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return &LotteryHistoryResponse{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}

func (s *ActivityService) AdminLotteryConfig(ctx context.Context) (*AdminLotteryConfig, error) {
	enabled, unitPayCents, budgetCents, tiers, _, _, expected, err := loadLotteryConfigRow(ctx, s.db)
	if err != nil {
		return nil, err
	}
	var spent int64
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents), 0) FROM activity_reward_ledger WHERE kind='lottery'`).Scan(&spent); err != nil {
		return nil, err
	}
	var outstanding int64
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(GREATEST(n,0)),0) FROM (SELECT SUM(delta_tickets) n FROM activity_lottery_ticket_ledger GROUP BY user_id) x`).Scan(&outstanding); err != nil {
		return nil, err
	}
	locked, err := lotteryTiersLocked(ctx, s.db)
	if err != nil {
		return nil, err
	}

	return &AdminLotteryConfig{
		Enabled:            enabled,
		UnitPayCents:       unitPayCents,
		BudgetCents:        budgetCents,
		SpentCents:         spent,
		ExpectedCents:      expected,
		OutstandingTickets: outstanding, ExpectedLiabilityCents: float64(outstanding) * expected, TiersLocked: locked,
		Tiers: tiers,
	}, nil
}

func (s *ActivityService) UpdateAdminLotteryConfig(ctx context.Context, actor int64, in AdminLotteryConfig) (*AdminLotteryConfig, error) {
	if in.UnitPayCents < 100 || in.UnitPayCents > 10_000_000 {
		return nil, fmt.Errorf("单次获赠充值门槛必须不少于 ¥1.00")
	}
	if in.BudgetCents < 0 {
		return nil, fmt.Errorf("抽奖预算不能为负数")
	}
	tiers, maxReward, _, _, err := parseAndValidateLotteryTiers(in.Tiers)
	if err != nil {
		return nil, err
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()

	var oldEnabled bool
	var oldUnitPayCents int
	var oldBudgetCents int64
	var oldTiers []byte
	if err := tx.QueryRowContext(ctx, `SELECT enabled, unit_pay_cents, budget_cents, tiers FROM activity_lottery_config WHERE id=1 FOR UPDATE`).Scan(&oldEnabled, &oldUnitPayCents, &oldBudgetCents, &oldTiers); err != nil {
		return nil, err
	}

	var lotterySpent int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents), 0) FROM activity_reward_ledger WHERE kind='lottery'`).Scan(&lotterySpent); err != nil {
		return nil, err
	}

	if in.BudgetCents < lotterySpent {
		return nil, fmt.Errorf("抽奖专用总预算不能低于已发放金额")
	}
	if in.Enabled {
		if in.BudgetCents-lotterySpent < int64(maxReward) {
			return nil, fmt.Errorf("启用抽奖时，抽奖剩余预算必须至少能覆盖一次最高档奖金（$%.2f）", float64(maxReward)/100)
		}
	}

	rawTiers, err := json.Marshal(tiers)
	if err != nil {
		return nil, err
	}
	var previousTiers []LotteryTier
	if err := json.Unmarshal(oldTiers, &previousTiers); err != nil {
		return nil, err
	}
	changed := len(previousTiers) != len(tiers)
	for i := range tiers {
		if i >= len(previousTiers) || previousTiers[i].RewardCents != tiers[i].RewardCents || previousTiers[i].Weight != tiers[i].Weight {
			changed = true
			break
		}
	}
	if changed {
		locked, err := lotteryTiersLocked(ctx, tx)
		if err != nil {
			return nil, err
		}
		if locked {
			return nil, fmt.Errorf("请先停止新增抽奖资格，并兑付已有及待同步次数后再修改奖品或概率；预算仍可补充")
		}
		for _, tier := range tiers {
			if tier.Weight == 0 {
				return nil, fmt.Errorf("八档奖项必须全部配置正整数权重")
			}
		}
	}

	if _, err := tx.ExecContext(ctx, `
		UPDATE activity_lottery_config
		SET enabled = $1, unit_pay_cents = $2, budget_cents = $3, tiers = $4::jsonb, updated_at = NOW()
		WHERE id = 1
	`, in.Enabled, in.UnitPayCents, in.BudgetCents, string(rawTiers)); err != nil {
		return nil, err
	}

	if oldEnabled && (!in.Enabled || in.UnitPayCents != oldUnitPayCents) {
		if _, err := tx.ExecContext(ctx, `UPDATE activity_lottery_windows SET closed_at = NOW() WHERE closed_at IS NULL`); err != nil {
			return nil, err
		}
	}
	if in.Enabled && (!oldEnabled || in.UnitPayCents != oldUnitPayCents) {
		if _, err := tx.ExecContext(ctx, `INSERT INTO activity_lottery_windows (unit_pay_cents, opened_at) VALUES ($1, NOW())`, in.UnitPayCents); err != nil {
			return nil, err
		}
	}

	if err := consumptionAudit(ctx, tx, actor, "lottery_config", 1, map[string]any{
		"before_enabled":        oldEnabled,
		"before_unit_pay_cents": oldUnitPayCents,
		"before_budget_cents":   oldBudgetCents,
		"after_enabled":         in.Enabled,
		"after_unit_pay_cents":  in.UnitPayCents,
		"after_budget_cents":    in.BudgetCents,
		"tiers":                 tiers,
	}); err != nil {
		return nil, err
	}

	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return s.AdminLotteryConfig(ctx)
}

func (s *ActivityService) AdminLotteryUser(ctx context.Context, userID int64) (*AdminLotteryUserSummary, error) {
	var exists bool
	if err := s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users WHERE id=$1)`, userID).Scan(&exists); err != nil {
		return nil, err
	}
	if !exists {
		return nil, fmt.Errorf("user not found")
	}

	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	if err := lockActivityWalletTx(ctx, tx, userID); err != nil {
		_ = tx.Rollback()
		return nil, err
	}

	if err := syncUserLotteryTx(ctx, tx, userID); err != nil {
		_ = tx.Rollback()
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}

	wallet, err := s.loadWalletSummary(ctx, userID, false)
	if err != nil {
		return nil, err
	}
	blocked, err := isUserLotteryDrawBlocked(ctx, s.db, userID)
	if err != nil {
		return nil, err
	}

	var netTickets int
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(delta_tickets), 0)::int FROM activity_lottery_ticket_ledger WHERE user_id=$1`, userID).Scan(&netTickets); err != nil {
		return nil, err
	}
	remaining := netTickets
	if remaining < 0 {
		remaining = 0
	}

	var totalWonCents int64
	if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(reward_cents), 0) FROM activity_lottery_draws WHERE user_id=$1 AND revoked_at IS NULL`, userID).Scan(&totalWonCents); err != nil {
		return nil, err
	}

	tRows, err := s.db.QueryContext(ctx, `
		SELECT id, source_type, source_id, order_id, delta_tickets, created_at
		FROM activity_lottery_ticket_ledger
		WHERE user_id = $1
		ORDER BY id DESC
		LIMIT 50
	`, userID)
	if err != nil {
		return nil, err
	}
	defer tRows.Close()
	tickets := make([]LotteryTicketLedgerItem, 0)
	for tRows.Next() {
		var item LotteryTicketLedgerItem
		var oid sql.NullInt64
		if err := tRows.Scan(&item.ID, &item.SourceType, &item.SourceID, &oid, &item.DeltaTickets, &item.CreatedAt); err != nil {
			return nil, err
		}
		if oid.Valid {
			v := oid.Int64
			item.OrderID = &v
		}
		tickets = append(tickets, item)
	}
	if err := tRows.Err(); err != nil {
		return nil, err
	}

	history, err := s.LotteryHistory(ctx, userID, 1, 50)
	if err != nil {
		return nil, err
	}

	return &AdminLotteryUserSummary{
		UserID:         userID,
		NetTickets:     netTickets,
		RemainingDraws: remaining,
		TotalWonCents:  totalWonCents,
		TotalWon:       float64(totalWonCents) / 100,
		DrawBlocked:    blocked,
		Wallet:         wallet,
		Tickets:        tickets,
		Draws:          history.Items,
	}, nil
}
