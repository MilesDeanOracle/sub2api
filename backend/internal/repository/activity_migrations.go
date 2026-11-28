package repository

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"os"
)

// prepareActivityMigration owns activity-specific migration compatibility.
// Keeping these checks here leaves the generic runner independent of the
// activity schema and its historical migration lineage.
func prepareActivityMigration(ctx context.Context, conn *sql.Conn, name string) (skip bool, err error) {
	// Retired files remain embedded for checksum validation, not execution.
	// Missing historical steps are superseded by the forward schema/data bridge.
	if retiredActivityMigration(name) {
		return true, nil
	}
	if name == "268_activity_atomic_billing.sql" {
		if err := requireActivityMaintenance(ctx, conn); err != nil {
			return false, fmt.Errorf("migration %s: %w", name, err)
		}
	}
	if name == "257zz_activity_legacy_bridge.sql" {
		var legacy bool
		if err := conn.QueryRowContext(ctx, `SELECT to_regclass('activity_config') IS NOT NULL AND NOT EXISTS(SELECT 1 FROM information_schema.columns WHERE table_schema=current_schema() AND table_name='activity_config' AND column_name='checkin_require_recharge')`).Scan(&legacy); err != nil {
			return false, err
		}
		if legacy {
			if err := requireActivityMaintenance(ctx, conn); err != nil {
				return false, fmt.Errorf("migration %s: %w", name, err)
			}
		}
	}
	return false, nil
}

func configureActivityMigrationTx(ctx context.Context, tx *sql.Tx, name string) error {
	if name != "257zz_activity_legacy_bridge.sql" && name != "268_activity_atomic_billing.sql" {
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

// Billing cutovers must not overlap old binaries that cannot write atomic
// activity events. This restores the existing maintenance acknowledgement used
// by the retired activity lineage, without weakening checksum validation.
func requireActivityMaintenance(ctx context.Context, conn *sql.Conn) error {
	if os.Getenv("ACTIVITY_MAINTENANCE_CONFIRMED") == "1" {
		return nil
	}
	var hasUsers bool
	if err := conn.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM users LIMIT 1)`).Scan(&hasUsers); err != nil {
		return err
	}
	if hasUsers {
		return errors.New("activity accounting upgrade requires a drained maintenance window; rehearse backup and reconciliation, stop all balance writers and flush pending usage logs, then set ACTIVITY_MAINTENANCE_CONFIRMED=1 for this migration run")
	}
	return nil
}
