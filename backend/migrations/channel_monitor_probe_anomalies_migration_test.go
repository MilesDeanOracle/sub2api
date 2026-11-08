package migrations

import (
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

func TestChannelMonitorProbeAnomaliesMigration(t *testing.T) {
	content, err := FS.ReadFile("251_channel_monitor_probe_anomalies.sql")
	require.NoError(t, err)

	sql := strings.Join(strings.Fields(string(content)), " ")
	require.Contains(t, sql, "CREATE TABLE IF NOT EXISTS channel_monitor_probe_anomalies")
	require.Contains(t, sql, `monitor_id BIGINT NOT NULL REFERENCES channel_monitors(id) ON DELETE CASCADE`)
	// TRIGGER 是 PG 保留字：列定义必须带双引号，否则迁移执行会语法错误。
	require.Contains(t, sql, `"trigger" VARCHAR(20) NOT NULL`)
	require.Contains(t, sql, "trigger_status VARCHAR(20) NOT NULL")
	require.Contains(t, sql, "resolution VARCHAR(20) NOT NULL")
	require.Contains(t, sql, "reason VARCHAR(50) NOT NULL DEFAULT ''")
	require.Contains(t, sql, "consecutive_seq INT NOT NULL")
	require.Contains(t, sql, "attempt_count SMALLINT NOT NULL")
	require.Contains(t, sql, "attempts JSONB NOT NULL")
	require.Contains(t, sql, "reference_history_id BIGINT NULL")
	require.Contains(t, sql, "written_history_id BIGINT NOT NULL")
	require.Contains(t, sql, "cycle_started_at TIMESTAMPTZ NOT NULL")
	require.Contains(t, sql, "cycle_finished_at TIMESTAMPTZ NOT NULL")
	require.Contains(t, sql, "created_at TIMESTAMPTZ NOT NULL DEFAULT NOW()")
	require.Contains(t, sql, "CREATE INDEX IF NOT EXISTS idx_cm_probe_anomalies_monitor_model_created ON channel_monitor_probe_anomalies (monitor_id, model, created_at DESC)")
	require.Contains(t, sql, "CREATE INDEX IF NOT EXISTS idx_cm_probe_anomalies_created ON channel_monitor_probe_anomalies (created_at)")
	require.Contains(t, sql, "CREATE INDEX IF NOT EXISTS idx_cm_probe_anomalies_written_history ON channel_monitor_probe_anomalies (written_history_id)")
}
