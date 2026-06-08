-- Etiquetas (labels) para organizar conversas.
CREATE TABLE IF NOT EXISTS labels (
    id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome      TEXT NOT NULL UNIQUE,
    cor       TEXT NOT NULL DEFAULT 'oklch(0.62 0.155 256)',
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE IF NOT EXISTS conversation_labels (
    conversation_id BIGINT NOT NULL REFERENCES conversations(id) ON DELETE CASCADE,
    label_id        BIGINT NOT NULL REFERENCES labels(id) ON DELETE CASCADE,
    PRIMARY KEY (conversation_id, label_id)
);

INSERT INTO labels (nome, cor) VALUES
    ('Urgente',        'oklch(0.64 0.20 25)'),
    ('Aguardando doc', 'oklch(0.78 0.14 78)'),
    ('Resolvido',      'oklch(0.76 0.165 152)'),
    ('Reclamação',     'oklch(0.60 0.15 300)')
ON CONFLICT (nome) DO NOTHING;
