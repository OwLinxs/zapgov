-- Usuários e papéis (auth + RBAC). Seed do admin é feito em Go (bcrypt no boot).
CREATE TABLE IF NOT EXISTS users (
    id         BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome       TEXT NOT NULL,
    email      TEXT NOT NULL UNIQUE,
    senha_hash TEXT NOT NULL,
    papel      TEXT NOT NULL DEFAULT 'atendente' CHECK (papel IN ('admin','supervisor','atendente')),
    ativo      BOOLEAN NOT NULL DEFAULT true,
    criado_em  TIMESTAMPTZ NOT NULL DEFAULT now()
);
