-- Fila de atendimento: status da conversa + atendente que assumiu.
-- '' = sem bot/normal · aguardando = na fila do setor · em_atendimento = alguém pegou · resolvido
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS status    TEXT NOT NULL DEFAULT '';
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS atendente TEXT NOT NULL DEFAULT '';
