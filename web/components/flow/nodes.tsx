"use client";

import { Handle, Position, NodeProps } from "reactflow";
import { Icon } from "@/components/ui";
import { IconName } from "@/lib/icons";

// Metadados de cada tipo de bloco: rótulo, ícone, cor e dado padrão ao criar.
export const NODE_META: Record<
  string,
  { label: string; icon: IconName; color: string; defaultData: Record<string, unknown> }
> = {
  start: { label: "Início", icon: "play", color: "var(--wa)", defaultData: {} },
  message: { label: "Mensagem", icon: "message", color: "var(--brand-text)", defaultData: { text: "Sua mensagem…" } },
  menu: {
    label: "Menu",
    icon: "list",
    color: "var(--st-scheduled)",
    defaultData: { text: "Escolha uma opção:", options: [{ key: "1", label: "Opção 1" }] },
  },
  input: { label: "Pergunta", icon: "edit", color: "var(--st-pending)", defaultData: { prompt: "Qual seu nome?", var: "nome" } },
  condition: { label: "Condição", icon: "filter", color: "var(--st-delivered)", defaultData: { var: "nome", op: "eq", value: "" } },
  action: { label: "Ação", icon: "zap", color: "var(--brand-text)", defaultData: { kind: "assign_department", department: "" } },
  handoff: { label: "Atendente", icon: "user", color: "var(--st-read)", defaultData: { department: "" } },
  end: { label: "Fim", icon: "ban", color: "var(--st-failed)", defaultData: {} },
};

const dotStyle = { width: 9, height: 9, background: "var(--border-strong)", border: "2px solid var(--bg-surface)" };

function Shell({ type, selected, children }: { type: string; selected: boolean; children: React.ReactNode }) {
  const m = NODE_META[type];
  return (
    <div
      className="card"
      style={{
        minWidth: 180,
        maxWidth: 240,
        boxShadow: selected ? "0 0 0 2px var(--brand)" : "var(--shadow-sm)",
        borderColor: selected ? "var(--brand)" : "var(--border)",
      }}
    >
      <div className="row center" style={{ gap: 8, padding: "8px 11px", borderBottom: "1px solid var(--border-subtle)" }}>
        <span className="kpi-ico" style={{ width: 22, height: 22, background: "color-mix(in oklch, " + m.color + " 18%, transparent)", color: m.color }}>
          <Icon name={m.icon} size={13} />
        </span>
        <span className="strong" style={{ fontSize: 12.5 }}>
          {m.label}
        </span>
      </div>
      <div style={{ padding: "9px 11px", fontSize: 12, color: "var(--text-secondary)" }}>{children}</div>
    </div>
  );
}

function trunc(s: unknown, n = 60) {
  const t = String(s ?? "");
  return t.length > n ? t.slice(0, n) + "…" : t || "—";
}

// Bloco com 1 saída padrão (message, input, action, start).
function SingleOut({ type, data, selected }: NodeProps & { type: string }) {
  return (
    <>
      {type !== "start" && <Handle type="target" position={Position.Top} style={dotStyle} />}
      <Shell type={type} selected={selected}>
        {type === "message" && trunc((data as any).text)}
        {type === "input" && (
          <>
            {trunc((data as any).prompt)}
            <div className="mono faint" style={{ fontSize: 10.5, marginTop: 3 }}>
              → {"{{"}{(data as any).var || "var"}{"}}"}
            </div>
          </>
        )}
        {type === "action" && ((data as any).kind === "abrir_chamado"
          ? <>Encaminhar ao atendimento</>
          : <>Atribuir setor: <b>{(data as any).department || "—"}</b></>)}
        {type === "start" && <span className="muted">Entrada do fluxo</span>}
      </Shell>
      <Handle type="source" position={Position.Bottom} style={dotStyle} />
    </>
  );
}

// Menu: uma saída por opção (sourceHandle = chave). No modo CAPTURA (data.var
// preenchido) vira saída única — a escolha é salva na variável e segue reto.
function MenuNode({ data, selected }: NodeProps) {
  const opts: { key: string; label: string }[] = (data as any).options || [];
  const captura = !!(data as any).var;
  return (
    <>
      <Handle type="target" position={Position.Top} style={dotStyle} />
      <Shell type="menu" selected={selected}>
        <div style={{ marginBottom: 6 }}>{trunc((data as any).text, 50)}</div>
        {captura && (
          <div className="mono faint" style={{ fontSize: 10, marginBottom: 4 }}>
            → salva em {"{{"}{(data as any).var}{"}}"}
          </div>
        )}
        <div className="col" style={{ gap: 4 }}>
          {opts.map((o) => (
            <div key={o.key} className="row center between" style={{ gap: 8, position: "relative", padding: "3px 0" }}>
              <span style={{ fontSize: 11.5 }}>
                <span className="mono strong">{o.key}</span> · {o.label}
              </span>
              {!captura && (
                <Handle
                  type="source"
                  position={Position.Right}
                  id={o.key}
                  style={{ ...dotStyle, position: "relative", right: -4, top: 0, transform: "none", background: "var(--brand)" }}
                />
              )}
            </div>
          ))}
        </div>
      </Shell>
      {captura && <Handle type="source" position={Position.Bottom} style={dotStyle} />}
    </>
  );
}

// Condição: duas saídas (true / false).
function ConditionNode({ data, selected }: NodeProps) {
  return (
    <>
      <Handle type="target" position={Position.Top} style={dotStyle} />
      <Shell type="condition" selected={selected}>
        <span className="mono" style={{ fontSize: 11.5 }}>
          {"{{"}{(data as any).var}{"}}"} {(data as any).op === "contains" ? "contém" : "="} &quot;{(data as any).value}&quot;
        </span>
        <div className="row between" style={{ marginTop: 6, fontSize: 10.5 }}>
          <span style={{ color: "var(--st-read)" }}>verdadeiro</span>
          <span style={{ color: "var(--st-failed)" }}>falso</span>
        </div>
      </Shell>
      <Handle type="source" position={Position.Bottom} id="true" style={{ ...dotStyle, left: "30%", background: "var(--st-read)" }} />
      <Handle type="source" position={Position.Bottom} id="false" style={{ ...dotStyle, left: "70%", background: "var(--st-failed)" }} />
    </>
  );
}

// Terminais (handoff, end): sem saída.
function Terminal({ type, data, selected }: NodeProps & { type: string }) {
  return (
    <>
      <Handle type="target" position={Position.Top} style={dotStyle} />
      <Shell type={type} selected={selected}>
        {type === "handoff" ? (
          <>Passa para atendente{(data as any).department ? <> · <b>{(data as any).department}</b></> : null}</>
        ) : (
          <span className="muted">Encerra a conversa</span>
        )}
      </Shell>
    </>
  );
}

export const nodeTypes = {
  start: (p: NodeProps) => <SingleOut {...p} type="start" />,
  message: (p: NodeProps) => <SingleOut {...p} type="message" />,
  input: (p: NodeProps) => <SingleOut {...p} type="input" />,
  action: (p: NodeProps) => <SingleOut {...p} type="action" />,
  menu: MenuNode,
  condition: ConditionNode,
  handoff: (p: NodeProps) => <Terminal {...p} type="handoff" />,
  end: (p: NodeProps) => <Terminal {...p} type="end" />,
};
