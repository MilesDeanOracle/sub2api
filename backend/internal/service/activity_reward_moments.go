package service

import (
	"context"
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/lib/pq"
)

type RewardMomentsConfig struct {
	Enabled bool    `json:"enabled"`
	UserIDs []int64 `json:"user_ids"`
}

type RewardSharing struct {
	Allowed        bool `json:"allowed"`
	DisplayEnabled bool `json:"display_enabled"`
}

type RewardMoment struct {
	ID           string    `json:"id"`
	Alias        string    `json:"alias"`
	Kind         string    `json:"kind"`
	CampaignID   *int64    `json:"campaign_id,omitempty"`
	CampaignName string    `json:"campaign_name,omitempty"`
	AmountCents  int64     `json:"amount_cents"`
	CreatedAt    time.Time `json:"created_at"`
}

type RewardMoments struct {
	Enabled bool           `json:"enabled"`
	Items   []RewardMoment `json:"items"`
}

func rewardMomentToken(secret, value string) string {
	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write([]byte(value))
	return hex.EncodeToString(mac.Sum(nil)[:8])
}

func (s *ActivityService) RewardSharing(ctx context.Context, uid int64) (*RewardSharing, error) {
	out := &RewardSharing{}
	err := s.db.QueryRowContext(ctx, `SELECT COALESCE((SELECT allowed FROM activity_reward_sharing WHERE user_id=$1),FALSE),enabled AND $1=ANY(user_ids) FROM activity_reward_moments_config WHERE id=1`, uid).Scan(&out.Allowed, &out.DisplayEnabled)
	return out, err
}

func (s *ActivityService) SetRewardSharing(ctx context.Context, uid int64, allowed bool) (*RewardSharing, error) {
	_, err := s.db.ExecContext(ctx, `INSERT INTO activity_reward_sharing(user_id,allowed) VALUES($1,$2)
 ON CONFLICT(user_id) DO UPDATE SET allowed=EXCLUDED.allowed,
 granted_at=CASE WHEN EXCLUDED.allowed AND NOT activity_reward_sharing.allowed THEN clock_timestamp() ELSE activity_reward_sharing.granted_at END,updated_at=clock_timestamp()`, uid, allowed)
	if err != nil {
		return nil, err
	}
	return s.RewardSharing(ctx, uid)
}

// No shared response cache: consent, scope changes and revocations are checked
// on every request. The bounded projection avoids scanning the financial ledger.
func (s *ActivityService) RewardMoments(ctx context.Context, uid int64) (*RewardMoments, error) {
	out := &RewardMoments{Items: []RewardMoment{}}
	rows, err := s.db.QueryContext(ctx, `SELECT m.ledger_id,m.user_id,m.kind,m.campaign_id,COALESCE(c.name,''),m.amount_cents,m.created_at,cfg.alias_secret
 FROM activity_reward_moments m
 JOIN activity_reward_moments_config cfg ON cfg.id=1 AND cfg.enabled AND $1=ANY(cfg.user_ids)
 JOIN activity_reward_sharing p ON p.user_id=m.user_id AND p.allowed AND m.created_at>=p.granted_at
 JOIN users u ON u.id=m.user_id AND u.status='active' AND u.deleted_at IS NULL
 LEFT JOIN activity_consumption_campaigns c ON c.id=m.campaign_id
 WHERE m.created_at>=NOW()-interval '7 days' AND m.created_at<=NOW() AND m.user_id<>$1
 AND CASE m.kind
 WHEN 'checkin' THEN activity_rollout_allowed('checkin',(SELECT checkin_rollout_percent FROM activity_config WHERE id=1),$1) AND activity_audience_allowed('checkin',m.user_id)
 WHEN 'lottery' THEN activity_audience_allowed('lottery',$1) AND activity_audience_allowed('lottery',m.user_id)
 WHEN 'consumption' THEN c.published_at IS NOT NULL AND activity_rollout_allowed('consumption',c.rollout_percent,$1) AND activity_audience_allowed('consumption',m.user_id)
 ELSE FALSE END
 ORDER BY m.created_at DESC,m.ledger_id DESC LIMIT 6`, uid)
	if err != nil {
		return nil, err
	}
	for rows.Next() {
		var item RewardMoment
		var id, owner int64
		var secret string
		if err := rows.Scan(&id, &owner, &item.Kind, &item.CampaignID, &item.CampaignName, &item.AmountCents, &item.CreatedAt, &secret); err != nil {
			rows.Close()
			return nil, err
		}
		item.ID = rewardMomentToken(secret, fmt.Sprintf("event:%d", id))
		year, week := time.Now().In(activityShanghai).ISOWeek()
		item.Alias = rewardMomentToken(secret, fmt.Sprintf("alias:%d-%d:%d", year, week, owner))
		out.Items = append(out.Items, item)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	sharing, err := s.RewardSharing(ctx, uid)
	if err != nil {
		return nil, err
	}
	out.Enabled = sharing.DisplayEnabled
	if !out.Enabled {
		out.Items = []RewardMoment{}
	}
	return out, nil
}

func (s *ActivityService) RewardMomentsConfig(ctx context.Context) (*RewardMomentsConfig, error) {
	out := &RewardMomentsConfig{UserIDs: []int64{}}
	err := s.db.QueryRowContext(ctx, `SELECT enabled,user_ids FROM activity_reward_moments_config WHERE id=1`).Scan(&out.Enabled, pq.Array(&out.UserIDs))
	return out, err
}

func (s *ActivityService) UpdateRewardMomentsConfig(ctx context.Context, actor int64, in RewardMomentsConfig) error {
	if len(in.UserIDs) > 500 {
		return fmt.Errorf("at most 500 viewer IDs are allowed")
	}
	ids := []int64{}
	seen := map[int64]bool{}
	for _, id := range in.UserIDs {
		if id <= 0 {
			return fmt.Errorf("user IDs must be positive integers")
		}
		if !seen[id] {
			ids = append(ids, id)
			seen[id] = true
		}
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id=ANY($1) AND deleted_at IS NULL`, pq.Array(ids)).Scan(&count); err != nil {
		return err
	}
	if count != len(ids) {
		return fmt.Errorf("some viewer IDs do not exist")
	}
	if _, err = tx.ExecContext(ctx, `UPDATE activity_reward_moments_config SET enabled=$1,user_ids=$2,updated_at=NOW() WHERE id=1`, in.Enabled, pq.Array(ids)); err != nil {
		return err
	}
	if err = consumptionAudit(ctx, tx, actor, "reward_moments_config", 0, RewardMomentsConfig{Enabled: in.Enabled, UserIDs: ids}); err != nil {
		return err
	}
	return tx.Commit()
}
