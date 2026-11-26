package service

import (
	"context"
	"database/sql"
	"fmt"
	"sort"

	"github.com/lib/pq"
)

type ActivityAudience struct {
	Mode    string  `json:"mode"`
	UserIDs []int64 `json:"user_ids"`
}

func validActivityKind(kind string) bool {
	return kind == "checkin" || kind == "consumption" || kind == "lottery"
}

func (s *ActivityService) Audience(ctx context.Context, kind string) (*ActivityAudience, error) {
	if !validActivityKind(kind) {
		return nil, fmt.Errorf("invalid activity kind")
	}
	out := &ActivityAudience{UserIDs: []int64{}}
	err := s.db.QueryRowContext(ctx, `SELECT mode,user_ids FROM activity_audiences WHERE kind=$1`, kind).Scan(&out.Mode, pq.Array(&out.UserIDs))
	return out, err
}

// The row share lock makes a completed scope update effective for subsequent
// claims. Lock order remains user -> audience -> runtime/config.
func activityAudienceAllowed(ctx context.Context, tx *sql.Tx, kind string, uid int64) error {
	_, err := activityAudienceModeAllowed(ctx, tx, kind, uid)
	return err
}

func activityAudienceModeAllowed(ctx context.Context, tx *sql.Tx, kind string, uid int64) (string, error) {
	var mode string
	var allowed bool
	if err := tx.QueryRowContext(ctx, `SELECT mode, mode='all' OR $2=ANY(user_ids) FROM activity_audiences WHERE kind=$1 FOR SHARE`, kind, uid).Scan(&mode, &allowed); err != nil {
		return "", err
	}
	if !allowed {
		return mode, ErrActivityIneligible
	}
	return mode, nil
}

func (s *ActivityService) UpdateAudience(ctx context.Context, actor int64, kind string, in ActivityAudience) error {
	if !validActivityKind(kind) || (in.Mode != "selected" && in.Mode != "all") {
		return fmt.Errorf("invalid audience mode or activity kind")
	}
	if len(in.UserIDs) > 500 {
		return fmt.Errorf("at most 500 test user IDs are allowed")
	}
	seen := map[int64]bool{}
	ids := []int64{}
	for _, id := range in.UserIDs {
		if id <= 0 {
			return fmt.Errorf("user IDs must be positive integers")
		}
		if !seen[id] {
			seen[id] = true
			ids = append(ids, id)
		}
	}
	sort.Slice(ids, func(i, j int) bool { return ids[i] < ids[j] })
	in.UserIDs = ids
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var old ActivityAudience
	if err = tx.QueryRowContext(ctx, `SELECT mode,user_ids FROM activity_audiences WHERE kind=$1 FOR UPDATE`, kind).Scan(&old.Mode, pq.Array(&old.UserIDs)); err != nil {
		return err
	}
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM users WHERE id=ANY($1) AND deleted_at IS NULL`, pq.Array(ids)).Scan(&count); err != nil {
		return err
	}
	if count != len(ids) {
		return fmt.Errorf("some user IDs do not exist or have been deleted")
	}
	if _, err = tx.ExecContext(ctx, `UPDATE activity_audiences SET mode=$2,user_ids=$3,updated_at=NOW() WHERE kind=$1`, kind, in.Mode, pq.Array(ids)); err != nil {
		return err
	}
	if err = consumptionAudit(ctx, tx, actor, "audience:"+kind, 0, map[string]any{"before": old, "after": in}); err != nil {
		return err
	}
	return tx.Commit()
}
