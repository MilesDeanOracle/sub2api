package repository

import (
	"context"
	"database/sql"
	"database/sql/driver"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"testing"
	"time"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/stretchr/testify/require"
)

func TestFeedbackEnrichmentPersistsAccountAndSevenDayModelGroupUsage(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	r := &feedbackRepository{db: db}
	end := time.Date(2026, 11, 10, 12, 0, 0, 0, time.UTC)
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id,user_id,created_at.*FOR UPDATE SKIP LOCKED`).WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "created_at"}).AddRow(1, 7, end))
	mock.ExpectQuery(`SELECT COALESCE\(username,''\),email,total_recharged`).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"username", "email", "total_recharged"}).AddRow("tester", "test@example.com", 42.5))
	mock.ExpectQuery(`SELECT COALESCE\(NULLIF\(u.requested_model,''\),u.model\).*SUM\(u.input_tokens::bigint.*GROUP BY`).WithArgs(int64(7), end.Add(-7*24*time.Hour), end).WillReturnRows(sqlmock.NewRows([]string{"model", "group_id", "group_name", "tokens"}).AddRow("gpt-model", 3, "standard", int64(2000000)).AddRow("other", nil, "", int64(1200000000)))
	mock.ExpectExec(`UPDATE feedback_tickets SET snapshot=\$2,enrichment_status='ready'`).WithArgs(int64(1), feedbackSnapshotMatcher{t: t, end: end}).WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectCommit()
	processed, err := r.EnrichNext(context.Background())
	require.NoError(t, err)
	require.True(t, processed)
	require.NoError(t, mock.ExpectationsWereMet())
}

type feedbackSnapshotMatcher struct {
	t   *testing.T
	end time.Time
}

func (m feedbackSnapshotMatcher) Match(value driver.Value) bool {
	data, ok := value.(string)
	if !ok {
		return false
	}
	var s feedbackSnapshot
	if json.Unmarshal([]byte(data), &s) != nil {
		return false
	}
	require.Equal(m.t, "tester", s.Username)
	require.Equal(m.t, "test@example.com", s.Email)
	require.Equal(m.t, 42.5, s.TotalRecharged)
	require.EqualValues(m.t, 1202000000, s.TotalTokens)
	require.Len(m.t, s.Usage, 2)
	require.EqualValues(m.t, 3, *s.Usage[0].GroupID)
	require.Nil(m.t, s.Usage[1].GroupID)
	require.True(m.t, m.end.Equal(s.WindowEnd))
	require.True(m.t, m.end.Add(-7*24*time.Hour).Equal(s.WindowStart))
	return true
}

func TestFeedbackEnrichmentFailureRetainsTicketAndSchedulesRetry(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectQuery(`SELECT id,user_id,created_at`).WillReturnRows(sqlmock.NewRows([]string{"id", "user_id", "created_at"}).AddRow(1, 7, time.Now()))
	mock.ExpectQuery(`SELECT COALESCE\(username`).WillReturnError(errors.New("query failed"))
	mock.ExpectRollback()
	mock.ExpectExec(`UPDATE feedback_tickets SET enrichment_attempts=enrichment_attempts\+1,enrichment_status=CASE WHEN enrichment_attempts>=7 THEN 'failed' ELSE 'pending' END,next_attempt_at=NOW\(\)\+INTERVAL '1 minute'`).WithArgs(int64(1)).WillReturnResult(sqlmock.NewResult(0, 1))
	processed, err := (&feedbackRepository{db: db}).EnrichNext(context.Background())
	require.Error(t, err)
	require.True(t, processed)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFeedbackGetEnforcesOwnerBeforeLoadingReplies(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	// The caller is user 9 even if the requested ticket belongs to somebody else.
	mock.ExpectQuery(`SELECT .*NULL::jsonb.*WHERE f.id=\$1 AND \(\$2::bigint=0 OR f.user_id=\$2\)`).WithArgs(int64(1), int64(9)).WillReturnError(sql.ErrNoRows)
	_, err = (&feedbackRepository{db: db}).Get(context.Background(), 1, 9)
	require.ErrorIs(t, err, service.ErrFeedbackNotFound)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFeedbackReplyAndStatusAreAtomic(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	mock.ExpectBegin()
	mock.ExpectExec(regexp.QuoteMeta(`UPDATE feedback_tickets SET status=$2,updated_at=NOW() WHERE id=$1`)).WithArgs(int64(1), "resolved").WillReturnResult(sqlmock.NewResult(0, 1))
	mock.ExpectExec(`INSERT INTO feedback_replies`).WithArgs(int64(1), int64(2), "answer").WillReturnError(errors.New("insert failed"))
	mock.ExpectRollback()
	require.Error(t, (&feedbackRepository{db: db}).Reply(context.Background(), 1, 2, "answer", true))
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFeedbackReadReceiptIsOwnerScopedAndLimitedToDisplayedReply(t *testing.T) {
	for _, owned := range []bool{true, false} {
		t.Run(fmt.Sprint(owned), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			mock.ExpectQuery(`WITH owned AS \( SELECT f.id FROM feedback_tickets f WHERE f.id=\$1 AND f.user_id=\$2 AND EXISTS \(SELECT 1 FROM feedback_replies r WHERE r.ticket_id=f.id AND r.id=\$3 AND r.author_role='admin'\) \), marked AS \( UPDATE feedback_replies SET read_at=NOW\(\) WHERE ticket_id IN \(SELECT id FROM owned\) AND id<=\$3 AND author_role='admin' AND read_at IS NULL RETURNING id \) SELECT EXISTS \(SELECT 1 FROM owned\)`).WithArgs(int64(23), int64(7), int64(42)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(owned))
			err = (&feedbackRepository{db: db}).MarkRead(context.Background(), 23, 7, 42)
			if owned {
				require.NoError(t, err)
			} else {
				require.ErrorIs(t, err, service.ErrFeedbackNotFound)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestFeedbackSummaryCountsTicketsForCurrentRole(t *testing.T) {
	db, mock, err := sqlmock.New()
	require.NoError(t, err)
	defer db.Close()
	r := &feedbackRepository{db: db}
	mock.ExpectQuery(`SELECT COUNT\(\*\) FROM feedback_tickets WHERE status='open'`).WillReturnRows(sqlmock.NewRows([]string{"count"}).AddRow(4))
	summary, err := r.Summary(context.Background(), 0)
	require.NoError(t, err)
	require.EqualValues(t, 4, summary.PendingCount)
	require.Zero(t, summary.UnreadCount)
	mock.ExpectQuery(`SELECT COUNT\(DISTINCT f.id\),COALESCE\(MAX\(r.id\),0\).*WHERE f.user_id=\$1 AND r.author_role='admin' AND r.read_at IS NULL`).WithArgs(int64(7)).WillReturnRows(sqlmock.NewRows([]string{"count", "latest_reply_id"}).AddRow(2, 42))
	summary, err = r.Summary(context.Background(), 7)
	require.NoError(t, err)
	require.EqualValues(t, 2, summary.UnreadCount)
	require.EqualValues(t, 42, summary.LatestUnreadReplyID)
	require.Zero(t, summary.PendingCount)
	require.NoError(t, mock.ExpectationsWereMet())
}

func TestFeedbackUserReplyEnforcesOwnershipAndPriorAdminReply(t *testing.T) {
	for _, test := range []struct {
		name            string
		exists, replied bool
		want            error
	}{
		{"another owner", false, false, service.ErrFeedbackNotFound},
		{"awaiting first admin reply", true, false, service.ErrFeedbackAwaitingAdmin},
	} {
		t.Run(test.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			mock.ExpectBegin()
			query := mock.ExpectQuery(`SELECT EXISTS .*r.author_role='admin'.*f.id=\$1 AND f.user_id=\$2 FOR UPDATE OF f`).WithArgs(int64(23), int64(7))
			if test.exists {
				query.WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(test.replied))
			} else {
				query.WillReturnError(sql.ErrNoRows)
			}
			mock.ExpectRollback()
			require.ErrorIs(t, (&feedbackRepository{db: db}).ReplyMine(context.Background(), 23, 7, "追问"), test.want)
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}

func TestFeedbackUserReplyReopensTicketAtomically(t *testing.T) {
	for _, failInsert := range []bool{false, true} {
		t.Run(fmt.Sprint(failInsert), func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			mock.ExpectBegin()
			mock.ExpectQuery(`SELECT EXISTS .*FOR UPDATE OF f`).WithArgs(int64(23), int64(7)).WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(true))
			mock.ExpectExec(`UPDATE feedback_tickets SET status='open',updated_at=NOW\(\) WHERE id=\$1`).WithArgs(int64(23)).WillReturnResult(sqlmock.NewResult(0, 1))
			insert := mock.ExpectExec(`INSERT INTO feedback_replies \(ticket_id,user_id,author_role,content\) VALUES \(\$1,\$2,'user',\$3\)`).WithArgs(int64(23), int64(7), "追问")
			if failInsert {
				insert.WillReturnError(errors.New("insert failed"))
				mock.ExpectRollback()
			} else {
				insert.WillReturnResult(sqlmock.NewResult(5, 1))
				mock.ExpectCommit()
			}
			err = (&feedbackRepository{db: db}).ReplyMine(context.Background(), 23, 7, "追问")
			if failInsert {
				require.Error(t, err)
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
