"use client";

import { ReactNode, useState } from "react";
import { Topbar } from "@/components/shell/Topbar";
import { Logo } from "@/components/shell/Logo";
import {
  BarChart,
  Button,
  ConfirmModal,
  Donut,
  Field,
  Input,
  Segmented,
  Spark,
  StatusBadge,
  Tabs,
  WaBubble,
  useToast,
} from "@/components/ui";
import { VOLUME_HORA } from "@/lib/data";

function Block({ title, desc, children }: { title: string; desc?: string; children: ReactNode }) {
  return (
    <section className="ds-section">
      <h2>{title}</h2>
      {desc && <div className="ds-desc">{desc}</div>}
      <div className="card card-pad">{children}</div>
    </section>
  );
}

const COLOR_TOKENS: [string, string][] = [
  ["--brand", "Marca / primária"],
  ["--wa", "WhatsApp"],
  ["--st-delivered", "Entregue"],
  ["--st-read", "Lido"],
  ["--st-pending", "Pendente"],
  ["--st-failed", "Falhou"],
  ["--st-scheduled", "Agendado"],
];
const SURFACES = ["--bg-base", "--bg-sunken", "--bg-surface", "--bg-surface-2", "--bg-elevated", "--bg-hover"];
const TYPE_SPECS: [string, number, number][] = [
  ["Display 30 / 800", 32, 800],
  ["Título 19 / 700", 19, 700],
  ["Subtítulo 15 / 600", 15, 600],
  ["Corpo 14 / 400", 14, 400],
  ["Legenda 12 / 500", 12, 500],
];

