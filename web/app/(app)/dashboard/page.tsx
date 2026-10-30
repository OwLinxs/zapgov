"use client";

import { Fragment, useCallback, useEffect, useState } from "react";
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
import { useAuth } from "@/lib/auth";
import { useMetaConn } from "@/lib/org";
import {
  getKpis, getVolume, getActivity, getAtendentes, getSatisfacao, getHeatmap,
  type ApiKpis, type ApiVolumePoint, type ApiAtividade, type ApiAtendenteStat, type ApiSatisfacao, type ApiHeatCell,
} from "@/lib/api";
import { IconName } from "@/lib/icons";

const fmt1 = (n: number) => n.toLocaleString("pt-BR", { minimumFractionDigits: 1, maximumFractionDigits: 1 });

function KpiCard({
  label,
  value,
  suffix,
  delta,
  icon,
  bomQuandoSobe = true,
}: {
  label: string;
  value: string;
  suffix?: string;
  delta: number;
  icon: IconName;
  bomQuandoSobe?: boolean; // falhas: subir é ruim
}) {
  const up = delta >= 0;
  const bom = up === bomQuandoSobe;
  return (
    <div className="zg-card zg-kpi">
      <div className="zg-kpi-head">
        {label}
        <Icon name={icon} size={16} />
      </div>
      <div className="zg-kpi-value">
        {value}
        {suffix && <small>{suffix}</small>}
      </div>
      <div className="zg-kpi-foot">
        <span className={"zg-delta " + (bom ? "zg-delta--up" : "zg-delta--down")}>
          <Icon name={up ? "trendUp" : "trendDown"} size={14} stroke={2} />
          {(up ? "+" : "") + fmt1(delta) + "%"}
        </span>
        vs. ontem
      </div>
    </div>
  );
}

const DIAS = ["Dom", "Seg", "Ter", "Qua", "Qui", "Sex", "Sáb"];

// HeatmapCard: grade 7 dias × 24 horas com intensidade proporcional ao volume
// de mensagens recebidas — mostra os horários de pico do atendimento.
function HeatmapCard({ cells }: { cells: ApiHeatCell[] }) {
  const mapa = new Map<string, number>();
  let max = 0;
  for (const c of cells) {
    mapa.set(c.dia + ":" + c.hora, c.total);
    if (c.total > max) max = c.total;
  }
  // pico (célula mais quente) para o subtítulo
  const pico = cells.reduce((a, b) => (b.total > a.total ? b : a), { dia: 0, hora: 0, total: 0 });

  return (
    <div className="card">
      <div className="card-head">
        <div>
          <div className="card-title">Horário de pico</div>
          <div className="card-sub">
            Mensagens recebidas por dia/hora (30 dias)
            {pico.total > 0 && ` · pico: ${DIAS[pico.dia]} ${String(pico.hora).padStart(2, "0")}h`}
          </div>
        </div>
      </div>
      <div className="card-pad" style={{ overflowX: "auto" }}>
        <div style={{ display: "grid", gridTemplateColumns: "34px repeat(24, 1fr)", gap: 3, minWidth: 640 }}>
          <span />
          {Array.from({ length: 24 }, (_, h) => (
            <span key={h} className="mono faint" style={{ fontSize: 11, textAlign: "center" }}>
              {h % 3 === 0 ? h : ""}
            </span>
          ))}
          {DIAS.map((d, dia) => (
            <Fragment key={d}>
              <span className="faint" style={{ fontSize: 12, fontWeight: 500, alignSelf: "center" }}>{d}</span>
              {Array.from({ length: 24 }, (_, h) => {
                const v = mapa.get(dia + ":" + h) || 0;
                const pct = max > 0 ? Math.round((v / max) * 100) : 0;
                return (
                  <div
                    key={h}
                    title={`${d} ${String(h).padStart(2, "0")}h — ${v} mensagem${v === 1 ? "" : "s"}`}
                    style={{
                      height: 16, borderRadius: 3,
                      background: v === 0 ? "var(--bg-sunken)" : `color-mix(in srgb, var(--brand) ${Math.max(pct, 12)}%, var(--bg-sunken))`,
                    }}
                  />
                );
              })}
            </Fragment>
          ))}
        </div>
      </div>
    </div>
  );
}

