"use client";

import { Fragment, useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Topbar } from "@/components/shell/Topbar";
import {
  Avatar,
  Button,
  Donut,
  EmptyState,
  Icon,
  Input,
  Skel,
  StatusBadge,
  useToast,
} from "@/components/ui";
import { SETORES, SETOR_COR } from "@/lib/data";
import { listCampaigns, exportCampaignsCSV, cancelCampaign, type ApiCampaign } from "@/lib/api";
import { IconName } from "@/lib/icons";

const STATUS_OPTS = ["Todos", "enviado", "enviando", "agendado", "cancelado", "falhou", "revisao"];

// status de campanha → status visual do StatusBadge
function vis(s: string): string {
  switch (s) {
    case "enviando":
      return "pendente";
    case "enviado":
      return "enviado";
    case "falhou":
      return "falhou";
    case "agendado":
      return "agendado";
    default:
      return s;
  }
}

function fmtData(iso: string): string {
  return new Date(iso).toLocaleString("pt-BR", { day: "2-digit", month: "2-digit", year: "numeric", hour: "2-digit", minute: "2-digit" });
}

export default function HistoricoPage() {
  const router = useRouter();
  const toast = useToast();
  const [q, setQ] = useState("");
  const [setor, setSetor] = useState("Todos");
  const [status, setStatus] = useState("Todos");
  const [expand, setExpand] = useState<number | null>(null);
  const [list, setList] = useState<ApiCampaign[]>([]);
  const [loading, setLoading] = useState(true);
  const [erro, setErro] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setList(await listCampaigns());
      setErro(null);
    } catch (e) {
      setErro((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
    // Disparos rodam na fila (worker) → atualiza pra ver enviadas/falhas subir.
    const t = setInterval(load, 5000);
    return () => clearInterval(t);
  }, [load]);

  async function cancelarAgendado(id: number) {
    try {
      await cancelCampaign(id);
      toast({ type: "success", title: "Agendamento cancelado" });
      load();
    } catch (e) {
      toast({ type: "error", title: "Erro ao cancelar", message: (e as Error).message });
    }
  }

  async function exportar() {
    try {
      await exportCampaignsCSV();
    } catch (e) {
      toast({ type: "error", title: "Erro ao exportar", message: (e as Error).message });
    }
  }

  const rows = list.filter(
    (d) =>
      (setor === "Todos" || d.setor === setor) &&
      (status === "Todos" || d.status === status) &&
      (!q || d.template.toLowerCase().includes(q.toLowerCase()) || d.operador.toLowerCase().includes(q.toLowerCase())),
  );

  const totalEnv = list.reduce((a, d) => a + d.enviadas, 0);
  const totalEntr = list.reduce((a, d) => a + d.entregues, 0);
  const totalFalhas = list.reduce((a, d) => a + d.falhas, 0);
  const pct = (n: number, total: number) => (total > 0 ? Math.min(100, (n / total) * 100) + "%" : "0%");

  const stats: [string, string, IconName, string, string][] = [
    ["Disparos", list.length.toLocaleString("pt-BR"), "send", "var(--brand-text)", "var(--brand-soft)"],
    ["Total enviadas", totalEnv.toLocaleString("pt-BR"), "checkDouble", "var(--st-delivered)", "var(--st-delivered-soft)"],
    ["Total entregues", totalEntr.toLocaleString("pt-BR"), "eye", "var(--st-read)", "var(--st-read-soft)"],
    ["Total falhas", totalFalhas.toLocaleString("pt-BR"), "alertCircle", "var(--st-failed)", "var(--st-failed-soft)"],
  ];

  return (
    <>
      <Topbar
        title="Campanhas"
        sub="Disparos em massa enviados e agendados"
        actions={
          <>
            <Button variant="secondary" size="sm" icon="download" onClick={exportar} disabled={list.length === 0}>
              Exportar CSV
            </Button>
            <Button variant="ghost" size="sm" icon="refresh" onClick={load} aria-label="Atualizar" />
            <Button variant="primary" size="sm" icon="send" onClick={() => router.push("/disparo")}>
              Novo disparo
            </Button>
          </>
        }
      />
      <div className="content">
        <div className="content-pad col" style={{ gap: 20, maxWidth: 1280 }}>
          <div className="zg-card row wrap" style={{ boxShadow: "none", alignItems: "stretch" }}>
            {stats.map((s, i) => (
              <div key={i} style={{ flex: "1 1 160px", padding: "14px 20px", borderRight: i < stats.length - 1 ? "1px solid var(--border-subtle)" : 0 }}>
                <div className="row center" style={{ gap: 6, fontSize: 12, color: "var(--text-secondary)", fontWeight: 500 }}>
                  <Icon name={s[2]} size={14} style={{ color: s[3] }} />
                  {s[0]}
                </div>
                <div className="mono" style={{ fontSize: 24, lineHeight: "30px", fontWeight: 600 }}>{s[1]}</div>
              </div>
            ))}
          </div>

          <div className="card card-pad" style={{ padding: "12px 16px" }}>
            <div className="toolbar">
              <div className="toolbar-search">
                <Input icon="search" placeholder="Buscar por template ou operador…" value={q} onChange={(e) => setQ(e.target.value)} />
              </div>
              <select className="select" style={{ width: "auto" }} value={setor} onChange={(e) => setSetor(e.target.value)}>
                {["Todos", ...SETORES].map((s) => (
                  <option key={s}>{s}</option>
                ))}
              </select>
              <select className="select" style={{ width: "auto" }} value={status} onChange={(e) => setStatus(e.target.value)}>
                {STATUS_OPTS.map((s) => (
                  <option key={s} value={s}>
                    {s === "Todos" ? "Todos os status" : s}
                  </option>
                ))}
              </select>
              <div className="grow" />
              <span className="muted" style={{ fontSize: 13 }}>
                {rows.length} resultados
              </span>
            </div>
          </div>

          <div className="card" style={{ overflow: "hidden" }}>
            {erro ? (
              <EmptyState error title="Não foi possível carregar as campanhas" sub={erro} action={<Button size="sm" icon="refresh" onClick={load}>Tentar novamente</Button>} />
            ) : loading ? (
              <div className="card-pad col" style={{ gap: 10 }}>
                {[0, 1, 2].map((i) => (
                  <Skel key={i} h={28} />
                ))}
              </div>
            ) : rows.length === 0 ? (
              <EmptyState icon="megaphone" title="Nenhuma campanha ainda" sub="Quando você enviar ou agendar um disparo, ele aparece aqui com a entrega e a leitura." action={<Button size="sm" variant="primary" icon="send" onClick={() => router.push("/disparo")}>Novo disparo</Button>} />
            ) : (
              <div style={{ overflowX: "auto" }}>
                <table className="ds-table">
                  <thead>
                    <tr>
                      <th style={{ width: 34 }}></th>
                      <th>Data/Hora</th>
                      <th>Template</th>
                      <th>Setor</th>
                      <th className="num">Enviadas</th>
                      <th>Entrega</th>
                      <th className="num">Lidas</th>
                      <th className="num">Falhas</th>
                      <th>Status</th>
                      <th>Operador</th>
                    </tr>
                  </thead>
                  <tbody>
                    {rows.map((d) => {
                      const taxaE = d.enviadas ? (d.entregues / d.enviadas) * 100 : 0;
                      const isOpen = expand === d.id;
                      return (
                        <Fragment key={d.id}>
                          <tr className="expandable" onClick={() => setExpand(isOpen ? null : d.id)}>
                            <td>
                              <span className="icon" style={{ color: "var(--text-tertiary)", transform: isOpen ? "rotate(90deg)" : "none", transition: "transform .15s" }}>
                                <Icon name="chevronRight" size={15} />
                              </span>
                            </td>
                            <td className="mono" style={{ fontSize: 12, whiteSpace: "nowrap" }}>
                              {fmtData(d.criado_em)}
                            </td>
                            <td className="strong">
                              {d.template}
                              <span className="mono faint" style={{ fontSize: 11 }}>
                                {" "}
                                · {d.language}
                              </span>
                            </td>
                            <td>
                              <span className="tag">
                                <span className="tag-dot" style={{ background: SETOR_COR[d.setor] || "var(--text-tertiary)" }} /> {d.setor}
                              </span>
                            </td>
                            <td className="num mono strong">{d.enviadas.toLocaleString("pt-BR")}</td>
                            <td>
                              <div className="row center" style={{ gap: 8 }} title={`Lidas ${d.lidas} · Entregues ${d.entregues} · Falhas ${d.falhas}`}>
                                <span className="zg-progress">
                                  <span className="read" style={{ width: pct(d.lidas, d.enviadas) }} />
                                  <span className="delivered" style={{ width: pct(Math.max(0, d.entregues - d.lidas), d.enviadas) }} />
                                  <span className="failed" style={{ width: pct(d.falhas, d.enviadas) }} />
                                </span>
                                <span className="mono" style={{ fontSize: 12 }}>
                                  {taxaE.toFixed(0)}%
                                </span>
                              </div>
                            </td>
                            <td className="num mono" style={{ color: d.lidas > 0 ? "var(--st-read)" : "var(--text-tertiary)" }}>
                              {d.lidas || "0"}
                            </td>
                            <td className="num mono" style={{ color: d.falhas > 0 ? "var(--st-failed)" : "var(--text-tertiary)" }}>
                              {d.falhas || "0"}
                            </td>
                            <td>
                              <StatusBadge status={vis(d.status)} />
                            </td>
                            <td>
                              <div className="row center" style={{ gap: 7 }}>
                                <Avatar name={d.operador || "—"} size={22} />
                                <span style={{ fontSize: 12 }}>{(d.operador || "—").split(" ")[0]}</span>
                              </div>
                            </td>
                          </tr>
                          {isOpen && (
                            <tr>
                              <td colSpan={10} style={{ padding: 0, background: "var(--bg-sunken)" }}>
                                <div style={{ padding: "18px 22px", display: "grid", gridTemplateColumns: "repeat(7, minmax(0, 1fr)) auto", gap: 24, alignItems: "center" }}>
                                  {(
                                    [
                                      ["Total", d.total, "var(--text-primary)"],
                                      ["Enviadas", d.enviadas, "var(--st-delivered)"],
                                      ["Entregues", d.entregues, "var(--st-read)"],
                                      ["Lidas", d.lidas, "var(--st-read)"],
                                      ["Falhas", d.falhas, "var(--st-failed)"],
                                      ["Descadastrados", d.puladas || 0, "var(--text-secondary)"],
                                      ["Conferir envio", d.revisao || 0, "var(--st-failed)"],
                                    ] as [string, number, string][]
                                  ).map((s, i) => (
                                    <div key={i} className="stat-inline">
                                      <span className="v" style={{ color: s[2] }}>
                                        {s[1].toLocaleString("pt-BR")}
                                      </span>
                                      <span className="l">{s[0]}</span>
                                    </div>
                                  ))}
                                  <Donut value={d.enviadas ? (d.entregues / d.enviadas) * 100 : 0} size={72} stroke={8} color="var(--st-read)" label="entrega" />
                                  {d.revisao > 0 && <p style={{ gridColumn: "1 / -1", fontSize: 12 }}>
                                    Não foi possível confirmar todos os envios. Confira o histórico e a entrega no WhatsApp antes de enviar novamente.
                                  </p>}
                                  {d.status === "agendado" && (
                                    <Button
                                      variant="danger"
                                      size="sm"
                                      icon="ban"
                                      onClick={(e) => { e.stopPropagation(); cancelarAgendado(d.id); }}
                                    >
                                      Cancelar agendamento
                                    </Button>
                                  )}
                                </div>
                              </td>
                            </tr>
                          )}
                        </Fragment>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      </div>
    </>
  );
}