export default function DSPage() {
  const toast = useToast();
  const [demoModal, setDemoModal] = useState(false);

  return (
    <>
      <Topbar title="Design System" sub="Fundação visual e componentes do ZapGov" />
      <div className="content">
        <div className="content-pad">
          <div className="ds-page">
            <Block title="Marca" desc='Wordmark com símbolo "raio" — agilidade institucional. Funciona em fundo claro e escuro.'>
              <div className="row center" style={{ gap: 40, flexWrap: "wrap" }}>
                <Logo size={44} />
                <div style={{ padding: "16px 22px", background: "var(--brand)", borderRadius: 12 }}>
                  <Logo size={34} />
                </div>
                <div className="row center" style={{ gap: 14 }}>
                  {[22, 30, 40].map((s) => (
                    <div key={s} style={{ width: s, height: s, borderRadius: s * 0.28, background: "var(--brand)", display: "grid", placeItems: "center" }}>
                      <svg width={s * 0.6} height={s * 0.6} viewBox="0 0 24 24">
                        <path d="M13 2 4 13h7l-1 9 10-12h-7l1-8z" fill="white" />
                      </svg>
                    </div>
                  ))}
                </div>
              </div>
            </Block>

            <Block title="Cores — semânticas" desc="Status consistentes em toda a plataforma. Tokens OKLCH adaptados a dark e light.">
              <div className="swatch-grid">
                {COLOR_TOKENS.map(([v, l]) => (
                  <div key={v}>
                    <div className="swatch" style={{ background: `var(${v})` }} />
                    <div style={{ marginTop: 7 }}>
                      <div className="strong" style={{ fontSize: 12.5 }}>
                        {l}
                      </div>
                      <div className="mono faint" style={{ fontSize: 10.5 }}>
                        {v}
                      </div>
                    </div>
                  </div>
                ))}
              </div>
            </Block>

            <Block title="Superfícies" desc="Camadas de elevação do navy base.">
              <div className="row" style={{ gap: 10, flexWrap: "wrap" }}>
                {SURFACES.map((v) => (
                  <div key={v} className="col" style={{ gap: 6 }}>
                    <div style={{ width: 96, height: 56, borderRadius: 9, background: `var(${v})`, border: "1px solid var(--border)" }} />
                    <div className="mono faint" style={{ fontSize: 10 }}>
                      {v.replace("--bg-", "")}
                    </div>
                  </div>
                ))}
              </div>
            </Block>

            <Block title="Tipografia" desc="Plus Jakarta Sans para interface · JetBrains Mono para dados, IDs e métricas.">
              <div className="col" style={{ gap: 0 }}>
                {TYPE_SPECS.map((t, i) => (
                  <div key={i} className="spec-row">
                    <span style={{ fontSize: t[1], fontWeight: t[2], width: 380 }}>Comunicação oficial ágil</span>
                    <span className="mono faint" style={{ fontSize: 11.5 }}>
                      {t[0]}
                    </span>
                  </div>
                ))}
                <div className="spec-row" style={{ borderBottom: "none" }}>
                  <span className="mono" style={{ fontSize: 22, fontWeight: 700, width: 380 }}>
                    8.472 · 97,8% · R$ 4.990
                  </span>
                  <span className="mono faint" style={{ fontSize: 11.5 }}>
                    JetBrains Mono — métricas
                  </span>
                </div>
              </div>
            </Block>

            <Block title="Botões" desc="Variantes, tamanhos e estados.">
              <div className="col" style={{ gap: 14 }}>
                <div className="row wrap" style={{ gap: 10 }}>
                  <Button variant="primary" icon="send">
                    Primário
                  </Button>
                  <Button variant="wa" icon="message">
                    WhatsApp
                  </Button>
                  <Button variant="secondary" icon="download">
                    Secundário
                  </Button>
                  <Button variant="ghost" icon="filter">
                    Ghost
                  </Button>
                  <Button variant="danger" icon="trash">
                    Perigo
                  </Button>
                  <Button variant="primary" aria-disabled>
                    Desabilitado
                  </Button>
                </div>
                <div className="row center wrap" style={{ gap: 10 }}>
                  <Button variant="primary" size="lg">
                    Grande
                  </Button>
                  <Button variant="primary">Padrão</Button>
                  <Button variant="primary" size="sm">
                    Pequeno
                  </Button>
                  <Button variant="secondary" icon="plus" />
                  <Button variant="ghost" icon="more" />
                </div>
              </div>
            </Block>

            <Block title="Status e badges" desc="Estados de mensagem e marcadores.">
              <div className="row wrap" style={{ gap: 8 }}>
                {["enviado", "entregue", "lido", "pendente", "falhou", "agendado", "aprovado", "rejeitado"].map((s) => (
                  <StatusBadge key={s} status={s} />
                ))}
              </div>
            </Block>

            <Block title="Campos de formulário" desc="Inputs, selects, toggles e seleção.">
              <div style={{ display: "grid", gridTemplateColumns: "1fr 1fr", gap: 16, maxWidth: 560 }}>
                <Field label="Campo com ícone">
                  <Input icon="search" placeholder="Buscar…" />
                </Field>
                <Field label="Seleção">
                  <select className="select">
                    <option>Opção A</option>
                    <option>Opção B</option>
                  </select>
                </Field>
                <Field label="Com erro" error="Campo obrigatório">
                  <Input error defaultValue="" />
                </Field>
                <div className="col" style={{ gap: 12, justifyContent: "center" }}>
                  <label className="row center" style={{ gap: 9, fontSize: 13, cursor: "pointer" }}>
                    <input type="checkbox" className="checkbox" defaultChecked /> Checkbox marcado
                  </label>
                  <label className="row center" style={{ gap: 9, fontSize: 13, cursor: "pointer" }}>
                    <input type="checkbox" className="toggle" defaultChecked /> Toggle ligado
                  </label>
                </div>
              </div>
            </Block>

            <Block title="Navegação" desc="Tabs e controle segmentado.">
              <div className="col" style={{ gap: 18 }}>
                <Tabs
                  value="a"
                  onChange={() => {}}
                  tabs={[
                    { id: "a", label: "Aprovados", count: 12 },
                    { id: "b", label: "Pendentes", count: 3 },
                    { id: "c", label: "Rejeitados" },
                  ]}
                />
                <Segmented
                  value="x"
                  onChange={() => {}}
                  options={[
                    { value: "x", label: "Por hora" },
                    { value: "y", label: "7 dias" },
                    { value: "z", label: "30 dias" },
                  ]}
                />
              </div>
            </Block>

            <Block title="Balões de WhatsApp" desc="Componente de pré-visualização com variáveis, status e botões de ação.">
              <div className="wa-canvas" style={{ maxWidth: 420 }}>
                <WaBubble text="Olá {{1}}! Sua consulta está confirmada para {{2}}." dir="out" highlight status="lido" time="09:14" buttons={["Confirmar presença", "Reagendar"]} />
                <WaBubble text="Perfeito, estarei lá. Obrigado!" dir="in" time="09:20" />
              </div>
            </Block>

            <Block title="Visualização de dados" desc="Gráficos leves em SVG: barras, donut e sparkline.">
              <div className="row" style={{ gap: 40, flexWrap: "wrap", alignItems: "center" }}>
                <div style={{ width: 280 }}>
                  <BarChart height={110} data={VOLUME_HORA.slice(0, 6)} secondaryKey="lidas" />
                </div>
                <Donut value={84.3} size={100} color="var(--st-read)" label="leitura" />
                <div className="col" style={{ gap: 4 }}>
                  <Spark data={[4, 5, 4, 6, 7, 6, 8, 7, 9]} width={140} height={44} />
                  <span className="faint" style={{ fontSize: 11 }}>
                    sparkline de volume
                  </span>
                </div>
              </div>
            </Block>

            <Block title="Overlays e feedback" desc="Modais, toasts e estados vazios.">
              <div className="row wrap" style={{ gap: 10 }}>
                <Button variant="secondary" onClick={() => setDemoModal(true)}>
                  Abrir modal
                </Button>
                <Button variant="secondary" onClick={() => toast({ type: "success", title: "Operação concluída", message: "Tudo certo por aqui." })}>
                  Toast sucesso
                </Button>
                <Button variant="secondary" onClick={() => toast({ type: "error", title: "Falha no envio", message: "3 números inválidos." })}>
                  Toast erro
                </Button>
                <Button variant="secondary" onClick={() => toast({ type: "wa", title: "Nova resposta", message: "João respondeu sua mensagem." })}>
                  Toast WhatsApp
                </Button>
              </div>
            </Block>
          </div>
        </div>
      </div>

      <ConfirmModal
        open={demoModal}
        onClose={() => setDemoModal(false)}
        onConfirm={() => setDemoModal(false)}
        title="Exemplo de confirmação"
        message="Este é o padrão de modal de confirmação usado em toda a plataforma."
        confirmLabel="Entendi"
      />
    </>
  );
}
