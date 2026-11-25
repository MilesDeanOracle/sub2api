package repository

import (
	"context"
	sqlmock "github.com/DATA-DOG/go-sqlmock"
	"github.com/stretchr/testify/require"
	"testing"
)

func TestActivityMaintenanceGuard(t *testing.T) {
	for _, hasUsers := range []bool{false, true} {
		t.Run(map[bool]string{false: "empty installation", true: "existing users"}[hasUsers], func(t *testing.T) {
			t.Setenv("ACTIVITY_MAINTENANCE_CONFIRMED", "")
			db, mock, err := sqlmock.New()
			require.NoError(t, err)
			defer db.Close()
			conn, err := db.Conn(context.Background())
			require.NoError(t, err)
			defer conn.Close()
			mock.ExpectQuery("SELECT EXISTS").WillReturnRows(sqlmock.NewRows([]string{"exists"}).AddRow(hasUsers))
			err = requireActivityMaintenance(context.Background(), conn)
			if hasUsers {
				require.ErrorContains(t, err, "drained maintenance window")
			} else {
				require.NoError(t, err)
			}
			require.NoError(t, mock.ExpectationsWereMet())
		})
	}
	t.Run("explicit maintenance acknowledgment", func(t *testing.T) {
		t.Setenv("ACTIVITY_MAINTENANCE_CONFIRMED", "1")
		require.NoError(t, requireActivityMaintenance(context.Background(), nil))
	})
}
