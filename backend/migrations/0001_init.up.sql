-- Fatia inicial de mensageria (1 tenant). O multi-tenant por schema entra depois.

CREATE TABLE IF NOT EXISTS conversations (
    id                BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    contact_phone     TEXT NOT NULL UNIQUE,
    contact_name      TEXT NOT NULL DEFAULT '',
    unread_count      INT  NOT NULL DEFAULT 0,
    last_msg_at       TIMESTAMPTZ NOT NULL DEFAULT now(),
    window_expires_at TIMESTAMPTZ,
    created_at        TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS messages (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    wa_message_id   TEXT NOT NULL UNIQUE,            -- idempotência de webhook
    direction       TEXT NOT NULL CHECK (direction IN ('in','out')),
    body            TEXT NOT NULL,
    status          TEXT NOT NULL DEFAULT 'pending', -- sent|delivered|read|failed|received
    error           TEXT,
    created_at      TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at      TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE INDEX IF NOT EXISTS idx_messages_conversation ON messages(conversation_id, created_at DESC);
CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(created_at DESC);
