-- Flow builder estilo n8n: grafo de blocos + estado de execução por conversa.

CREATE TABLE IF NOT EXISTS flows (
    id        BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    nome      TEXT NOT NULL,
    graph     JSONB NOT NULL DEFAULT '{"nodes":[],"edges":[]}',
    ativo     BOOLEAN NOT NULL DEFAULT false,
    criado_em TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- "Program counter" por telefone: onde a conversa está no grafo + variáveis.
CREATE TABLE IF NOT EXISTS conversation_flow_state (
    phone        TEXT PRIMARY KEY,
    current_node TEXT NOT NULL,
    vars         JSONB NOT NULL DEFAULT '{}',
    updated_at   TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Setor atribuído à conversa (usado pelo bloco handoff).
ALTER TABLE conversations ADD COLUMN IF NOT EXISTS setor TEXT NOT NULL DEFAULT '';

-- Fluxo de exemplo, ativo, para já dar o que testar.
INSERT INTO flows (nome, ativo, graph) VALUES (
  'Atendimento padrão', true,
  '{
    "nodes":[
      {"id":"start","type":"start","position":{"x":300,"y":0},"data":{}},
      {"id":"menu","type":"menu","position":{"x":250,"y":110},"data":{"text":"Olá! 👋 Bem-vindo ao atendimento. Escolha uma opção:","options":[{"key":"1","label":"Saúde"},{"key":"2","label":"Tributos"},{"key":"3","label":"Outro assunto"}]}},
      {"id":"m_saude","type":"message","position":{"x":0,"y":280},"data":{"text":"Você escolheu *Saúde*. Descreva sua necessidade."}},
      {"id":"h_saude","type":"handoff","position":{"x":0,"y":400},"data":{"department":"Saúde"}},
      {"id":"m_trib","type":"message","position":{"x":300,"y":280},"data":{"text":"Você escolheu *Tributos*."}},
      {"id":"h_trib","type":"handoff","position":{"x":300,"y":400},"data":{"department":"Tributos"}},
      {"id":"m_outro","type":"message","position":{"x":600,"y":280},"data":{"text":"Descreva seu assunto que vamos encaminhar."}},
      {"id":"end","type":"end","position":{"x":600,"y":400},"data":{}}
    ],
    "edges":[
      {"source":"start","target":"menu"},
      {"source":"menu","sourceHandle":"1","target":"m_saude"},
      {"source":"menu","sourceHandle":"2","target":"m_trib"},
      {"source":"menu","sourceHandle":"3","target":"m_outro"},
      {"source":"m_saude","target":"h_saude"},
      {"source":"m_trib","target":"h_trib"},
      {"source":"m_outro","target":"end"}
    ]
  }'
) ON CONFLICT DO NOTHING;
