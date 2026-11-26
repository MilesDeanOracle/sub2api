package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Wei-Shaw/sub2api/internal/repository/activitystore"
	"math"
	"time"

	"github.com/shopspring/decimal"
)

type ConsumptionConfig struct {
	Enabled bool `json:"enabled"`
}

func (s *ActivityService) ConsumptionConfig(ctx context.Context) (*ConsumptionConfig, error) {
	out := &ConsumptionConfig{}
	err := s.db.QueryRowContext(ctx, `SELECT enabled FROM activity_consumption_config WHERE id=1`).Scan(&out.Enabled)
	return out, err
}

func (s *ActivityService) SetConsumptionEnabled(ctx context.Context, actor int64, enabled bool) (*ConsumptionConfig, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return nil, err
	}
	defer tx.Rollback()
	var old bool
	if err := tx.QueryRowContext(ctx, `SELECT enabled FROM activity_consumption_config WHERE id=1 FOR UPDATE`).Scan(&old); err != nil {
		return nil, err
	}
	if old != enabled {
		if enabled {
			_, err = tx.ExecContext(ctx, `UPDATE activity_consumption_pauses SET ended_at=clock_timestamp() WHERE ended_at IS NULL`)
		} else {
			_, err = tx.ExecContext(ctx, `INSERT INTO activity_consumption_pauses(started_at) VALUES(clock_timestamp())`)
		}
		if err != nil {
			return nil, err
		}
		if _, err = tx.ExecContext(ctx, `UPDATE activity_consumption_config SET enabled=$1,updated_at=clock_timestamp() WHERE id=1`, enabled); err != nil {
			return nil, err
		}
		if err = consumptionAudit(ctx, tx, actor, "consumption_master_switch", 1, map[string]any{"before_enabled": old, "enabled": enabled}); err != nil {
			return nil, err
		}
	}
	if err := tx.Commit(); err != nil {
		return nil, err
	}
	return &ConsumptionConfig{Enabled: enabled}, nil
}

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
	Enabled            bool                  `json:"enabled"`
	Campaigns          []ConsumptionCampaign `json:"campaigns"`
	BlockReason        string                `json:"block_reason,omitempty"`
	UserRemainingCents int64                 `json:"user_remaining_cents"`
	AwardsEnabled      bool                  `json:"awards_enabled"`
	ProcessingPending  bool                  `json:"processing_pending"`
	Campaign           *ConsumptionCampaign  `json:"campaign"`
	Spend              string                `json:"spend"`
	ClaimableCents     int64                 `json:"claimable_cents"`
	Blocked            bool                  `json:"blocked"`
	BudgetBlocked      bool                  `json:"budget_blocked"`
	Wallet             ActivityWalletSummary `json:"wallet"`
	History            []ConsumptionHistory  `json:"history"`
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
		err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM (SELECT e.user_id FROM activity_usage_events e JOIN activity_consumption_campaigns c ON c.id=$1 WHERE e.reversed_at IS NULL AND activity_consumption_enabled_at(e.occurred_at) AND e.occurred_at>=c.starts_at AND e.occurred_at<LEAST(c.ends_at,COALESCE(c.stopped_at,c.ends_at)) AND activity_rollout_allowed('consumption', c.rollout_percent, e.user_id) GROUP BY e.user_id HAVING SUM(e.eligible_cost)>0) participants`, c.ID).Scan(&c.Participants)
		if err != nil {
			return nil, err
		}
	}
	return campaigns, nil
}

func validateConsumptionCampaign(c ConsumptionCampaign) error {
	if c.Name == "" || len(c.Name) > 100 || c.RolloutPercent < 1 || c.RolloutPercent > 100 || c.BudgetCents < 0 || !c.EndsAt.After(c.StartsAt) || c.ClaimUntil.Before(c.EndsAt.Add(24*time.Hour)) || len(c.Tiers) < 1 || len(c.Tiers) > 5 {
		return fmt.Errorf("invalid campaign: name, dates, coverage and 1 to 5 tiers are required")
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
		return fmt.Errorf("campaign budget must cover all tier rewards for at least one user")
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
	if c.ID == 0 {
		err = tx.QueryRowContext(ctx, `INSERT INTO activity_consumption_campaigns(name,starts_at,ends_at,claim_until,rollout_percent,budget_cents) VALUES($1,$2,$3,$4,$5,$6) RETURNING id`, c.Name, c.StartsAt, c.EndsAt, c.ClaimUntil, c.RolloutPercent, c.BudgetCents).Scan(&c.ID)
	} else {
		old, e := scanCampaign(tx.QueryRowContext(ctx, `SELECT `+consumptionColumns+` FROM activity_consumption_campaigns WHERE id=$1 FOR UPDATE`, c.ID))
		if e != nil {
			return 0, e
		}
		if old.PublishedAt != nil {
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
	var userCap int64
	if err = tx.QueryRowContext(ctx, `SELECT user_lifetime_cap_cents FROM activity_config WHERE id=1 FOR UPDATE`).Scan(&userCap); err != nil {
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
		now := time.Now()
		if !c.EndsAt.After(now) {
			return fmt.Errorf("campaign has ended")
		}
		if c.PublishedAt == nil && !c.StartsAt.After(now) {
			c.StartsAt = now
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
		if userCap < sum {
			return fmt.Errorf("user cap must cover all tiers; check-in also consumes this limit")
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
	if _, err = tx.ExecContext(ctx, `
		UPDATE activity_consumption_campaigns
		SET state = $2,
		    starts_at = CASE WHEN $2 = 'active' AND published_at IS NULL AND starts_at <= NOW() THEN NOW() ELSE starts_at END,
		    stopped_at = CASE WHEN $2 IN ('paused', 'closed') THEN COALESCE(stopped_at, NOW()) ELSE stopped_at END,
		    published_at = CASE WHEN $2 = 'active' THEN COALESCE(published_at, NOW()) ELSE published_at END
		WHERE id = $1
	`, id, state); err != nil {
		return err
	}
	if err = consumptionAudit(ctx, tx, actor, "state:"+state, id, nil); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *ActivityService) ConsumptionStatus(ctx context.Context, uid int64, selected ...int64) (*ConsumptionSummary, error) {
	out := &ConsumptionSummary{
		Spend:     "0",
		Campaigns: []ConsumptionCampaign{},
		History:   []ConsumptionHistory{},
		Wallet:    ActivityWalletSummary{UnabsorbedGift: "0", Ledger: []ActivityWalletLedgerItem{}},
	}
	var visible bool
	if err := s.db.QueryRowContext(ctx, `SELECT activity_audience_allowed('consumption',$1) OR EXISTS(SELECT 1 FROM activity_consumption_entitlements WHERE user_id=$1)`, uid).Scan(&visible); err != nil {
		return nil, err
	}
	if !visible {
		return out, nil
	}

	wallet, err := s.loadWalletSummary(ctx, uid, true)
	if err != nil {
		return nil, err
	}
	out.Wallet = wallet

	out.Enabled, out.AwardsEnabled, out.ProcessingPending, err = activitystore.ConsumptionRuntime(ctx, s.db, uid)
	if err != nil {
		return nil, err
	}

	var selectedID int64
	if len(selected) > 0 {
		selectedID = selected[0]
	}
	campaignRows, err := s.db.QueryContext(ctx, `SELECT `+consumptionColumns+` FROM activity_consumption_campaigns c WHERE published_at IS NOT NULL AND (activity_rollout_allowed('consumption',c.rollout_percent,$1) OR EXISTS(SELECT 1 FROM activity_consumption_entitlements e WHERE e.campaign_id=c.id AND e.user_id=$1)) ORDER BY (c.state='active' AND c.starts_at<=NOW() AND c.ends_at>NOW()) DESC,id DESC LIMIT 50`, uid)
	if err != nil {
		return nil, err
	}
	for campaignRows.Next() {
		c, err := scanCampaign(campaignRows)
		if err != nil {
			campaignRows.Close()
			return nil, err
		}
		out.Campaigns = append(out.Campaigns, *c)
	}
	err = campaignRows.Err()
	campaignRows.Close()
	if err != nil {
		return nil, err
	}
	c, err := scanCampaign(s.db.QueryRowContext(ctx, `SELECT `+consumptionColumns+` FROM activity_consumption_campaigns c WHERE published_at IS NOT NULL AND ($2::bigint=0 OR c.id=$2) AND (activity_rollout_allowed('consumption', c.rollout_percent, $1) OR EXISTS(SELECT 1 FROM activity_consumption_entitlements e WHERE e.campaign_id=c.id AND e.user_id=$1)) ORDER BY EXISTS(SELECT 1 FROM activity_consumption_entitlements e WHERE e.campaign_id=c.id AND e.user_id=$1 AND e.state='reserved' AND c.claim_until>NOW()) DESC,CASE WHEN EXISTS(SELECT 1 FROM activity_consumption_entitlements e WHERE e.campaign_id=c.id AND e.user_id=$1 AND e.state='reserved' AND c.claim_until>NOW()) THEN c.claim_until END ASC,(c.state='active' AND c.starts_at<=NOW() AND c.ends_at>NOW()) DESC,id DESC LIMIT 1`, uid, selectedID))
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
		if err = s.db.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM activity_consumption_entitlements WHERE user_id=$1 AND state='review') OR EXISTS(SELECT 1 FROM activity_refund_reviews WHERE user_id=$1 AND review_status='pending') OR EXISTS(SELECT 1 FROM activity_wallets WHERE user_id=$1 AND recovery_debt_cents>0) OR EXISTS(SELECT 1 FROM payment_orders WHERE user_id=$1 AND status IN ('REFUND_REQUESTED','REFUNDING','REFUND_PENDING'))`, uid).Scan(&out.Blocked); err != nil {
			return nil, err
		}
		var campaignUsed, personalUsed, userCap int64
		if err := s.db.QueryRowContext(ctx, `SELECT COALESCE(SUM(reward_cents),0) FROM activity_consumption_entitlements WHERE campaign_id=$1 AND (claimed_at IS NOT NULL OR (state IN ('reserved','review') AND (state='review' OR $2>NOW())))`, c.ID, c.ClaimUntil).Scan(&campaignUsed); err != nil {
			return nil, err
		}
		if err := s.db.QueryRowContext(ctx, `SELECT user_lifetime_cap_cents,(SELECT COALESCE(SUM(amount_cents),0) FROM activity_reward_ledger WHERE user_id=$1 AND kind<>'lottery')+activity_reserved_cents($1) FROM activity_config WHERE id=1`, uid).Scan(&userCap, &personalUsed); err != nil {
			return nil, err
		}
		out.UserRemainingCents = userCap - personalUsed
		if out.UserRemainingCents < 0 {
			out.UserRemainingCents = 0
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
			if state == "" && spend.Mul(decimal.NewFromInt(100)).GreaterThanOrEqual(decimal.NewFromInt(t.ThresholdCents)) {
				if t.RewardCents > out.UserRemainingCents {
					out.BlockReason = "user_cap"
				}
				if t.RewardCents > c.BudgetCents-campaignUsed {
					out.BudgetBlocked = true
				}
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

	if err := activitystore.LockWallet(ctx, tx, uid); err != nil {
		return err
	}

	var active bool
	if err = tx.QueryRowContext(ctx, `SELECT status='active' FROM users WHERE id=$1 AND deleted_at IS NULL`, uid).Scan(&active); err != nil {
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
	if err := syncUserRefundReviewsTx(ctx, tx, uid); err != nil {
		return err
	}
	var lock int
	if err = tx.QueryRowContext(ctx, `SELECT id FROM activity_config WHERE id=1 FOR UPDATE`).Scan(&lock); err != nil {
		return err
	}
	var blocked bool
	if err = tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM activity_consumption_entitlements WHERE user_id=$1 AND state='review') OR EXISTS(SELECT 1 FROM activity_refund_reviews WHERE user_id=$1 AND review_status='pending') OR EXISTS(SELECT 1 FROM activity_wallets WHERE user_id=$1 AND recovery_debt_cents>0) OR EXISTS(SELECT 1 FROM payment_orders WHERE user_id=$1 AND status IN ('REFUND_REQUESTED','REFUNDING','REFUND_PENDING'))`, uid).Scan(&blocked); err != nil {
		return err
	}
	if blocked {
		return ErrActivityReviewBlocked
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
	if len(rewards) == 0 {
		var claimed bool
		if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM activity_consumption_entitlements WHERE campaign_id=$1 AND user_id=$2 AND state='claimed' AND ($3=0 OR position=$3))`, cid, uid, position).Scan(&claimed); err != nil {
			return err
		}
		if !claimed {
			return fmt.Errorf("奖励尚未可领取、已过期或已撤销，请刷新活动进度")
		}
	}
	for _, r := range rewards {
		if _, err = tx.ExecContext(ctx, `UPDATE activity_consumption_entitlements SET state='claimed',claimed_at=NOW() WHERE campaign_id=$1 AND user_id=$2 AND position=$3`, cid, uid, r.position); err != nil {
			return err
		}
		if err := creditActivityWalletTx(ctx, tx, uid, "consumption", fmt.Sprintf("%d:%d:%d", cid, uid, r.position), r.amount); err != nil {
			return err
		}
	}
	return tx.Commit()
}

