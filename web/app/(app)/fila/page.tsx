"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Topbar } from "@/components/shell/Topbar";
import { Button, Icon } from "@/components/ui";
import { listConversations, getRoteamento, type ApiConversation } from "@/lib/api";
import { SETOR_COR } from "@/lib/data";

// Painel de fila ao vivo (pensado pra TV/monitor da recepção): fila por setor
// com SLA visual, atendimentos em andamento por atendente, relógio. Atualiza
// sozinho a cada 10s; botão de tela cheia esconde o resto do painel.

function minutosDesde(iso: string | null): number {
  if (!iso) return 0;
  return Math.max(0, Math.floor((Date.now() - new Date(iso).getTime()) / 60000));
}

export default function FilaPage() {
  const fullRef = useRef<HTMLDivElement>(null);
  const [fila, setFila] = useState<ApiConversation[]>([]);
  const [atendendo, setAtendendo] = useState<ApiConversation[]>([]);
  const [slaMin, setSlaMin] = useState(15);
  const [agora, setAgora] = useState(new Date());
  const [erro, setErro] = useState(false);

  const load = useCallback(async () => {
    try {
      const [ag, at] = await Promise.all([
        listConversations({ status: "aguardando", limit: 300 }),
        listConversations({ status: "em_atendimento", limit: 300 }),
      ]);
      setFila(ag);
      setAtendendo(at);
      setErro(false);
    } catch {
      setErro(true);
    }
  }, []);

  useEffect(() => {
    load();
    getRoteamento().then((r) => r.sla_minutos > 0 && setSlaMin(r.sla_minutos)).catch(() => {});
    const t = setInterval(load, 10000);
    const clock = setInterval(() => setAgora(new Date()), 1000);
    return () => { clearInterval(t); clearInterval(clock); };
  }, [load]);

  const slaCor = (min: number) =>
    min >= slaMin ? "var(--st-failed)" : min >= slaMin / 2 ? "var(--st-pending)" : "var(--st-read)";

  // Fila agrupada por setor: quantidade + espera da mais antiga.
  const porSetor = new Map<string, { count: number; maxMin: number }>();
  for (const c of fila) {
    const s = c.setor || "Sem setor";
    const cur = porSetor.get(s) || { count: 0, maxMin: 0 };
    cur.count++;
    cur.maxMin = Math.max(cur.maxMin, minutosDesde(c.queued_at));
    porSetor.set(s, cur);
  }
  const setores = [...porSetor.entries()].sort((a, b) => b[1].maxMin - a[1].maxMin);

  // Em atendimento agrupado por atendente.
  const porAtendente = new Map<string, number>();
  for (const c of atendendo) {
    const a = c.atendente || "—";
    porAtendente.set(a, (porAtendente.get(a) || 0) + 1);
  }
  const atendentes = [...porAtendente.entries()].sort((a, b) => b[1] - a[1]);

  const maisAntiga = fila.length ? Math.max(...fila.map((c) => minutosDesde(c.queued_at))) : 0;
  const estourados = fila.filter((c) => minutosDesde(c.queued_at) >= slaMin).length;

  function telaCheia() {
    if (document.fullscreenElement) document.exitFullscreen();
    else fullRef.current?.requestFullscreen().catch(() => {});
  }

  return (
    <>
      <Topbar
        title="Fila ao Vivo"
        sub={`Atualiza a cada 10s · SLA: ${slaMin} min`}
        actions={
          <Button variant="secondary" icon="grid" onClick={telaCheia}>
            Tela cheia (TV)
          </Button>
        }
      />
      <div className="content" ref={fullRef} style={{ background: "var(--bg-base, var(--bg-sunken))" }}>
        <div className="content-pad col" style={{ gap: 18 }}>
          {/* Relógio + status */}
          <div className="row between center">
            <div className="row center" style={{ gap: 14 }}>
              <span className="mono" style={{ fontSize: 40, fontWeight: 800, letterSpacing: "-0.02em" }}>
                {agora.toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit", second: "2-digit" })}
              </span>
              <span className="muted" style={{ fontSize: 15 }}>
                {agora.toLocaleDateString("pt-BR", { weekday: "long", day: "numeric", month: "long" })}
              </span>
            </div>
            {erro && (
              <span className="badge badge-failed" style={{ fontSize: 13 }}>
                <Icon name="alertCircle" size={14} /> Sem conexão com a API
              </span>
            )}
          </div>

          {/* Números grandes */}
          <div style={{ display: "grid", gridTemplateColumns: "repeat(4, 1fr)", gap: 14 }}>
            {(
              [
                ["Na fila", String(fila.length), fila.length > 0 ? "var(--st-pending)" : "var(--st-read)", "clock"],
                ["Mais antiga", fila.length ? maisAntiga + " min" : "—", slaCor(maisAntiga), "history"],
                ["SLA estourado", String(estourados), estourados > 0 ? "var(--st-failed)" : "var(--st-read)", "alertCircle"],
                ["Em atendimento", String(atendendo.length), "var(--brand-text)", "users"],
              ] as [string, string, string, "clock" | "history" | "alertCircle" | "users"][]
            ).map(([label, valor, cor, ico]) => (
              <div key={label} className="card card-pad col center" style={{ gap: 6, padding: "26px 16px" }}>
                <span className="row center" style={{ gap: 8, fontSize: 14, color: "var(--text-tertiary)", fontWeight: 600 }}>
                  <Icon name={ico} size={16} /> {label}
                </span>
                <span className="mono" style={{ fontSize: 52, fontWeight: 800, lineHeight: 1, color: cor }}>
                  {valor}
                </span>
              </div>
            ))}
          </div>

          <div className="grid-2" style={{ alignItems: "start" }}>
            {/* Fila por setor */}
            <div className="card">
              <div className="card-head">
                <div className="card-title" style={{ fontSize: 16 }}>Fila por departamento</div>
              </div>
              {setores.length === 0 ? (
                <div className="card-pad col center" style={{ gap: 8, padding: 36, color: "var(--st-read)" }}>
                  <Icon name="checkCircle" size={34} />
                  <span style={{ fontSize: 16, fontWeight: 700 }}>Fila vazia 🎉</span>
                </div>
              ) : (
                <div className="col">
                  {setores.map(([setor, info]) => (
                    <div key={setor} className="row between center" style={{ padding: "14px 18px", borderBottom: "1px solid var(--border-subtle)" }}>
                      <span className="row center" style={{ gap: 10, fontSize: 16, fontWeight: 700 }}>
                        <span style={{ width: 11, height: 11, borderRadius: 99, background: SETOR_COR[setor] || "var(--text-faint)" }} />
                        {setor}
                      </span>
                      <span className="row center" style={{ gap: 14 }}>
                        <span className="mono" style={{ fontSize: 15, color: slaCor(info.maxMin), fontWeight: 700 }}>
                          ⏱ {info.maxMin} min
                        </span>
                        <span
                          className="mono"
                          style={{
                            fontSize: 18, fontWeight: 800, minWidth: 40, textAlign: "center", padding: "3px 10px",
                            borderRadius: 9, background: "var(--st-pending-soft)", color: "var(--st-pending)",
                          }}
                        >
                          {info.count}
                        </span>
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </div>

            {/* Atendentes ativos */}
            <div className="card">
              <div className="card-head">
                <div className="card-title" style={{ fontSize: 16 }}>Em atendimento agora</div>
              </div>
              {atendentes.length === 0 ? (
                <div className="card-pad muted" style={{ padding: 36, textAlign: "center", fontSize: 14 }}>
                  Nenhum atendimento em andamento.
                </div>
              ) : (
                <div className="col">
                  {atendentes.map(([nome, count]) => (
                    <div key={nome} className="row between center" style={{ padding: "14px 18px", borderBottom: "1px solid var(--border-subtle)" }}>
                      <span style={{ fontSize: 16, fontWeight: 700 }}>{nome}</span>
                      <span
                        className="mono"
                        style={{
                          fontSize: 18, fontWeight: 800, minWidth: 40, textAlign: "center", padding: "3px 10px",
                          borderRadius: 9, background: "var(--brand-soft)", color: "var(--brand-text)",
                        }}
                      >
                        {count}
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          </div>
        </div>
      </div>
    </>
  );
}
