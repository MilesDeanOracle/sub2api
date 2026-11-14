-- Independent operations plans and durable account queue. No extra round/join tables.
CREATE TABLE operations_pelican_tasks (
 id BIGSERIAL PRIMARY KEY, name varchar(200) NOT NULL, enabled boolean NOT NULL DEFAULT false,
 interval_hours integer NOT NULL CHECK (interval_hours BETWEEN 1 AND 8760), account_configs jsonb NOT NULL,
 reasoning_effort varchar NOT NULL DEFAULT '', prompt text NOT NULL, storage_source varchar NOT NULL,
 config_version bigint NOT NULL DEFAULT 1, last_triggered_at timestamptz, next_run_at timestamptz,
 created_by bigint NOT NULL, updated_by bigint NOT NULL, created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(), deleted_at timestamptz
);
CREATE INDEX operationspelicantask_next_run_at ON operations_pelican_tasks(next_run_at) WHERE enabled AND deleted_at IS NULL;
CREATE TABLE operations_pelican_results (
 id BIGSERIAL PRIMARY KEY, task_id bigint NOT NULL REFERENCES operations_pelican_tasks(id), run_id varchar NOT NULL,
 account_id bigint NOT NULL, trigger_key varchar NOT NULL, status varchar NOT NULL CHECK(status IN ('queued','running','uploading','succeeded','failed','cancelled')),
 group_snapshots jsonb NOT NULL DEFAULT '[]', payload jsonb NOT NULL,
 pool_slot integer UNIQUE CHECK(pool_slot BETWEEN 1 AND 4), worker_token varchar NOT NULL DEFAULT '', heartbeat_at timestamptz, execution_deadline_at timestamptz,
 storage_binding_encrypted text NOT NULL DEFAULT '', object_key varchar NOT NULL DEFAULT '', version_id varchar NOT NULL DEFAULT '',
 queued_at timestamptz NOT NULL DEFAULT now(), expires_at timestamptz NOT NULL,
 cleanup_status varchar NOT NULL DEFAULT 'active', cleanup_attempts integer NOT NULL DEFAULT 0, cleanup_error text NOT NULL DEFAULT '', cleanup_next_at timestamptz,
 created_at timestamptz NOT NULL DEFAULT now(), updated_at timestamptz NOT NULL DEFAULT now(),
 UNIQUE(run_id,account_id), CHECK ((status IN ('running','uploading')) = (pool_slot IS NOT NULL))
);
CREATE INDEX operationspelicanresult_task_account_id ON operations_pelican_results(task_id,account_id,id DESC);
CREATE INDEX operationspelicanresult_trigger_key ON operations_pelican_results(task_id,trigger_key);
CREATE INDEX operationspelicanresult_queued ON operations_pelican_results(queued_at,id) WHERE status='queued';
CREATE INDEX operationspelicanresult_run_status ON operations_pelican_results(run_id,status);
CREATE INDEX operationspelicanresult_expiry ON operations_pelican_results(expires_at,cleanup_next_at);
CREATE INDEX operationspelicanresult_groups ON operations_pelican_results USING gin(group_snapshots);
