"use client";

import { useCallback, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Topbar } from "@/components/shell/Topbar";
import {
  Button,
  Icon,
  Segmented,
  BarChart,
  Donut,
  StatusBadge,
  Skel,
  Avatar,
} from "@/components/ui";
import { SETOR_COR } from "@/lib/data";
import {
  getKpis, getVolume, getActivity, getAtendentes, getSatisfacao,
  type ApiKpis, type ApiVolumePoint, type ApiAtividade, type ApiAtendenteStat, type ApiSatisfacao,
} from "@/lib/api";
import { IconName } from "@/lib/icons";

function KpiCard({
  label,
  value,
  suffix,
  delta,
  icon,
  color,
}: {
  label: string;
  value: string;
  suffix?: string;
  delta: number;
  icon: IconName;
  color: { c: string; b: string };
}) {
  const up = delta >= 0;
  return (
    <div className="card kpi">
      <div className="kpi-top">
        <span className="kpi-label">{label}</span>
        <span className="kpi-ico" style={{ background: color.b, color: color.c }}>
          <Icon name={icon} size={16} />
        </span>
      </div>
      <div className="row between" style={{ alignItems: "flex-end" }}>
        <div className="kpi-value">
          {value}
          {suffix && <span style={{ fontSize: 16, color: "var(--text-tertiary)", marginLeft: 1 }}>{suffix}</span>}
        </div>
      </div>
      <div className="row center" style={{ gap: 7 }}>
        <span className={"kpi-delta " + (up ? "kpi-up" : "kpi-down")}>
          <Icon name={up ? "trendUp" : "trendDown"} size={13} />
          {(up ? "+" : "") + delta + "%"}
        </span>
        <span style={{ fontSize: 11.5, color: "var(--text-faint)" }}>vs. ontem</span>
      </div>
    </div>
  );
}

