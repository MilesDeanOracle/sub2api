package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"time"

	"github.com/shopspring/decimal"
)

type ConsumptionTier struct {
	Position       int    `json:"position"`
	ThresholdCents int64  `json:"threshold_cents"`
	RewardCents    int64  `json:"reward_cents"`
	State          string `json:"state,omitempty"`
}
type ConsumptionCampaign struct {
	ID             int64             `json:"id"`
	Name           string            `json:"name"`
	StartsAt       time.Time         `json:"starts_at"`
	EndsAt         time.Time         `json:"ends_at"`
	ClaimUntil     time.Time         `json:"claim_until"`
	RolloutPercent int               `json:"rollout_percent"`
	BudgetCents    int64             `json:"budget_cents"`
	State          string            `json:"state"`
	PublishedAt    *time.Time        `json:"published_at"`
	Tiers          []ConsumptionTier `json:"tiers"`
	ReservedCents  int64             `json:"reserved_cents"`
	ClaimedCents   int64             `json:"claimed_cents"`
	Participants   int64             `json:"participants"`
	QualifiedUsers int64             `json:"qualified_users"`
}
type ConsumptionSummary struct {
	AwardsEnabled     bool                 `json:"awards_enabled"`
	ProcessingPending bool                 `json:"processing_pending"`
	Campaign          *ConsumptionCampaign `json:"campaign"`
	Spend             string               `json:"spend"`
	ClaimableCents    int64                `json:"claimable_cents"`
	Blocked           bool                 `json:"blocked"`
	BudgetBlocked     bool                 `json:"budget_blocked"`
	Funds             ConsumptionFunds     `json:"funds"`
	History           []ConsumptionHistory `json:"history"`
}
type ConsumptionFunds struct {
	Principal  string `json:"principal"`
	Gift       string `json:"gift"`
	Unverified string `json:"unverified"`
}
type ConsumptionHistory struct {
	Campaign    string    `json:"campaign"`
	Position    int       `json:"position"`
	RewardCents int64     `json:"reward_cents"`
	State       string    `json:"state"`
	CreatedAt   time.Time `json:"created_at"`
}

func consumptionTiers(ctx context.Context, db interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}, id int64) ([]ConsumptionTier, error) {
	rows, err := db.QueryContext(ctx, `SELECT position,threshold_cents,reward_cents FROM activity_consumption_tiers WHERE campaign_id=$1 ORDER BY position`, id)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	tiers := []ConsumptionTier{}
	for rows.Next() {
		var t ConsumptionTier
		if err := rows.Scan(&t.Position, &t.ThresholdCents, &t.RewardCents); err != nil {
			return nil, err
		}
		tiers = append(tiers, t)
	}
	return tiers, rows.Err()
}

const consumptionColumns = `id,name,starts_at,ends_at,claim_until,rollout_percent,budget_cents,state,published_at`

