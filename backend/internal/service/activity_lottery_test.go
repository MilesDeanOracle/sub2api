package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
)

func TestLotteryOrderTicketsSingleOrderFloor(t *testing.T) {
	// Why: Each balance order must independently floor(pay_cents / unit_pay_cents)
	// so ¥9 gives 0, ¥25 gives 2, and remainders never carry across orders.
	cases := []struct {
		payCents int64
		unit     int
		want     int
	}{
		{0, 1000, 0},
		{999, 1000, 0},
		{1000, 1000, 1},
		{2599, 1000, 2},
		{5000, 1000, 5},
		{50000, 1000, 50},
		{3000, 2000, 1},
	}
	for _, tc := range cases {
		require.Equal(t, tc.want, calculateLotteryOrderTickets(tc.payCents, tc.unit))
	}
}

func TestLotteryWindowScopingIgnoresHistoricalAndClosedPeriods(t *testing.T) {
	// Why: Enabling lottery must never retroactively grant draws to pre-launch
	// orders or orders settled while lottery was temporarily disabled.
	t0 := time.Date(2026, 11, 25, 10, 0, 0, 0, time.UTC)
	windows := []lotteryWindow{
		{id: 1, unitPayCents: 1000, openedAt: t0, closedAt: sql.NullTime{Time: t0.Add(2 * time.Hour), Valid: true}},
		{id: 2, unitPayCents: 2000, openedAt: t0.Add(4 * time.Hour)},
	}

	_, ok := matchLotteryWindow(windows, t0.Add(-time.Second))
	require.False(t, ok, "historical order before first open must not match")

	unit, ok := matchLotteryWindow(windows, t0.Add(time.Hour))
	require.True(t, ok)
	require.Equal(t, 1000, unit)

	_, ok = matchLotteryWindow(windows, t0.Add(2*time.Hour))
	require.False(t, ok, "order at or after closed_at must not match")

	_, ok = matchLotteryWindow(windows, t0.Add(3*time.Hour))
	require.False(t, ok, "order during disabled gap must not match")

	unit, ok = matchLotteryWindow(windows, t0.Add(5*time.Hour))
	require.True(t, ok)
	require.Equal(t, 2000, unit, "reopened window uses its own unit_pay_cents")
}

func TestLotteryTierValidationAndExpectedValue(t *testing.T) {
	tiers := defaultLotteryTiers()
	normalized, maxReward, totalWeight, expectedCents, err := parseAndValidateLotteryTiers(tiers)
	require.NoError(t, err)
	require.Len(t, normalized, 8)
	require.Equal(t, 5000, maxReward)
	require.Equal(t, int64(10000), totalWeight)
	// All eight prizes participate while the expected reward stays 62.9 cents.
	for _, tier := range normalized {
		require.Positive(t, tier.Weight)
	}
	require.InDelta(t, 62.9, expectedCents, 1e-6)

	badCount := tiers[:5]
	_, _, _, _, err = parseAndValidateLotteryTiers(badCount)
	require.Error(t, err)

	badWeight := append([]LotteryTier(nil), tiers...)
	badWeight[3].Weight = -1
	_, _, _, _, err = parseAndValidateLotteryTiers(badWeight)
	require.Error(t, err)

	badReward := append([]LotteryTier(nil), tiers...)
	badReward[0].RewardCents = 0
	_, _, _, _, err = parseAndValidateLotteryTiers(badReward)
	require.Error(t, err)
}

func legacySixLotteryTiers() []LotteryTier {
	return []LotteryTier{
		{Position: 1, RewardCents: 50, Weight: 8800},
		{Position: 2, RewardCents: 100, Weight: 950},
		{Position: 3, RewardCents: 300, Weight: 200},
		{Position: 4, RewardCents: 500, Weight: 40},
		{Position: 5, RewardCents: 1000, Weight: 9},
		{Position: 6, RewardCents: 5000, Weight: 1},
	}
}

func TestLotteryLegacyInactivePrizes(t *testing.T) {
	tiers := append(legacySixLotteryTiers(), LotteryTier{Position: 7, RewardCents: 200}, LotteryTier{Position: 8, RewardCents: 10_000_000})
	_, maxReward, total, ev, err := parseAndValidateLotteryTiers(tiers)
	require.NoError(t, err)
	require.Equal(t, 5000, maxReward, "inactive legacy prizes must not pause a funded pool")
	require.InDelta(t, 62.9, ev, 1e-6)
	for r := int64(0); r < total; r++ {
		chosen, err := pickWeightedLotteryTier(tiers, total, func(int64) (int64, error) { return r, nil })
		require.NoError(t, err)
		require.LessOrEqual(t, chosen.Position, 6, "zero-weight legacy prizes must never be selected")
	}
	raw, err := json.Marshal(tiers)
	require.NoError(t, err)
	require.Contains(t, string(raw), `"weight":0`, "admin API must explicitly return zero, not omit it")
	for i := range tiers {
		tiers[i].Weight = 0
	}
	_, _, _, _, err = parseAndValidateLotteryTiers(tiers)
	require.Error(t, err)
}

