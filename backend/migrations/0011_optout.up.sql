-- LGPD: opt-out. Cidadão pede para sair → não recebe mais mensagens.

-- Flag na conversa (para o inbox e bloqueio de resposta manual).
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS opt_out BOOLEAN NOT NULL DEFAULT false;

-- Lista canônica de telefones descadastrados (para bloquear disparos em massa).
CREATE TABLE IF NOT EXISTS opt_outs (
    phone     TEXT PRIMARY KEY,         -- formato canônico (dígitos, com 9 BR)
    motivo    TEXT NOT NULL DEFAULT '',
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);
