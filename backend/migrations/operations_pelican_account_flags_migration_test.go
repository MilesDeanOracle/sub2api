package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// TestOperationsPelicanAccountFlagsMigration 校验 257 号迁移建出账号级检测开关表：
// account_id 主键级联删除 + detect_paused 默认 false，供定时调度器过滤被暂停账号。
func TestOperationsPelicanAccountFlagsMigration(t *testing.T) {
	content, err := FS.ReadFile("257_operations_pelican_account_flags.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS operations_pelican_account_flags")
	require.Contains(t, sql, "account_id BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE")
	require.Contains(t, sql, "detect_paused BOOLEAN NOT NULL DEFAULT false")
}
