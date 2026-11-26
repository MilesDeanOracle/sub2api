package activitystore

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type Funds struct {
	Wallet, Debt          int64
	Balance, Frozen, Gift string
}

func LoadFunds(ctx context.Context, tx Tx, uid int64) (Funds, error) {
	var f Funds
	err := Scan(ctx, tx, `SELECT w.available_cents,w.recovery_debt_cents,u.balance::text,COALESCE(u.frozen_balance,0)::text,w.unabsorbed_gift::text FROM activity_wallets w JOIN users u ON u.id=w.user_id WHERE w.user_id=$1`, []any{uid}, &f.Wallet, &f.Debt, &f.Balance, &f.Frozen, &f.Gift)
	return f, err
}
func RecoveryState(ctx context.Context, tx Tx, kind, source string) (string, error) {
	var state string
	err := Scan(ctx, tx, `SELECT state FROM activity_reward_recoveries WHERE kind=$1 AND source_id=$2`, []any{kind, source}, &state)
	return state, err
}
func AdjustBalance(ctx context.Context, tx Tx, uid int64, delta string) error {
	_, err := tx.ExecContext(ctx, `UPDATE users SET balance=balance+$2::numeric,updated_at=NOW() WHERE id=$1`, uid, delta)
	return err
}

type Recovery struct {
	Kind, Source, State                   string
	UserID, Amount, Wallet, Balance, Debt int64
	Gift                                  string
}

// ApplyRecovery persists a service-calculated split in the caller's transaction.
// Gross earning and budget ledgers are deliberately retained as immutable history.
func ApplyRecovery(ctx context.Context, tx Tx, r Recovery) error {
	var after int64
	if err := Scan(ctx, tx, `UPDATE activity_wallets SET available_cents=available_cents-$2,recovery_debt_cents=recovery_debt_cents+$3,unabsorbed_gift=unabsorbed_gift-$4::numeric,updated_at=NOW() WHERE user_id=$1 RETURNING available_cents`, []any{r.UserID, r.Wallet, r.Debt, r.Gift}, &after); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO activity_reward_recoveries(kind,source_id,user_id,amount_cents,wallet_cents,balance_cents,debt_cents,absorbed_gift,state) VALUES($1,$2,$3,$4,$5,$6,$7,$8::numeric,$9) ON CONFLICT(kind,source_id) DO UPDATE SET amount_cents=EXCLUDED.amount_cents,wallet_cents=EXCLUDED.wallet_cents,balance_cents=EXCLUDED.balance_cents,debt_cents=EXCLUDED.debt_cents,absorbed_gift=EXCLUDED.absorbed_gift,state=EXCLUDED.state,updated_at=NOW()`, r.Kind, r.Source, r.UserID, r.Amount, r.Wallet, r.Balance, r.Debt, r.Gift, r.State); err != nil {
		return err
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO activity_wallet_ledger(user_id,direction,kind,source_id,amount_cents,wallet_after_cents) VALUES($1,'revoke',$2,$3,$4,$5)`, r.UserID, r.Kind, fmt.Sprintf("%s:%d", r.Source, time.Now().UnixNano()), r.Amount, after)
	return err
}

func ReleaseRecovery(ctx context.Context, tx Tx, uid int64, source string) error {
	var r Recovery
	r.UserID = uid
	err := Scan(ctx, tx, `UPDATE activity_reward_recoveries SET state='released',updated_at=NOW() WHERE kind='lottery_refund' AND source_id=$1 AND state='held' RETURNING amount_cents,wallet_cents,balance_cents,debt_cents,absorbed_gift::text`, []any{source}, &r.Amount, &r.Wallet, &r.Balance, &r.Debt, &r.Gift)
	if err == sql.ErrNoRows {
		return nil
	}
	if err != nil {
		return err
	}
	if r.Balance > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET balance=balance+($2::numeric/100),updated_at=NOW() WHERE id=$1`, uid, r.Balance); err != nil {
			return err
		}
	}
	var after int64
	if err := Scan(ctx, tx, `UPDATE activity_wallets SET available_cents=available_cents+$2,recovery_debt_cents=recovery_debt_cents-$3,unabsorbed_gift=unabsorbed_gift+$4::numeric,updated_at=NOW() WHERE user_id=$1 RETURNING available_cents`, []any{uid, r.Wallet, r.Debt, r.Gift}, &after); err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO activity_wallet_ledger(user_id,direction,kind,source_id,amount_cents,wallet_after_cents) VALUES($1,'credit','refund_restore',$2,$3,$4)`, uid, fmt.Sprintf("%s:%d", source, time.Now().UnixNano()), r.Amount, after)
	return err
}

func RepayDebt(ctx context.Context, tx Tx, uid, wallet, balance, debt int64) error {
	var after int64
	if err := Scan(ctx, tx, `UPDATE activity_wallets SET available_cents=available_cents-$2,recovery_debt_cents=0,unabsorbed_gift=GREATEST(0,unabsorbed_gift-$3::numeric/100),updated_at=NOW() WHERE user_id=$1 RETURNING available_cents`, []any{uid, wallet, balance}, &after); err != nil {
		return err
	}
	if balance > 0 {
		if _, err := tx.ExecContext(ctx, `UPDATE users SET balance=balance-$2::numeric/100,updated_at=NOW() WHERE id=$1`, uid, balance); err != nil {
			return err
		}
	}
	_, err := tx.ExecContext(ctx, `INSERT INTO activity_wallet_ledger(user_id,direction,kind,source_id,amount_cents,wallet_after_cents) VALUES($1,'revoke','debt_repayment',$2,$3,$4)`, uid, fmt.Sprintf("%d:%d", uid, time.Now().UnixNano()), debt, after)
	return err
}

// TransferWallet writes the debit, site-balance credit and immutable transfer
// ledger together. Eligibility/freeze checks remain the caller's responsibility.
func TransferWallet(ctx context.Context, tx Tx, uid, amount int64) (string, int64, error) {
	var balance string
	err := Scan(ctx, tx, `UPDATE users SET balance=balance+($1::numeric/100),updated_at=NOW() WHERE id=$2 AND status='active' AND deleted_at IS NULL RETURNING balance::text`, []any{amount, uid}, &balance)
	if err != nil {
		return "", 0, err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE activity_wallets SET available_cents=available_cents-$2,total_withdrawn_cents=total_withdrawn_cents+$2,unabsorbed_gift=unabsorbed_gift+($2::numeric/100),updated_at=NOW() WHERE user_id=$1`, uid, amount); err != nil {
		return "", 0, err
	}
	var ledger int64
	err = Scan(ctx, tx, `INSERT INTO activity_wallet_ledger(user_id,direction,kind,source_id,amount_cents,wallet_after_cents,user_balance_after,created_at) VALUES($1,'withdraw','manual_withdraw',$2,$3,0,$4::numeric,clock_timestamp()) RETURNING id`, []any{uid, fmt.Sprintf("withdraw:%d:%d", uid, time.Now().UnixNano()), amount, balance}, &ledger)
	return balance, ledger, err
}
