-- 渠道监控 V1 探测异常记录（channel-monitor-v1-probe-retry 方案）。
-- 只记录失败策略（首次 error/failed 且重试后仍未恢复）落空的周期；
-- 降级周期、重试后恢复的周期不写入。channel_monitor_histories 保持不动。
-- 注意：TRIGGER 是 PG 保留字，列名必须带双引号（API 字段名仍为 trigger）。
CREATE TABLE IF NOT EXISTS channel_monitor_probe_anomalies (
    id                    BIGSERIAL PRIMARY KEY,
    monitor_id            BIGINT       NOT NULL REFERENCES channel_monitors(id) ON DELETE CASCADE,
    model                 VARCHAR(200) NOT NULL,
    "trigger"             VARCHAR(20)  NOT NULL,            -- scheduled / manual
    trigger_status        VARCHAR(20)  NOT NULL,            -- 首次失败状态：error / failed
    resolution            VARCHAR(20)  NOT NULL,            -- substituted / real_written
    reason                VARCHAR(50)  NOT NULL DEFAULT '', -- real_written 时：no_reference / too_many_consecutive
    consecutive_seq       INT          NOT NULL,            -- 连续异常周期序号，从 1 开始
    attempt_count         SMALLINT     NOT NULL,
    attempts              JSONB        NOT NULL,            -- 每次尝试明细
    reference_history_id  BIGINT       NULL,                -- 被复制的参考行
    reference_checked_at  TIMESTAMPTZ  NULL,
    written_history_id    BIGINT       NOT NULL,            -- 本周期最终写入的历史行
    cycle_started_at      TIMESTAMPTZ  NOT NULL,
    cycle_finished_at     TIMESTAMPTZ  NOT NULL,
    created_at            TIMESTAMPTZ  NOT NULL DEFAULT NOW()
);

CREATE INDEX IF NOT EXISTS idx_cm_probe_anomalies_monitor_model_created
    ON channel_monitor_probe_anomalies (monitor_id, model, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_cm_probe_anomalies_created
    ON channel_monitor_probe_anomalies (created_at);
CREATE INDEX IF NOT EXISTS idx_cm_probe_anomalies_written_history
    ON channel_monitor_probe_anomalies (written_history_id);
