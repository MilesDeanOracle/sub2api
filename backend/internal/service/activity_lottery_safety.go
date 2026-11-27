package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"github.com/Wei-Shaw/sub2api/internal/repository/activitystore"
)

// Close the issuance interval at the moment a payout exhausts usable budget.
// Historical orders retain the rules of their original settlement interval.
func pauseDepletedLotteryTx(ctx context.Context, tx *sql.Tx) error {
	var enabled bool
	var budget int64
	var raw []byte
	if err := tx.QueryRowContext(ctx, `SELECT enabled,budget_cents,tiers FROM activity_lottery_config WHERE id=1 FOR UPDATE`).Scan(&enabled, &budget, &raw); err != nil {
		return err
	}
	if !enabled {
		return nil
	}
	var tiers []LotteryTier
	if err := json.Unmarshal(raw, &tiers); err != nil {
		return err
	}
	_, maxReward, _, _, err := parseAndValidateLotteryTiers(tiers)
	if err != nil {
		return err
	}
	var spent int64
	if err := tx.QueryRowContext(ctx, `SELECT COALESCE(SUM(amount_cents),0) FROM activity_reward_ledger WHERE kind='lottery'`).Scan(&spent); err != nil {
		return err
	}
	if budget-spent >= int64(maxReward) {
		return nil
	}
	if _, err := tx.ExecContext(ctx, `UPDATE activity_lottery_windows SET closed_at=clock_timestamp() WHERE closed_at IS NULL`); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `UPDATE activity_lottery_config SET enabled=false,updated_at=NOW() WHERE id=1`)
	return err
}

func lotteryTiersLocked(ctx context.Context, db activityAccountingDB) (bool, error) {
	return activitystore.LotteryTiersLocked(ctx, db)
}