func scanCampaign(row interface{ Scan(...any) error }) (*ConsumptionCampaign, error) {
	c := &ConsumptionCampaign{}
	err := row.Scan(&c.ID, &c.Name, &c.StartsAt, &c.EndsAt, &c.ClaimUntil, &c.RolloutPercent, &c.BudgetCents, &c.State, &c.PublishedAt)
	return c, err
}
func (s *ActivityService) ConsumptionCampaigns(ctx context.Context) ([]ConsumptionCampaign, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT `+consumptionColumns+` FROM activity_consumption_campaigns ORDER BY id DESC LIMIT 50`)
	if err != nil {
		return nil, err
	}
	campaigns := []ConsumptionCampaign{}
	for rows.Next() {
		c, err := scanCampaign(rows)
		if err != nil {
			rows.Close()
			return nil, err
		}
		campaigns = append(campaigns, *c)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	for i := range campaigns {
		c := &campaigns[i]
		c.Tiers, err = consumptionTiers(ctx, s.db, c.ID)
		if err != nil {
			return nil, err
		}
		err = s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(reward_cents) FILTER(WHERE state IN ('reserved','review') AND claimed_at IS NULL AND (state='review' OR $2>NOW())),0),COALESCE(SUM(reward_cents) FILTER(WHERE claimed_at IS NOT NULL),0),COUNT(DISTINCT user_id) FROM activity_consumption_entitlements WHERE campaign_id=$1`, c.ID, c.ClaimUntil).Scan(&c.ReservedCents, &c.ClaimedCents, &c.QualifiedUsers)
		if err != nil {
			return nil, err
		}
		err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM (SELECT user_id FROM activity_fund_ledger l JOIN activity_consumption_campaigns c ON c.id=$1 WHERE l.created_at>=c.starts_at AND l.created_at<LEAST(c.ends_at,COALESCE(c.stopped_at,c.ends_at)) AND l.user_id%100<c.rollout_percent GROUP BY user_id HAVING SUM(eligible_cost)>0) participants`, c.ID).Scan(&c.Participants)
		if err != nil {
			return nil, err
		}
	}
	return campaigns, nil
}
func validateConsumptionCampaign(c ConsumptionCampaign) error {
	if c.Name == "" || len(c.Name) > 100 || c.RolloutPercent < 1 || c.RolloutPercent > 100 || c.BudgetCents < 0 || !c.EndsAt.After(c.StartsAt) || c.ClaimUntil.Before(c.EndsAt.Add(24*time.Hour)) || len(c.Tiers) != 3 {
		return fmt.Errorf("invalid campaign: name, dates, coverage and exactly three tiers are required")
	}
	var previous, total int64
	for i, t := range c.Tiers {
		if t.Position != i+1 || t.ThresholdCents <= previous || t.RewardCents <= 0 || t.RewardCents > math.MaxInt32 {
			return fmt.Errorf("thresholds must increase and rewards must be positive")
		}
		previous = t.ThresholdCents
		total += t.RewardCents
	}
	if c.BudgetCents < total {
		return fmt.Errorf("campaign budget must cover all three rewards for one user")
	}
	return nil
}
func consumptionAudit(ctx context.Context, tx *sql.Tx, actor int64, action string, target int64, detail any) error {
	b, err := json.Marshal(detail)
	if err != nil {
		return err
	}
	_, err = tx.ExecContext(ctx, `INSERT INTO activity_consumption_audit(actor_id,action,target_id,detail) VALUES($1,$2,$3,$4::jsonb)`, actor, action, target, string(b))
	return err
}
func (s *ActivityService) SaveConsumptionCampaign(ctx context.Context, actor int64, c ConsumptionCampaign) (int64, error) {
	if err := validateConsumptionCampaign(c); err != nil {
		return 0, err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, err
	}
	defer tx.Rollback()
	var shared, userCap int64
	if err = tx.QueryRowContext(ctx, `SELECT total_budget_cents,user_lifetime_cap_cents FROM activity_config WHERE id=1 FOR UPDATE`).Scan(&shared, &userCap); err != nil {
		return 0, err
	}
	if c.ID == 0 {
		err = tx.QueryRowContext(ctx, `INSERT INTO activity_consumption_campaigns(name,starts_at,ends_at,claim_until,rollout_percent,budget_cents) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, c.Name, c.StartsAt, c.EndsAt, c.ClaimUntil, c.RolloutPercent, c.BudgetCents).Scan(&c.ID)
	} else {
		old, e := scanCampaign(tx.QueryRowContext(ctx, `SELECT `+consumptionColumns+` FROM activity_consumption_campaigns WHERE id=$1 FOR UPDATE`, c.ID))
		if e != nil {
			return 0, e
		}
		if old.PublishedAt != nil {
			if c.BudgetCents > shared {
				return 0, fmt.Errorf("increase the shared activity budget before increasing this campaign budget")
			}
			if c.BudgetCents < old.BudgetCents {
				return 0, fmt.Errorf("published campaign budget may only increase")
			}
			_, err = tx.ExecContext(ctx, `UPDATE activity_consumption_campaigns SET budget_cents=$2 WHERE id=$1`, c.ID, c.BudgetCents)
		} else {
			_, err = tx.ExecContext(ctx, `UPDATE activity_consumption_campaigns SET name=$2,starts_at=$3,ends_at=$4,claim_until=$5,rollout_percent=$6,budget_cents=$7 WHERE id=$1`, c.ID, c.Name, c.StartsAt, c.EndsAt, c.ClaimUntil, c.RolloutPercent, c.BudgetCents)
		}
		if err != nil {
			return 0, err
		}
		if old.PublishedAt != nil {
			if err = consumptionAudit(ctx, tx, actor, "budget_increase", c.ID, c); err != nil {
				return 0, err
			}
			return c.ID, tx.Commit()
		}
	}
	if err != nil {
		return 0, err
	}
	if _, err = tx.ExecContext(ctx, `DELETE FROM activity_consumption_tiers WHERE campaign_id=$1`, c.ID); err != nil {
		return 0, err
	}
	for _, t := range c.Tiers {
		if _, err = tx.ExecContext(ctx, `INSERT INTO activity_consumption_tiers(campaign_id,position,threshold_cents,reward_cents) VALUES($1,$2,$3,$4)`, c.ID, t.Position, t.ThresholdCents, t.RewardCents); err != nil {
			return 0, err
		}
	}
	if err = consumptionAudit(ctx, tx, actor, "save_draft", c.ID, c); err != nil {
		return 0, err
	}
	return c.ID, tx.Commit()
}
func (s *ActivityService) SetConsumptionState(ctx context.Context, actor, id int64, state string) error {
	if state != "active" && state != "paused" && state != "closed" {
		return fmt.Errorf("invalid campaign state")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var shared, userCap int64
	if err = tx.QueryRowContext(ctx, `SELECT total_budget_cents,user_lifetime_cap_cents FROM activity_config WHERE id=1 FOR UPDATE`).Scan(&shared, &userCap); err != nil {
		return err
	}
	c, err := scanCampaign(tx.QueryRowContext(ctx, `SELECT `+consumptionColumns+` FROM activity_consumption_campaigns WHERE id=$1 FOR UPDATE`, id))
	if err != nil {
		return err
	}
	if c.State == "closed" {
		return fmt.Errorf("closed campaigns cannot be reopened")
	}
	if state == "active" {
		if c.PublishedAt != nil && c.State != "active" {
			return fmt.Errorf("paused campaigns cannot resume; create a new campaign to avoid counting paused consumption")
		}
		if !c.StartsAt.After(time.Now()) && c.PublishedAt == nil {
			return fmt.Errorf("publish before the start time; past consumption is not retroactive")
		}
		if !c.EndsAt.After(time.Now()) {
			return fmt.Errorf("campaign has ended")
		}
		c.Tiers, err = consumptionTiers(ctx, tx, id)
		if err != nil {
			return err
		}
		if err = validateConsumptionCampaign(*c); err != nil {
			return err
		}
		var sum int64
		for _, t := range c.Tiers {
			sum += t.RewardCents
		}
		if shared < c.BudgetCents || userCap < sum {
			return fmt.Errorf("shared budget must cover campaign budget and user cap must cover all tiers; check-in also consumes these limits")
		}
		var overlaps bool
		if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM activity_consumption_campaigns WHERE id<>$1 AND state IN ('active','paused') AND starts_at<$3 AND ends_at>$2)`, id, c.StartsAt, c.EndsAt).Scan(&overlaps); err != nil {
			return err
		}
		if overlaps {
			return fmt.Errorf("campaign dates overlap an existing published campaign")
		}
		if _, err = tx.ExecContext(ctx, `UPDATE activity_config SET second_recharge_enabled=false WHERE id=1`); err != nil {
			return err
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE activity_consumption_campaigns SET state=$2,stopped_at=CASE WHEN $2 IN ('paused','closed') THEN COALESCE(stopped_at,NOW()) ELSE stopped_at END,published_at=CASE WHEN $2='active' THEN COALESCE(published_at,NOW()) ELSE published_at END WHERE id=$1`, id, state); err != nil {
		return err
	}
	if err = consumptionAudit(ctx, tx, actor, "state:"+state, id, nil); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *ActivityService) ConsumptionStatus(ctx context.Context, uid int64) (*ConsumptionSummary, error) {
	out := &ConsumptionSummary{Spend: "0", History: []ConsumptionHistory{}, Funds: ConsumptionFunds{Principal: "0", Gift: "0", Unverified: "0"}}
	var visible bool
	if err := s.db.QueryRowContext(ctx, `SELECT activity_audience_allowed('consumption',$1)`, uid).Scan(&visible); err != nil {
		return nil, err
	}
	if !visible {
		return out, nil
	}

	if err := s.db.QueryRowContext(ctx, `SELECT awards_enabled,EXISTS(SELECT 1 FROM activity_consumption_outbox WHERE user_id=$1 AND processed_at IS NULL) FROM activity_runtime WHERE id=1`, uid).Scan(&out.AwardsEnabled, &out.ProcessingPending); err != nil {
		return nil, err
	}
	err := s.db.QueryRowContext(ctx, `SELECT principal::text,gift::text,unverified::text FROM activity_funds WHERE user_id=$1`, uid).Scan(&out.Funds.Principal, &out.Funds.Gift, &out.Funds.Unverified)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	c, err := scanCampaign(s.db.QueryRowContext(ctx, `SELECT `+consumptionColumns+` FROM activity_consumption_campaigns c WHERE published_at IS NOT NULL AND ($1%100<rollout_percent OR EXISTS(SELECT 1 FROM activity_consumption_entitlements e WHERE e.campaign_id=c.id AND e.user_id=$1)) ORDER BY EXISTS(SELECT 1 FROM activity_consumption_entitlements e WHERE e.campaign_id=c.id AND e.user_id=$1 AND e.state IN ('reserved','review') AND c.claim_until>NOW()) DESC,(c.starts_at<=NOW() AND c.ends_at>NOW()) DESC,id DESC LIMIT 1`, uid))
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return nil, err
	}
	if err == nil {
		out.Campaign = c
		c.Tiers, err = consumptionTiers(ctx, s.db, c.ID)
		if err != nil {
			return nil, err
		}
		if err = s.db.QueryRowContext(ctx, `SELECT activity_consumption_spend($1,$2)::text`, c.ID, uid).Scan(&out.Spend); err != nil {
			return nil, err
		}
		if err = s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM activity_consumption_entitlements WHERE user_id=$1 AND state='review') OR EXISTS(SELECT 1 FROM activity_refund_reviews WHERE user_id=$1 AND review_status='pending')`, uid).Scan(&out.Blocked); err != nil {
			return nil, err
		}
		for i := range c.Tiers {
			t := &c.Tiers[i]
			var state string
			err = s.db.QueryRowContext(ctx, `SELECT state FROM activity_consumption_entitlements WHERE campaign_id=$1 AND user_id=$2 AND position=$3`, c.ID, uid, t.Position).Scan(&state)
			if err != nil && !errors.Is(err, sql.ErrNoRows) {
				return nil, err
			}
			t.State = "locked"
			if state != "" {
				t.State = state
			}
			if state == "reserved" && time.Now().After(c.ClaimUntil) {
				t.State = "expired"
			}
			if t.State == "reserved" && !out.Blocked && out.AwardsEnabled {
				out.ClaimableCents += t.RewardCents
			}
			spend, _ := decimal.NewFromString(out.Spend)
			if state == "" && !out.ProcessingPending && spend.Mul(decimal.NewFromInt(100)).GreaterThanOrEqual(decimal.NewFromInt(t.ThresholdCents)) {
				out.BudgetBlocked = true
			}
		}
	}
	rows, err := s.db.QueryContext(ctx, `SELECT c.name,e.position,e.reward_cents,CASE WHEN e.state='reserved' AND c.claim_until<=NOW() THEN 'expired' ELSE e.state END,e.created_at FROM activity_consumption_entitlements e JOIN activity_consumption_campaigns c ON c.id=e.campaign_id WHERE e.user_id=$1 ORDER BY e.created_at DESC LIMIT 50`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	for rows.Next() {
		var h ConsumptionHistory
		if err = rows.Scan(&h.Campaign, &h.Position, &h.RewardCents, &h.State, &h.CreatedAt); err != nil {
			return nil, err
		}
		out.History = append(out.History, h)
	}
	return out, rows.Err()
}
func (s *ActivityService) ClaimConsumption(ctx context.Context, uid, cid int64, position int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var active bool
	if err = tx.QueryRowContext(ctx, `SELECT status='active' FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, uid).Scan(&active); err != nil {
		return err
	}
	if !active {
		return ErrActivityIneligible
	}
	if err := activityAudienceAllowed(ctx, tx, "consumption", uid); err != nil {
		return err
	}
	if err := activityAwardsAllowed(ctx, tx); err != nil {
		return err
	}
	var lock int
	if err = tx.QueryRowContext(ctx, `SELECT id FROM activity_config WHERE id=1 FOR UPDATE`).Scan(&lock); err != nil {
		return err
	}
	var blocked bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM activity_consumption_entitlements WHERE user_id=$1 AND state='review') OR EXISTS(SELECT 1 FROM activity_refund_reviews WHERE user_id=$1 AND review_status='pending')`, uid).Scan(&blocked); err != nil {
		return err
	}
	if blocked {
		return fmt.Errorf("rewards are awaiting refund or settlement review")
	}
	rows, err := tx.QueryContext(ctx, `SELECT e.position,e.reward_cents FROM activity_consumption_entitlements e JOIN activity_consumption_campaigns c ON c.id=e.campaign_id WHERE e.user_id=$1 AND e.campaign_id=$2 AND e.state='reserved' AND c.claim_until>NOW() AND ($3=0 OR e.position=$3) ORDER BY e.position FOR UPDATE OF e`, uid, cid, position)
	if err != nil {
		return err
	}
	type reward struct {
		position int
		amount   int64
	}
	rewards := []reward{}
	for rows.Next() {
		var r reward
		if err = rows.Scan(&r.position, &r.amount); err != nil {
			rows.Close()
			return err
		}
		rewards = append(rewards, r)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, r := range rewards {
		if _, err = tx.ExecContext(ctx, `INSERT INTO activity_reward_ledger(user_id,kind,source_id,amount_cents) VALUES($1,'consumption',$2,$3)`, uid, fmt.Sprintf("%d:%d:%d", cid, uid, r.position), r.amount); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE activity_consumption_entitlements SET state='claimed',claimed_at=NOW() WHERE campaign_id=$1 AND user_id=$2 AND position=$3`, cid, uid, r.position); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE users SET balance=balance+$2::numeric/100,updated_at=NOW() WHERE id=$1`, uid, r.amount); err != nil {
			return err
		}
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	s.invalidateBalance(ctx, uid)
	return nil
}

// Reconcile unverified opening funds without changing spendable total balance.
func (s *ActivityService) ReconcileConsumptionFunds(ctx context.Context, actor, uid int64, amount string, kind, note string) error {
	value, err := decimal.NewFromString(amount)
	if err != nil || !value.IsPositive() || !value.Equal(value.Round(8)) || note == "" || (kind != "principal" && kind != "gift") {
		return fmt.Errorf("positive amount (up to 8 decimals), source and audit note are required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var id int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=$1 AND deleted_at IS NULL FOR UPDATE`, uid).Scan(&id); err != nil {
		return err
	}
	var available string
	if err = tx.QueryRowContext(ctx, `SELECT unverified::text FROM activity_funds WHERE user_id=$1 FOR UPDATE`, uid).Scan(&available); err != nil {
		return err
	}
	balance, _ := decimal.NewFromString(available)
	if value.GreaterThan(balance) {
		return fmt.Errorf("amount exceeds remaining unverified funds")
	}
	if _, err = tx.ExecContext(ctx, `UPDATE activity_funds SET unverified=unverified-$2::numeric,principal=principal+CASE WHEN $3='principal' THEN $2::numeric ELSE 0 END,gift=gift+CASE WHEN $3='gift' THEN $2::numeric ELSE 0 END WHERE user_id=$1`, uid, value.String(), kind); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO activity_fund_ledger(user_id,source,principal,gift,unverified,note) VALUES($1,'reconcile',CASE WHEN $3='principal' THEN $2::numeric ELSE 0 END,CASE WHEN $3='gift' THEN $2::numeric ELSE 0 END,-$2::numeric,$4)`, uid, value.String(), kind, note); err != nil {
		return err
	}
	if err = consumptionAudit(ctx, tx, actor, "reconcile", uid, map[string]string{"amount": value.String(), "source": kind, "note": note}); err != nil {
		return err
	}
	return tx.Commit()
}

// Correct a confirmed settlement's reward progress; no balance adjustment is implied.
func (s *ActivityService) CorrectConsumption(ctx context.Context, actor, ledgerID int64, note string) error {
	if note == "" {
		return fmt.Errorf("audit note is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var uid int64
	if err = tx.QueryRowContext(ctx, `SELECT user_id FROM activity_fund_ledger WHERE id=$1`, ledgerID).Scan(&uid); err != nil {
		return err
	}
	var id int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, uid).Scan(&id); err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, `SELECT id FROM activity_config WHERE id=1 FOR UPDATE`).Scan(&id); err != nil {
		return err
	}
	var cost string
	var occurred time.Time
	if err = tx.QueryRowContext(ctx, `SELECT eligible_cost::text,created_at FROM activity_fund_ledger WHERE id=$1 AND source IN ('usage','batch_capture') AND eligible_cost>0 FOR UPDATE`, ledgerID).Scan(&cost, &occurred); err != nil {
		return err
	}
	var exists bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM activity_fund_ledger WHERE source=$1)`, fmt.Sprintf("correction:%d", ledgerID)).Scan(&exists); err != nil {
		return err
	}
	if exists {
		return fmt.Errorf("settlement already corrected")
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO activity_fund_ledger(user_id,source,eligible_cost,created_at,note) VALUES($1,$2,-$3::numeric,$4,$5)`, uid, fmt.Sprintf("correction:%d", ledgerID), cost, occurred, note); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE activity_consumption_entitlements e SET state=CASE WHEN e.state='claimed' THEN 'review' ELSE 'revoked' END FROM activity_consumption_tiers t WHERE e.campaign_id=t.campaign_id AND e.position=t.position AND e.user_id=$1 AND e.state IN ('reserved','claimed') AND activity_consumption_spend(e.campaign_id,$1)*100<t.threshold_cents`, uid); err != nil {
		return err
	}
	if err = consumptionAudit(ctx, tx, actor, "settlement_correction", ledgerID, map[string]string{"note": note}); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *ActivityService) ResolveConsumptionReview(ctx context.Context, actor, uid int64, approve bool, note string) error {
	if note == "" {
		return fmt.Errorf("audit note is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var lock int64
	if err = tx.QueryRowContext(ctx, `SELECT id FROM users WHERE id=$1 FOR UPDATE`, uid).Scan(&lock); err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, `SELECT id FROM activity_config WHERE id=1 FOR UPDATE`).Scan(&lock); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE activity_refund_reviews SET review_status='reviewed',updated_at=NOW() WHERE user_id=$1`, uid); err != nil {
		return err
	}
	state := "revoked"
	if approve {
		state = "reserved"
	}
	if _, err = tx.ExecContext(ctx, `UPDATE activity_consumption_entitlements e SET state=CASE WHEN claimed_at IS NOT NULL THEN 'claimed' WHEN c.claim_until<=NOW() THEN 'expired' ELSE $2 END FROM activity_consumption_campaigns c WHERE e.campaign_id=c.id AND e.user_id=$1 AND e.state='review'`, uid, state); err != nil {
		return err
	}
	if err = consumptionAudit(ctx, tx, actor, "review_resolution", uid, map[string]any{"approve": approve, "note": note}); err != nil {
		return err
	}
	return tx.Commit()
}
func (s *ActivityService) ConsumptionAudit(ctx context.Context) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,actor_id,action,target_id,detail::text,created_at FROM activity_consumption_audit ORDER BY id DESC LIMIT 100`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id, actor int64
		var target sql.NullInt64
		var action, detail string
		var at time.Time
		if err = rows.Scan(&id, &actor, &action, &target, &detail, &at); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"id": id, "actor_id": actor, "action": action, "target_id": target.Int64, "detail": detail, "created_at": at})
	}
	return items, rows.Err()
}
func (s *ActivityService) ConsumptionFundLedger(ctx context.Context, uid int64) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,source,principal::text,gift::text,unverified::text,eligible_cost::text,created_at,note FROM activity_fund_ledger WHERE user_id=$1 ORDER BY id DESC LIMIT 100`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var src, p, g, u, c, note string
		var at time.Time
		if err = rows.Scan(&id, &src, &p, &g, &u, &c, &at, &note); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{"id": id, "source": src, "principal": p, "gift": g, "unverified": u, "eligible_cost": c, "created_at": at, "note": note})
	}
	return items, rows.Err()
}
