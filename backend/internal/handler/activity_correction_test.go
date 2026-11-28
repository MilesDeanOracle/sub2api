//go:build unit

package handler

import (
	"database/sql"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/DATA-DOG/go-sqlmock"
	"github.com/Wei-Shaw/sub2api/internal/server/middleware"
	"github.com/Wei-Shaw/sub2api/internal/service"
	"github.com/gin-gonic/gin"
	"github.com/stretchr/testify/require"
)

func TestActivityCorrectionSignedEventID(t *testing.T) {
	gin.SetMode(gin.TestMode)
	for _, tc := range []struct {
		name, body string
		id         int64
	}{
		{"transactional", `{"ledger_id":-12,"note":"duplicate bill"}`, -12},
		{"legacy", `{"ledger_id":12,"note":"duplicate bill"}`, 12},
		{"zero", `{"ledger_id":0,"note":"duplicate bill"}`, 0},
	} {
		t.Run(tc.name, func(t *testing.T) {
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			if tc.id != 0 {
				mock.ExpectBegin()
				mock.ExpectQuery("SELECT user_id, reversed_at FROM activity_usage_events").WithArgs(tc.id).WillReturnError(sql.ErrNoRows)
				mock.ExpectRollback()
			}
			w := httptest.NewRecorder()
			c, _ := gin.CreateTestContext(w)
			c.Request = httptest.NewRequest(http.MethodPost, "/activity/consumption/correct", strings.NewReader(tc.body))
			c.Request.Header.Set("Content-Type", "application/json")
			c.Set(string(middleware.ContextKeyUser), middleware.AuthSubject{UserID: 99})
			NewActivityHandler(service.NewActivityService(db, nil, nil)).CorrectConsumption(c)
			require.Equal(t, http.StatusBadRequest, w.Code)
			if tc.id == 0 {
				require.Contains(t, w.Body.String(), "Invalid settlement")
			} else {
				require.Contains(t, w.Body.String(), sql.ErrNoRows.Error())
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
}