export default function DashboardPage() {
  const router = useRouter();
  const { user } = useAuth();
  // Atendente não acessa o dashboard — se cair aqui por URL, volta pra caixa.
  useEffect(() => {
    if (user && user.papel === "atendente") router.replace("/inbox");
  }, [user, router]);
  const [periodo, setPeriodo] = useState<"hora" | "dia">("hora");
  const [kpis, setKpis] = useState<ApiKpis | null>(null);
  const [dados, setDados] = useState<ApiVolumePoint[]>([]);
  const [atividade, setAtividade] = useState<ApiAtividade[]>([]);
  const [atendentes, setAtendentes] = useState<ApiAtendenteStat[]>([]);
  const [sat, setSat] = useState<ApiSatisfacao | null>(null);
  const [heat, setHeat] = useState<ApiHeatCell[]>([]);
  const [loading, setLoading] = useState(true);

  const load = useCallback(async (p: "hora" | "dia") => {
    setLoading(true);
    try {
      const [k, v, a, at, sa, hm] = await Promise.all([
        getKpis(), getVolume(p), getActivity(), getAtendentes().catch(() => []),
        getSatisfacao().catch(() => null), getHeatmap().catch(() => []),
      ]);
      setKpis(k);
      setDados(v);
      setAtividade(a);
      setAtendentes(at);
      setSat(sa);
      setHeat(hm);
    } catch { /* mantém o que já tem */ } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => { load(periodo); }, [periodo, load]);

  const trocarPeriodo = (p: string) => setPeriodo(p as "hora" | "dia");

  const falha = kpis ? Math.max(0, 100 - kpis.taxaEntrega) : 0;
  const conn = useMetaConn();
  const hora = new Date().getHours();
  const saudacao = hora < 12 ? "Bom dia" : hora < 18 ? "Boa tarde" : "Boa noite";
  const hoje = new Date().toLocaleDateString("pt-BR", { weekday: "long", day: "numeric", month: "long" });

  return (
    <>
      <Topbar title="Dashboard" />
      <div className="content">
        <div className="content-pad col" style={{ gap: 24, maxWidth: 1280 }}>
          <div className="row wrap" style={{ alignItems: "flex-end", gap: 16 }}>
            <div style={{ flex: "1 1 300px" }}>
              <h2 className="t-display">
                {saudacao}
                {user?.nome ? ", " + user.nome.split(" ")[0] : ""}
              </h2>
              <p style={{ marginTop: 4, color: "var(--text-secondary)" }}>{hoje.charAt(0).toUpperCase() + hoje.slice(1)}</p>
            </div>
            <Button variant="secondary" size="sm" icon="refresh" onClick={() => load(periodo)}>
              Atualizar
            </Button>
            <Button variant="primary" size="sm" icon="send" onClick={() => router.push("/disparo")}>
              Novo disparo
            </Button>
          </div>
          <div className="kpi-grid">
            {!kpis ? (
              [0, 1, 2, 3].map((i) => (
                <div key={i} className="zg-card zg-kpi">
                  <Skel w="50%" h={14} />
                  <Skel w="40%" h={28} />
                  <Skel w="60%" h={12} />
                </div>
              ))
            ) : (
              <>
                <KpiCard label="Enviadas hoje" value={kpis.enviadasHoje.toLocaleString("pt-BR")} delta={kpis.enviadasDelta} icon="send" />
                <KpiCard label="Taxa de entrega" value={fmt1(kpis.taxaEntrega)} suffix="%" delta={kpis.entregaDelta} icon="checkDouble" />
                <KpiCard label="Taxa de leitura" value={fmt1(kpis.taxaLeitura)} suffix="%" delta={kpis.leituraDelta} icon="eye" />
                <KpiCard label="Falhas" value={kpis.falhas.toLocaleString("pt-BR")} delta={kpis.falhasDelta} icon="alertCircle" bomQuandoSobe={false} />
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
                    { value: "hora", label: "Hoje" },
                    { value: "dia", label: "7 dias" },
                  ]}
                />
              </div>
              <div className="card-pad">
                {dados.length === 0 ? (
                  <div className="col center" style={{ height: 188, justifyContent: "center", color: "var(--text-tertiary)", fontSize: 13 }}>
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
                    <span className="mono" style={{ fontWeight: 600, fontSize: 15 }}>
                      {fmt1(kpis?.taxaEntrega ?? 0)}%
                    </span>
                    <span className="faint" style={{ fontSize: 11 }}>
                      entregue
                    </span>
                  </div>
                  <div className="col center" style={{ gap: 1 }}>
                    <span className="mono" style={{ fontWeight: 600, fontSize: 15, color: "var(--st-failed)" }}>
                      {fmt1(falha)}%
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
                    <span className="zg-kpi-value">
                      {sat.media > 0 ? fmt1(sat.media) : "—"}
                      <small>/5</small>
                    </span>
                    <div className="col" style={{ gap: 1, fontSize: 12, color: "var(--text-tertiary)" }}>
                      <span>{sat.respostas} respostas de {sat.enviadas} pesquisas ({sat.enviadas > 0 ? Math.round((sat.respostas / sat.enviadas) * 100) : 0}%)</span>
                    </div>
                  </div>
                </div>
              )}
              <div className="card card-pad col" style={{ gap: 12 }}>
                <div className="row between center">
                  <div className="card-title">Conexão com a Meta</div>
                  {conn &&
                    (conn === "ok" ? (
                      <span className="badge badge-read">
                        <Icon name="checkCircle" size={14} /> Conectado
                      </span>
                    ) : (
                      <span className="badge badge-failed">
                        <Icon name="alertCircle" size={14} /> Sem credenciais
                      </span>
                    ))}
                </div>
                <div style={{ fontSize: 13, color: "var(--text-secondary)" }}>
                  {conn === "err"
                    ? "Cadastre as credenciais em Configurações → WhatsApp / Meta para enviar e receber mensagens."
                    : "Meta Cloud API · WhatsApp Business. Acompanhe a entrega pelas taxas acima."}
                </div>
              </div>
            </div>
          </div>

          {/* Horário de pico (heatmap de mensagens recebidas, 30 dias) */}
          {heat.length > 0 && <HeatmapCard cells={heat} />}

          {/* Feed de atividade */}
          <div className="card">
            <div className="card-head">
              <div className="card-title">Atividade recente</div>
              <a
                className="row center"
                style={{ gap: 4, fontSize: 13, color: "var(--brand-text)", fontWeight: 600, cursor: "pointer" }}
                onClick={() => router.push("/historico")}
              >
                Ver campanhas <Icon name="chevronRight" size={14} />
              </a>
            </div>
            <div>
              {atividade.length === 0 && (
                <div style={{ padding: "26px 18px", textAlign: "center", color: "var(--text-tertiary)", fontSize: 13 }}>
                  {loading ? "Carregando…" : "Nenhuma atividade ainda"}
                </div>
              )}
              {atividade.map((a, i) => {
                const cfg =
                  a.tipo === "disparo"
                    ? { ic: "send" as IconName, c: "var(--brand-text)", b: "var(--brand-soft)" }
                    : a.tipo === "resposta"
                    ? { ic: "reply" as IconName, c: "var(--wa-deep)", b: "var(--wa-soft)" }
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
                    <span style={{ fontSize: 12, color: "var(--text-tertiary)", whiteSpace: "nowrap", minWidth: 64, textAlign: "right" }}>
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
                      <td className="num">{a.atendimentos.toLocaleString("pt-BR")}</td>
                      <td className="num">{a.mensagens.toLocaleString("pt-BR")}</td>
                      <td className="num">{a.tempo_medio_min > 0 ? a.tempo_medio_min.toFixed(0) + " min" : "—"}</td>
                      <td className="num">{a.satisfacao > 0 ? fmt1(a.satisfacao) + "/5" : "—"}</td>
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