export default function DashboardPage() {
  const router = useRouter();
  const [periodo, setPeriodo] = useState<"hora" | "dia">("hora");
  const [kpis, setKpis] = useState<ApiKpis | null>(null);
  const [dados, setDados] = useState<ApiVolumePoint[]>([]);
  const [atividade, setAtividade] = useState<ApiAtividade[]>([]);
  const [atendentes, setAtendentes] = useState<ApiAtendenteStat[]>([]);
  const [sat, setSat] = useState<ApiSatisfacao | null>(null);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async (p: "hora" | "dia") => {
    setLoading(true);
    try {
      const [k, v, a, at, sa] = await Promise.all([
        getKpis(), getVolume(p), getActivity(), getAtendentes().catch(() => []),
        getSatisfacao().catch(() => null),
      ]);
      setKpis(k);
      setDados(v);
      setAtividade(a);
      setAtendentes(at);
      setSat(sa);
    } catch { /* mantém o que já tem */ } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(periodo); }, [periodo, load]);

  const trocarPeriodo = (p: string) => setPeriodo(p as "hora" | "dia");

  const falha = kpis ? Math.max(0, 100 - kpis.taxaEntrega) : 0;

  return (
    <>
      <Topbar
        title="Dashboard"
        sub={"Visão geral de hoje — " + new Date().toLocaleDateString("pt-BR", { weekday: "long", day: "numeric", month: "long" })}
        actions={
          <>
            <Button variant="secondary" icon="refresh" onClick={() => load(periodo)}>
              Atualizar
            </Button>
            <Button variant="primary" icon="send" onClick={() => router.push("/disparo")}>
              Novo Disparo
            </Button>
          </>
        }
      />
      <div className="content">
        <div className="content-pad col" style={{ gap: 16 }}>
          <div className="kpi-grid">
            {!kpis ? (
              [0, 1, 2, 3].map((i) => <Skel key={i} h={118} />)
            ) : (
              <>
                <KpiCard label="Enviadas hoje" value={kpis.enviadasHoje.toLocaleString("pt-BR")} delta={kpis.enviadasDelta} icon="send" color={{ c: "var(--brand-text)", b: "var(--brand-soft)" }} />
                <KpiCard label="Taxa de entrega" value={kpis.taxaEntrega.toFixed(1)} suffix="%" delta={kpis.entregaDelta} icon="checkDouble" color={{ c: "var(--st-delivered)", b: "var(--st-delivered-soft)" }} />
                <KpiCard label="Taxa de leitura" value={kpis.taxaLeitura.toFixed(1)} suffix="%" delta={kpis.leituraDelta} icon="eye" color={{ c: "var(--st-read)", b: "var(--st-read-soft)" }} />
                <KpiCard label="Falhas" value={kpis.falhas.toLocaleString("pt-BR")} delta={kpis.falhasDelta} icon="alertCircle" color={{ c: "var(--st-failed)", b: "var(--st-failed-soft)" }} />
              </>
            )}
          </div>

          <div className="grid-2">
            {/* Gráfico de volume */}
            <div className="card">
              <div className="card-head">
                <div>
                  <div className="card-title">Volume de mensagens</div>
                  <div className="card-sub">Enviadas vs. lidas</div>
                </div>
                <Segmented
                  value={periodo}
                  onChange={trocarPeriodo}
                  options={[
                    { value: "hora", label: "Por hora" },
                    { value: "dia", label: "7 dias" },
                  ]}
                />
              </div>
              <div className="card-pad">
                {dados.length === 0 ? (
                  <div className="col center" style={{ height: 188, justifyContent: "center", color: "var(--text-faint)", fontSize: 13 }}>
                    {loading ? "Carregando…" : "Sem mensagens no período"}
                  </div>
                ) : (
                  <BarChart data={dados} secondaryKey="lidas" valueKey="value" height={188} color="var(--brand)" />
                )}
                <div className="row center" style={{ gap: 18, marginTop: 16, paddingTop: 14, borderTop: "1px solid var(--border-subtle)" }}>
                  <div className="row center" style={{ gap: 7, fontSize: 12, color: "var(--text-secondary)" }}>
                    <span style={{ width: 10, height: 10, borderRadius: 3, background: "var(--brand)" }} /> Enviadas
                  </div>
                  <div className="row center" style={{ gap: 7, fontSize: 12, color: "var(--text-secondary)" }}>
                    <span style={{ width: 10, height: 10, borderRadius: 3, background: "var(--st-read)" }} /> Lidas
                  </div>
                </div>
              </div>
            </div>

            {/* Coluna direita */}
            <div className="col" style={{ gap: 16 }}>
              <div className="card card-pad col" style={{ gap: 14, alignItems: "center" }}>
                <div className="card-title" style={{ alignSelf: "flex-start" }}>
                  Engajamento
                </div>
                <Donut value={kpis?.taxaLeitura ?? 0} color="var(--st-read)" size={128} stroke={12} label="lidas" />
                <div className="row" style={{ gap: 20, fontSize: 12 }}>
                  <div className="col center" style={{ gap: 1 }}>
                    <span className="mono" style={{ fontWeight: 700, fontSize: 15 }}>
                      {(kpis?.taxaEntrega ?? 0).toFixed(1)}%
                    </span>
                    <span className="faint" style={{ fontSize: 11 }}>
                      entregue
                    </span>
                  </div>
                  <div className="col center" style={{ gap: 1 }}>
                    <span className="mono" style={{ fontWeight: 700, fontSize: 15, color: "var(--st-failed)" }}>
                      {falha.toFixed(1)}%
                    </span>
                    <span className="faint" style={{ fontSize: 11 }}>
                      falha
                    </span>
                  </div>
                </div>
              </div>
              {sat && sat.enviadas > 0 && (
                <div className="card card-pad col" style={{ gap: 8 }}>
                  <div className="card-title">Satisfação (30 dias)</div>
                  <div className="row center" style={{ gap: 14 }}>
                    <span className="mono" style={{ fontSize: 34, fontWeight: 700, color: "var(--st-pending)" }}>
                      {sat.media > 0 ? sat.media.toFixed(1) : "—"}
                    </span>
                    <div className="col" style={{ gap: 1, fontSize: 12, color: "var(--text-tertiary)" }}>
                      <span>{"⭐".repeat(Math.round(sat.media || 0))}</span>
                      <span>{sat.respostas} respostas de {sat.enviadas} pesquisas ({sat.enviadas > 0 ? Math.round((sat.respostas / sat.enviadas) * 100) : 0}%)</span>
                    </div>
                  </div>
                </div>
              )}
              <div className="card card-pad col" style={{ gap: 12 }}>
                <div className="row between center">
                  <div className="card-title">Status da conexão</div>
                  <span className="badge badge-read">
                    <span className="dot" /> Operacional
                  </span>
                </div>
                {[["Meta Cloud API"], ["Webhook de entrada"], ["Número verde"]].map((r, i) => (
                  <div key={i} className="row between center" style={{ fontSize: 12.5 }}>
                    <span className="t-sec">{r[0]}</span>
                    <span className="row center" style={{ gap: 6, color: "var(--st-read)", fontWeight: 600 }}>
                      <span style={{ width: 6, height: 6, borderRadius: 99, background: "var(--st-read)" }} /> OK
                    </span>
                  </div>
                ))}
                <div style={{ fontSize: 11, color: "var(--text-faint)", marginTop: 2 }}>
                  Última verificação há 30s · 99.98% uptime (30d)
                </div>
              </div>
            </div>
          </div>

          {/* Feed de atividade */}
          <div className="card">
            <div className="card-head">
              <div className="card-title">Atividade recente</div>
              <a
                className="row center"
                style={{ gap: 5, fontSize: 12.5, color: "var(--brand-text)", fontWeight: 600, cursor: "pointer" }}
                onClick={() => router.push("/historico")}
              >
                Ver histórico completo <Icon name="chevronRight" size={14} />
              </a>
            </div>
            <div>
              {atividade.length === 0 && (
                <div style={{ padding: "26px 18px", textAlign: "center", color: "var(--text-faint)", fontSize: 13 }}>
                  {loading ? "Carregando…" : "Nenhuma atividade ainda"}
                </div>
              )}
              {atividade.map((a, i) => {
                const cfg =
                  a.tipo === "disparo"
                    ? { ic: "send" as IconName, c: "var(--brand-text)", b: "var(--brand-soft)" }
                    : a.tipo === "resposta"
                    ? { ic: "reply" as IconName, c: "var(--wa)", b: "var(--wa-soft)" }
                    : { ic: "alertCircle" as IconName, c: "var(--st-failed)", b: "var(--st-failed-soft)" };
                return (
                  <div
                    key={i}
                    className="row center"
                    style={{ gap: 13, padding: "13px 18px", borderBottom: i < atividade.length - 1 ? "1px solid var(--border-subtle)" : "none" }}
                  >
                    <span className="kpi-ico" style={{ background: cfg.b, color: cfg.c, width: 32, height: 32 }}>
                      <Icon name={cfg.ic} size={15} />
                    </span>
                    <div className="col grow" style={{ gap: 1, minWidth: 0 }}>
                      <div className="row center" style={{ gap: 8 }}>
                        <span className="strong" style={{ fontSize: 13 }}>
                          {a.titulo}
                        </span>
                        {a.status && <StatusBadge status={a.status} />}
                      </div>
                      <div className="row center" style={{ gap: 8, fontSize: 12, color: "var(--text-tertiary)" }}>
                        <span>{a.meta}</span>
                        {a.operador && (
                          <>
                            <span className="faint">·</span>
                            <span>{a.operador}</span>
                          </>
                        )}
                      </div>
                    </div>
                    <span className="tag" style={{ borderColor: "transparent", background: "transparent" }}>
                      <span className="tag-dot" style={{ background: SETOR_COR[a.setor] || "var(--text-faint)" }} /> {a.setor}
                    </span>
                    <span style={{ fontSize: 11.5, color: "var(--text-faint)", whiteSpace: "nowrap", minWidth: 64, textAlign: "right" }}>
                      {a.tempo}
                    </span>
                  </div>
                );
              })}
            </div>
          </div>

          {/* Desempenho por atendente (últimos 30 dias) */}
          <div className="card">
            <div className="card-head">
              <div>
                <div className="card-title">Desempenho por atendente</div>
                <div className="card-sub">Últimos 30 dias</div>
              </div>
            </div>
            {atendentes.length === 0 ? (
              <div className="card-pad muted" style={{ fontSize: 13, textAlign: "center" }}>
                {loading ? "Carregando…" : "Sem atendimentos no período."}
              </div>
            ) : (
              <table className="ds-table">
                <thead>
                  <tr>
                    <th>Atendente</th>
                    <th style={{ textAlign: "right" }}>Atendimentos</th>
                    <th style={{ textAlign: "right" }}>Mensagens</th>
                    <th style={{ textAlign: "right" }}>Tempo médio</th>
                    <th style={{ textAlign: "right" }}>Satisfação</th>
                  </tr>
                </thead>
                <tbody>
                  {atendentes.map((a) => (
                    <tr key={a.nome}>
                      <td className="strong">
                        <div className="row center" style={{ gap: 9 }}>
                          <Avatar name={a.nome} size={28} /> {a.nome}
                        </div>
                      </td>
                      <td style={{ textAlign: "right" }} className="mono">{a.atendimentos}</td>
                      <td style={{ textAlign: "right" }} className="mono">{a.mensagens}</td>
                      <td style={{ textAlign: "right" }} className="mono">{a.tempo_medio_min > 0 ? a.tempo_medio_min.toFixed(0) + " min" : "—"}</td>
                      <td style={{ textAlign: "right" }} className="mono">{a.satisfacao > 0 ? "⭐ " + a.satisfacao.toFixed(1) : "—"}</td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>
      </div>
    </>
  );
}
