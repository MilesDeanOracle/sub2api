-- 鹈鹕运营检测的账号级开关。detect_paused = true 时定时调度器不再为该账号
-- 生成检测 run（手动“立即执行”不受影响）。账号删除时随 accounts 级联清理。

CREATE TABLE IF NOT EXISTS operations_pelican_account_flags (
    account_id    BIGINT PRIMARY KEY REFERENCES accounts(id) ON DELETE CASCADE,
    detect_paused BOOLEAN NOT NULL DEFAULT false,
    updated_at    TIMESTAMPTZ NOT NULL DEFAULT NOW()
);
