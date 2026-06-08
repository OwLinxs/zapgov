-- Departamentos (antes era lista fixa no front) + chatbot de autoatendimento.

CREATE TABLE IF NOT EXISTS departments (
    id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome      TEXT NOT NULL UNIQUE,
    cor       TEXT NOT NULL DEFAULT 'oklch(0.6 0.02 264)',
    ativo     BOOLEAN NOT NULL DEFAULT true,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Seed com os setores que o front já usava (mantém compatibilidade).
INSERT INTO departments (nome, cor) VALUES
    ('Saúde',              'oklch(0.6 0.14 152)'),
    ('Tributos',           'oklch(0.64 0.13 78)'),
    ('Educação',           'oklch(0.6 0.13 256)'),
    ('Trânsito',           'oklch(0.6 0.15 25)'),
    ('Assistência Social', 'oklch(0.6 0.14 300)'),
    ('Meio Ambiente',      'oklch(0.6 0.12 160)'),
    ('Geral',              'oklch(0.6 0.02 264)')
ON CONFLICT (nome) DO NOTHING;

-- Configuração única do bot (linha id=1).
CREATE TABLE IF NOT EXISTS bot_config (
    id       INT PRIMARY KEY DEFAULT 1,
    enabled  BOOLEAN NOT NULL DEFAULT false,
    welcome  TEXT NOT NULL DEFAULT 'Olá! 👋 Bem-vindo ao atendimento. Escolha uma opção:',
    fallback TEXT NOT NULL DEFAULT 'Não entendi. Digite o número de uma das opções.',
    CONSTRAINT bot_config_single CHECK (id = 1)
);

INSERT INTO bot_config (id) VALUES (1) ON CONFLICT (id) DO NOTHING;

-- Opções do menu do bot.
CREATE TABLE IF NOT EXISTS bot_options (
    id            BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    ord           INT NOT NULL DEFAULT 0,
    chave         TEXT NOT NULL,          -- o que o cidadão digita, ex: "1"
    label         TEXT NOT NULL,          -- texto da opção no menu
    resposta      TEXT NOT NULL,          -- resposta enviada ao escolher
    department_id BIGINT REFERENCES departments(id) ON DELETE SET NULL,
    ativo         BOOLEAN NOT NULL DEFAULT true
);

CREATE INDEX IF NOT EXISTS idx_bot_options_ord ON bot_options(ord);
