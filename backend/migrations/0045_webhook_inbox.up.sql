CREATE TABLE webhook_inbox (
 id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
 tenant_id BIGINT NOT NULL REFERENCES tenants(id),
 digest TEXT NOT NULL,
 payload JSONB NOT NULL,
 attempts INT NOT NULL DEFAULT 0,
 next_attempt_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 processed_at TIMESTAMPTZ,
 last_error TEXT NOT NULL DEFAULT '',
 created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
 UNIQUE (tenant_id, digest)
);
CREATE INDEX idx_webhook_inbox_pending ON webhook_inbox(next_attempt_at, id) WHERE processed_at IS NULL;