func TestLotteryWeightedSelectionBoundaries(t *testing.T) {
	tiers, _, totalWeight, _, err := parseAndValidateLotteryTiers(defaultLotteryTiers())
	require.NoError(t, err)

	check := func(r int64, wantPos, wantCents int) {
		t.Helper()
		got, err := pickWeightedLotteryTier(tiers, totalWeight, func(max int64) (int64, error) {
			require.Equal(t, int64(10000), max)
			return r, nil
		})
		require.NoError(t, err)
		require.Equal(t, wantPos, got.Position)
		require.Equal(t, wantCents, got.RewardCents)
	}

	check(0, 1, 50)
	check(8737, 1, 50)
	check(8738, 2, 100)
	check(9698, 2, 100)
	check(9699, 3, 200)
	check(9848, 3, 200)
	check(9849, 4, 300)
	check(9948, 4, 300)
	check(9949, 5, 500)
	check(9988, 5, 500)
	check(9989, 6, 1000)
	check(9997, 6, 1000)
	check(9998, 7, 2000)
	check(9999, 8, 5000)
}

func TestLotteryRefundRemainingTickets(t *testing.T) {
	// Full refund revokes all granted tickets.
	rem, refundCents := calculateLotteryRefundRemainingTickets("REFUNDED", 100, 100, 100, 1000, 10)
	require.Equal(t, 0, rem)
	require.Equal(t, int64(10000), refundCents)

	// Partial refund on order with bonus ($130 credited for ¥100 paid; $65 refunded -> ¥50 refunded -> ¥50 net -> 5 tickets).
	rem, refundCents = calculateLotteryRefundRemainingTickets("PARTIALLY_REFUNDED", 130, 100, 65, 1000, 10)
	require.Equal(t, 5, rem)
	require.Equal(t, int64(5000), refundCents)

	// Partial refund leaving ¥29 net -> floor(2900 / 1000) = 2 tickets.
	rem, refundCents = calculateLotteryRefundRemainingTickets("PARTIALLY_REFUNDED", 100, 100, 71, 1000, 10)
	require.Equal(t, 2, rem)
	require.Equal(t, int64(7100), refundCents)
}

