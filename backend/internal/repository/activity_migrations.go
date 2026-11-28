package repository

import (
	"context"
	"database/sql"
)

// prepareActivityMigration owns activity-specific migration compatibility.
// This fork only supports fresh installations: the retired trigger-based
// lineage is never executed, and the legacy bridge/conversion migrations have
// been removed. Keeping the retired-file guard here leaves the generic runner
// independent of the activity schema.
func prepareActivityMigration(_ context.Context, _ *sql.Conn, name string) (skip bool, err error) {
	// Retired files remain embedded for checksum validation, not execution.
	if retiredActivityMigration(name) {
		return true, nil
	}
	return false, nil
}

func configureActivityMigrationTx(ctx context.Context, tx *sql.Tx, name string) error {
	if name != "268_activity_atomic_billing.sql" {
		return nil
	}
	_, err := tx.ExecContext(ctx, `SET LOCAL lock_timeout='5s'`)
	return err
}

func retiredActivityMigration(name string) bool {
	switch name {
	case "257_activity_rewards.sql",
		"258_activity_checkin_min_spend.sql",
		"259_consumption_rewards.sql",
		"260_consumption_refund_lock_order.sql",
		"261_consumption_batch_rounding.sql",
		"262_activity_production_hardening.sql",
		"263_activity_audiences.sql":
		return true
	default:
		return false
	}
}
