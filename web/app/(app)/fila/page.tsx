"use client";

import { ReactNode, useCallback, useEffect, useRef, useState } from "react";
import { useRouter } from "next/navigation";
import { Topbar } from "@/components/shell/Topbar";
import { Alert, Avatar, Button, EmptyState } from "@/components/ui";
import { claimConversation, listConversations, getRoteamento, type ApiConversation } from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { SETOR_COR } from "@/lib/data";

// Painel de fila ao vivo (pensado pra TV/monitor da recepção): fila por setor
// com SLA visual, atendimentos em andamento por atendente, relógio. Atualiza
// sozinho a cada 10s; botão de tela cheia esconde o resto do painel.

function minutosDesde(iso: string | null): number {
  if (!iso) return 0;
  return Math.max(0, Math.floor((Date.now() - new Date(iso).getTime()) / 60000));
}

export default function FilaPage() {
  const router = useRouter();
  const { user } = useAuth();
  // Módulo "fila" desligado no órgão → sem acesso ao painel.
  useEffect(() => {
    if (user && user.modulos && !user.modulos.fila) router.replace("/inbox");
  }, [user, router]);
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

  // Setor selecionado no card filtra a tabela de espera.
  const [setorSel, setSetorSel] = useState<string | null>(null);
  const linhas = [...fila]
    .filter((c) => !setorSel || (c.setor || "Sem setor") === setorSel)
    .sort((x, y) => minutosDesde(y.queued_at) - minutosDesde(x.queued_at));
  const setorMaisAntigo = setores[0]?.[0];
  const atendPorSetor = (setor: string) => atendendo.filter((c) => (c.setor || "Sem setor") === setor).length;

  async function assumir(c: ApiConversation) {
    try {
      await claimConversation(c.id, user?.nome ?? "Atendente");
      router.push("/inbox?conv=" + c.id);
    } catch {
      setErro(true);
    }
  }

  const resumo: [string, ReactNode][] = [
    ["Aguardando", fila.length.toLocaleString("pt-BR")],
    [
      "Maior espera",
      <span key="m" className="row" style={{ alignItems: "baseline", gap: 8 }}>
        <span style={{ color: fila.length ? slaCor(maisAntiga) : undefined }}>{fila.length ? maisAntiga + " min" : "—"}</span>
        {setorMaisAntigo && <span style={{ fontFamily: "var(--font-sans)", fontSize: 12, fontWeight: 400, color: "var(--text-tertiary)" }}>{setorMaisAntigo}</span>}
      </span>,
    ],
    ["Fora do SLA", <span key="e" style={{ color: estourados > 0 ? "var(--st-failed)" : undefined }}>{estourados}</span>],
    ["Em atendimento", atendendo.length.toLocaleString("pt-BR")],
    ["Atendentes atendendo", atendentes.filter(([n]) => n !== "—").length],
  ];

  return (
    <>
      <Topbar
        title="Fila ao Vivo"
        sub={`Atualiza a cada 10 s · meta de espera: ${slaMin} min`}
        actions={
          <Button variant="secondary" size="sm" icon="grid" onClick={telaCheia}>
            Tela cheia
          </Button>
        }
      />
      <div className="content" ref={fullRef} style={{ background: "var(--bg-base)" }}>
        <div className="content-pad col" style={{ gap: 20, maxWidth: 1280 }}>
          <div className="row between center wrap" style={{ gap: 12 }}>
            <div className="row center" style={{ gap: 12, alignItems: "baseline" }}>
              <span className="mono" style={{ fontSize: 28, lineHeight: "32px", fontWeight: 600 }}>
                {agora.toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit", second: "2-digit" })}
              </span>
              <span style={{ fontSize: 14, color: "var(--text-secondary)" }}>
                {agora.toLocaleDateString("pt-BR", { weekday: "long", day: "numeric", month: "long" })}
              </span>
            </div>
            {erro && (
              <Alert tone="err" style={{ flex: "0 1 auto" }} action={<Button size="sm" icon="refresh" onClick={load}>Tentar novamente</Button>}>
                <b>Sem conexão com o servidor.</b> Os números podem estar desatualizados.
              </Alert>
            )}
          </div>

          <div className="zg-card row wrap" style={{ boxShadow: "none", alignItems: "stretch" }}>
            {resumo.map(([label, valor], i) => (
              <div key={label} style={{ flex: "1 1 160px", padding: "14px 20px", borderRight: i < resumo.length - 1 ? "1px solid var(--border-subtle)" : 0 }}>
                <div style={{ fontSize: 12, color: "var(--text-secondary)", fontWeight: 500 }}>{label}</div>
                <div className="mono" style={{ fontSize: 24, lineHeight: "30px", fontWeight: 600 }}>{valor}</div>
              </div>
            ))}
          </div>

          {setores.length === 0 ? (
            <div className="zg-card">
              <EmptyState icon="checkCircle" title="Ninguém aguardando agora" sub="Novas conversas encaminhadas pelo chatbot aparecem aqui em até 10 segundos." />
            </div>
          ) : (
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fit, minmax(196px, 1fr))", gap: 12 }}>
              {setores.map(([setor, info]) => {
                const sel = setorSel === setor;
                const late = info.maxMin >= slaMin;
                return (
                  <button
                    key={setor}
                    className="zg-card col"
                    aria-pressed={sel}
                    onClick={() => setSetorSel(sel ? null : setor)}
                    style={{ gap: 10, padding: "14px 16px", textAlign: "left", font: "inherit", color: "inherit", cursor: "pointer", borderColor: sel ? "var(--brand)" : undefined, boxShadow: sel ? "0 0 0 3px var(--brand-soft)" : undefined }}
                  >
                    <span className="row center" style={{ gap: 8, width: "100%" }}>
                      <span className="tag-dot" style={{ width: 8, height: 8, borderRadius: 99, background: SETOR_COR[setor] || "var(--text-faint)" }} />
                      <span className="grow" style={{ fontWeight: 600, fontSize: 14 }}>{setor}</span>
                      {late && <span className="badge badge-failed" style={{ height: 20, padding: "0 7px", fontSize: 11 }}>SLA</span>}
                    </span>
                    <span className="row" style={{ alignItems: "baseline", gap: 6 }}>
                      <span className="mono" style={{ fontSize: 28, lineHeight: "32px", fontWeight: 600 }}>{info.count}</span>
                      <span style={{ fontSize: 13, color: "var(--text-secondary)" }}>aguardando</span>
                    </span>
                    <span className="row" style={{ gap: 12, fontSize: 12, color: "var(--text-secondary)" }}>
                      <span>
                        Maior espera <b className="mono" style={{ color: slaCor(info.maxMin), fontWeight: 600 }}>{info.maxMin} min</b>
                      </span>
                    </span>
                    <span style={{ fontSize: 12, color: "var(--text-secondary)" }}>{atendPorSetor(setor)} em atendimento</span>
                  </button>
                );
              })}
            </div>
          )}

          <section className="zg-table-wrap" aria-labelledby="espera-title">
            <div className="zg-table-toolbar" style={{ flexWrap: "wrap" }}>
              <h2 id="espera-title" style={{ fontSize: 15, lineHeight: "22px", fontWeight: 600 }}>Aguardando atendimento</h2>
              <span className="zg-count">{linhas.length}</span>
              {setorSel && <span style={{ fontSize: 13, color: "var(--text-tertiary)" }}>Setor: {setorSel}</span>}
              <span className="grow" />
              {setorSel && (
                <button className="btn btn-ghost btn-sm" onClick={() => setSetorSel(null)}>
                  Limpar filtro
                </button>
              )}
              <span style={{ fontSize: 12, color: "var(--text-tertiary)" }}>Mais antigas primeiro</span>
            </div>
            {linhas.length === 0 ? (
              <div style={{ padding: 24, textAlign: "center", fontSize: 13, color: "var(--text-tertiary)" }}>Ninguém aguardando.</div>
            ) : (
              <div style={{ overflowX: "auto" }}>
                <table className="zg-table">
                  <thead>
                    <tr>
                      <th style={{ width: 32 }}>#</th>
                      <th>Cidadão</th>
                      <th>Setor</th>
                      <th className="num">Aguardando</th>
                      <th>SLA</th>
                      <th>
                        <span style={{ position: "absolute", left: -9999 }}>Ações</span>
                      </th>
                    </tr>
                  </thead>
                  <tbody>
                    {linhas.map((c, i) => {
                      const min = minutosDesde(c.queued_at);
                      const st = min >= slaMin ? ["badge badge-failed", "Fora do SLA"] : min >= slaMin / 2 ? ["badge badge-pending", "Perto do limite"] : ["badge badge-read", "No prazo"];
                      return (
                        <tr key={c.id}>
                          <td className="num" style={{ textAlign: "left", color: "var(--text-tertiary)" }}>{i + 1}</td>
                          <td>
                            <b style={{ fontWeight: 600 }}>{c.contact_name || c.contact_phone}</b>
                            <span className="sub mono">{c.contact_phone}</span>
                          </td>
                          <td>{c.setor || "Sem setor"}</td>
                          <td className="num" style={{ color: slaCor(min), fontWeight: 600 }}>{min} min</td>
                          <td>
                            <span className={st[0]}>{st[1]}</span>
                          </td>
                          <td style={{ textAlign: "right" }}>
                            <Button variant="primary" size="sm" icon="userCheck" onClick={() => assumir(c)}>
                              Assumir
                            </Button>
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
            <div className="zg-table-foot">
              <span>{fila.length} na fila · {atendendo.length} em atendimento</span>
              <span>Tempo de espera conta a partir do encaminhamento pelo chatbot</span>
            </div>
          </section>

          {atendentes.length > 0 && (
            <section className="zg-table-wrap" aria-labelledby="atend-title">
              <div className="zg-table-toolbar">
                <h2 id="atend-title" style={{ fontSize: 15, lineHeight: "22px", fontWeight: 600 }}>Em atendimento agora</h2>
                <span className="zg-count">{atendendo.length}</span>
              </div>
              <table className="zg-table">
                <tbody>
                  {atendentes.map(([nome, count]) => (
                    <tr key={nome}>
                      <td>
                        <span className="row center" style={{ gap: 10 }}>
                          <Avatar name={nome} size={28} />
                          {nome}
                        </span>
                      </td>
                      <td className="num">{count} {count === 1 ? "conversa" : "conversas"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            </section>
          )}
        </div>
      </div>
    </>
  );
}
