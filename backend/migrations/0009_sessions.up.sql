-- Sessões de atendimento: cada ciclo (abre → resolvido) é uma sessão.
-- Permite histórico completo do contato E histórico separado por sessão/janela.

CREATE TABLE IF NOT EXISTS sessions (
    id              BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    setor           TEXT NOT NULL DEFAULT '',
    atendente       TEXT NOT NULL DEFAULT '',
    status          TEXT NOT NULL DEFAULT 'aberta', -- aberta | resolvida
    dados           JSONB NOT NULL DEFAULT '{}',
    iniciada_em     TIMESTAMPTZ NOT NULL DEFAULT now(),
    encerrada_em    TIMESTAMPTZ
);

ALTER TABLE messages ADD COLUMN IF NOT EXISTS session_id BIGINT REFERENCES sessions(id) ON DELETE SET NULL;

CREATE INDEX IF NOT EXISTS idx_sessions_conv ON sessions(conversation_id, iniciada_em DESC);
CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);
