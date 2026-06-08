-- LGPD/compliance: trilha de auditoria de todas as ações que alteram dados.
CREATE TABLE IF NOT EXISTS audit_log (
    id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    user_id   BIGINT,
    user_nome TEXT NOT NULL DEFAULT '',
    papel     TEXT NOT NULL DEFAULT '',
    metodo    TEXT NOT NULL,
    rota      TEXT NOT NULL,
    status    INT  NOT NULL,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX IF NOT EXISTS idx_audit_criado ON audit_log(criado_em DESC);