// CorrectConsumption reverses an ingested usage event's reward progress without
// touching sub2api's users or usage_logs tables.
func (s *ActivityService) CorrectConsumption(ctx context.Context, actor, usageLogID int64, note string) error {
	if note == "" {
		return fmt.Errorf("audit note is required")
	}
	if usageLogID == 0 {
		return fmt.Errorf("usage event ID is required")
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	var uid int64
	var reversed sql.NullTime
	if err = tx.QueryRowContext(ctx, `SELECT user_id, reversed_at FROM activity_usage_events WHERE usage_log_id=$1 FOR UPDATE`, usageLogID).Scan(&uid, &reversed); err != nil {
		return err
	}
	if reversed.Valid {
		return fmt.Errorf("settlement already corrected")
	}
	if _, err := tx.ExecContext(ctx, `INSERT INTO activity_wallets(user_id) VALUES($1) ON CONFLICT DO NOTHING`, uid); err != nil {
		return err
	}
	var lock int64
	if err = tx.QueryRowContext(ctx, `SELECT user_id FROM activity_wallets WHERE user_id=$1 FOR UPDATE`, uid).Scan(&lock); err != nil {
		return err
	}
	if err = tx.QueryRowContext(ctx, `SELECT id FROM activity_config WHERE id=1 FOR UPDATE`).Scan(&lock); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE activity_usage_events SET reversed_at=NOW(), reverse_note=$2 WHERE usage_log_id=$1`, usageLogID, note); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE activity_consumption_entitlements e SET state=CASE WHEN e.state='claimed' THEN 'review' ELSE 'revoked' END FROM activity_consumption_tiers t WHERE e.campaign_id=t.campaign_id AND e.position=t.position AND e.user_id=$1 AND e.state IN ('reserved','claimed') AND activity_consumption_spend(e.campaign_id,$1)*100<t.threshold_cents`, uid); err != nil {
		return err
	}
	if err = consumptionAudit(ctx, tx, actor, "settlement_correction", usageLogID, map[string]string{"note": note}); err != nil {
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
	if err := activitystore.LockWallet(ctx, tx, uid); err != nil {
		return err
	}
	var lock int64

	if err = tx.QueryRowContext(ctx, `SELECT id FROM activity_config WHERE id=1 FOR UPDATE`).Scan(&lock); err != nil {
		return err
	}
	if err := settleUserRefundRewardsTx(ctx, tx, uid); err != nil {
		return err
	}
	var inflight bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM payment_orders WHERE user_id=$1 AND status IN ('REFUND_REQUESTED','REFUNDING','REFUND_PENDING'))`, uid).Scan(&inflight); err != nil {
		return err
	}
	if inflight {
		return ErrActivityReviewBlocked
	}
	if approve {
		if err := collectActivityRecoveryDebt(ctx, tx, uid); err != nil {
			return err
		}
	}
	if !approve {
		rows, err := tx.QueryContext(ctx, `SELECT campaign_id,position,reward_cents FROM activity_consumption_entitlements WHERE user_id=$1 AND state='review' AND claimed_at IS NOT NULL`, uid)
		if err != nil {
			return err
		}
		type recovery struct {
			cid    int64
			pos    int
			amount int64
		}
		var recoveries []recovery
		for rows.Next() {
			var r recovery
			if err := rows.Scan(&r.cid, &r.pos, &r.amount); err != nil {
				rows.Close()
				return err
			}
			recoveries = append(recoveries, r)
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		for _, r := range recoveries {
			if err := recoverActivityReward(ctx, tx, uid, "consumption_recovery", fmt.Sprintf("%d:%d:%d", r.cid, uid, r.pos), r.amount, decimal.Zero, true, "settled"); err != nil {
				return err
			}
		}
	}
	if _, err = tx.ExecContext(ctx, `UPDATE activity_refund_reviews SET review_status='reviewed',updated_at=NOW() WHERE user_id=$1`, uid); err != nil {
		return err
	}
	state := "revoked"
	if approve {
		state = "reserved"
	}
	if _, err = tx.ExecContext(ctx, `UPDATE activity_consumption_entitlements e SET state=CASE WHEN claimed_at IS NOT NULL AND $3 THEN 'claimed' WHEN claimed_at IS NOT NULL THEN 'revoked' WHEN c.claim_until<=NOW() THEN 'expired' ELSE $2 END FROM activity_consumption_campaigns c WHERE e.campaign_id=c.id AND e.user_id=$1 AND e.state='review'`, uid, state, approve); err != nil {
		return err
	}
	if err = consumptionAudit(ctx, tx, actor, "review_resolution", uid, map[string]any{"approve": approve, "note": note}); err != nil {
		return err
	}
	if err := tx.Commit(); err != nil {
		return err
	}
	s.invalidateBalance(ctx, uid)
	return nil
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

func (s *ActivityService) ConsumptionUsageLedger(ctx context.Context, uid int64) ([]map[string]any, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT usage_log_id,request_id,actual_cost::text,absorbed_gift::text,eligible_cost::text,occurred_at,reversed_at IS NOT NULL,reverse_note FROM activity_usage_events WHERE user_id=$1 ORDER BY occurred_at DESC, usage_log_id ASC LIMIT 100`, uid)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []map[string]any{}
	for rows.Next() {
		var id int64
		var reqID, actual, absorbed, eligible, note string
		var at time.Time
		var reversed bool
		if err = rows.Scan(&id, &reqID, &actual, &absorbed, &eligible, &at, &reversed, &note); err != nil {
			return nil, err
		}
		items = append(items, map[string]any{
			"id":            id,
			"request_id":    reqID,
			"actual_cost":   actual,
			"absorbed_gift": absorbed,
			"eligible_cost": eligible,
			"occurred_at":   at,
			"reversed":      reversed,
			"note":          note,
		})
	}
	return items, rows.Err()
}
