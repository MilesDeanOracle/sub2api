package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestRechargeQuickButtonsMigration(t *testing.T) {
	content, err := FS.ReadFile("250_recharge_quick_buttons.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS recharge_quick_buttons")
	require.Contains(t, sql, "label VARCHAR(50) NOT NULL DEFAULT ''")
	require.Contains(t, sql, "url TEXT NOT NULL")
	require.Contains(t, sql, "open_method VARCHAR(20) NOT NULL DEFAULT 'new_tab'")
	require.Contains(t, sql, "enabled BOOLEAN NOT NULL DEFAULT TRUE")
	require.Contains(t, sql, "sort_order INT NOT NULL DEFAULT 0")
	require.Contains(t, sql, "CREATE INDEX IF NOT EXISTS idx_recharge_quick_buttons_enabled ON recharge_quick_buttons(enabled)")
}