func TestDrawLotteryBudgetPauseNeverSilentlyAltersOdds(t *testing.T) {
	// Why: When remaining budget ($30) < max prize ($50), drawing must pause and keep
	// the user's tickets intact rather than silently dropping the $50 prize tier.
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	tiersJSON, err := json.Marshal(defaultLotteryTiers())
	require.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM users WHERE id=\$1 FOR UPDATE`).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(7)))
	mock.ExpectExec(`INSERT INTO activity_wallets`).WithArgs(int64(7)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT user_id FROM activity_wallets WHERE user_id=\$1 FOR UPDATE`).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(int64(7)))
	mock.ExpectQuery(`SELECT status='active' FROM users`).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"active"}).AddRow(true))
	mock.ExpectQuery(`SELECT mode,`).WithArgs("lottery", int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"mode", "allowed", "admin"}).AddRow("all", true, false))
	mock.ExpectQuery(`SELECT id, batch_key, draw_count`).WithArgs(int64(7), "batch-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "batch_key", "draw_count", "draw_index", "tier_position", "reward_cents", "created_at", "revoked_at"}))
	mock.ExpectQuery(`SELECT awards_enabled FROM activity_runtime`).
		WillReturnRows(sqlmock.NewRows([]string{"awards_enabled"}).AddRow(true))
	// syncUserLotteryTx
	mock.ExpectQuery(`SELECT id, status, COALESCE\(refund_amount,0\)::text FROM payment_orders`).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"id", "status"}))
	mock.ExpectQuery(`SELECT activity_audience_allowed\('lottery',\$1\) FROM activity_audiences WHERE kind='lottery'`).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"allowed"}).AddRow(true))
	mock.ExpectQuery(`SELECT id, unit_pay_cents, opened_at, closed_at FROM activity_lottery_windows`).
		WillReturnRows(sqlmock.NewRows([]string{"id", "unit_pay_cents", "opened_at", "closed_at"}))
	mock.ExpectQuery(`SELECT lo\.order_id`).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"order_id", "unit_pay_cents", "effective_tickets", "status", "amount", "pay_amount", "refund_amount"}))
	// draw blocked check
	mock.ExpectQuery(`SELECT EXISTS`).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"blocked"}).AddRow(false))
	// net tickets = 5
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(delta_tickets\), 0\)::int FROM activity_lottery_ticket_ledger`).WithArgs(int64(7)).
		WillReturnRows(sqlmock.NewRows([]string{"net"}).AddRow(5))
	// Lottery budget = $50, already spent = $20 -> remaining = $30 < $50 max tier
	mock.ExpectQuery(`SELECT budget_cents, tiers FROM activity_lottery_config`).
		WillReturnRows(sqlmock.NewRows([]string{"budget_cents", "tiers"}).AddRow(int64(5000), tiersJSON))
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(amount_cents\), 0\) FROM activity_reward_ledger WHERE kind='lottery'`).
		WillReturnRows(sqlmock.NewRows([]string{"spent"}).AddRow(int64(2000)))
	mock.ExpectRollback()

	svc := NewActivityService(db, nil, nil)
	_, err = svc.DrawLottery(context.Background(), 7, 1, "batch-1")
	require.ErrorIs(t, err, ErrLotteryBudgetInsufficient)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestDrawLotteryIdempotentReplayReturnsSavedBatch(t *testing.T) {
	// Why: Network retries with the same idempotency_key must replay the committed
	// draw result without charging extra tickets or crediting wallet twice.
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()

	now := time.Now()
	tiersJSON, err := json.Marshal(defaultLotteryTiers())
	require.NoError(t, err)

	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id FROM users WHERE id=\$1 FOR UPDATE`).WithArgs(int64(9)).WillReturnRows(sqlmock.NewRows([]string{"id"}).AddRow(int64(9)))
	mock.ExpectExec(`INSERT INTO activity_wallets`).WithArgs(int64(9)).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectQuery(`SELECT user_id FROM activity_wallets WHERE user_id=\$1 FOR UPDATE`).WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"user_id"}).AddRow(int64(9)))
	mock.ExpectQuery(`SELECT status='active' FROM users`).WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"active"}).AddRow(true))
	mock.ExpectQuery(`SELECT mode,`).WithArgs("lottery", int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"mode", "allowed", "admin"}).AddRow("all", true, false))
	mock.ExpectQuery(`SELECT id, batch_key, draw_count`).WithArgs(int64(9), "idem-key-1").
		WillReturnRows(sqlmock.NewRows([]string{"id", "batch_key", "draw_count", "draw_index", "tier_position", "reward_cents", "created_at", "revoked_at"}).
			AddRow(int64(101), "idem-key-1", 1, 1, 3, 300, now, nil))
	mock.ExpectRollback()

	// readLotteryStatusSnapshot queries
	mock.ExpectQuery(`SELECT enabled, unit_pay_cents, budget_cents, tiers FROM activity_lottery_config`).
		WillReturnRows(sqlmock.NewRows([]string{"enabled", "unit_pay_cents", "budget_cents", "tiers"}).AddRow(true, 1000, int64(50000), tiersJSON))
	mock.ExpectQuery(`SELECT awards_enabled FROM activity_runtime`).
		WillReturnRows(sqlmock.NewRows([]string{"awards_enabled"}).AddRow(true))
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(delta_tickets\), 0\)::int FROM activity_lottery_ticket_ledger`).WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"net"}).AddRow(4))
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(reward_cents\), 0\)`).WithArgs(int64(9), sqlmock.AnyArg()).
		WillReturnRows(sqlmock.NewRows([]string{"total", "today"}).AddRow(int64(300), int64(300)))
	mock.ExpectQuery(`SELECT EXISTS`).WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"blocked"}).AddRow(false))
	mock.ExpectQuery(`SELECT COALESCE\(SUM\(amount_cents\), 0\) FROM activity_reward_ledger WHERE kind='lottery'`).
		WillReturnRows(sqlmock.NewRows([]string{"spent"}).AddRow(int64(300)))
	mock.ExpectQuery(`SELECT available_cents,total_earned_cents,total_withdrawn_cents,unabsorbed_gift::text,recovery_debt_cents FROM activity_wallets`).WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"available", "earned", "withdrawn", "gift", "debt"}).AddRow(int64(300), int64(300), int64(0), "0", int64(0)))
	mock.ExpectQuery(`SELECT activity_reserved_cents`).WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"reserved", "visible"}).AddRow(int64(0), true))
	mock.ExpectQuery(`SELECT awards_enabled FROM activity_runtime`).
		WillReturnRows(sqlmock.NewRows([]string{"awards_enabled"}).AddRow(true))
	mock.ExpectQuery(`SELECT EXISTS`).WithArgs(int64(9)).
		WillReturnRows(sqlmock.NewRows([]string{"blocked"}).AddRow(false))

	svc := NewActivityService(db, nil, nil)
	res, err := svc.DrawLottery(context.Background(), 9, 1, "idem-key-1")
	require.NoError(t, err)
	require.Len(t, res.Items, 1)
	require.Equal(t, int64(300), res.TotalRewardCents)
	require.True(t, math.Abs(res.TotalReward-3.0) < 1e-9)
	require.Equal(t, 4, res.Status.RemainingDraws)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestMigration259ZeroCoreInvasion(t *testing.T) {
	// Why: Hard zero-invasion invariant — lottery migration must never alter, add
	// triggers to, or add foreign keys referencing core sub2api tables.
	raw, err := os.ReadFile(filepath.Join("..", "..", "migrations", "259_activity_lottery.sql"))
	require.NoError(t, err)
	upper := strings.ToUpper(string(raw))
	require.NotContains(t, upper, "CREATE TRIGGER")
	for _, core := range []string{"USERS", "PAYMENT_ORDERS", "USAGE_LOGS", "REDEEM_CODES", "SETTINGS"} {
		require.NotContains(t, upper, "ALTER TABLE "+core)
		require.NotContains(t, upper, "REFERENCES "+core)
	}
}
