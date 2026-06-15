"use client";

import { useCallback, useEffect, useState } from "react";
import { Topbar } from "@/components/shell/Topbar";
import {
  Avatar, Button, Drawer, Field, Icon, Input, Modal, useToast,
} from "@/components/ui";
import { ConversaHistorico } from "@/components/inbox/ConversaHistorico";
import {
  listTickets, getTicketMetrics, getTicket, createTicket, setTicketStatus,
  assignTicket, commentTicket,
  type ApiTicket, type ApiTicketComment, type ApiTicketMetrics,
} from "@/lib/api";
import { useAuth } from "@/lib/auth";

// Colunas do Kanban (status ativos; cancelado fica fora da visão principal).
const COLUNAS: { status: ApiTicket["status"]; label: string }[] = [
  { status: "aberto", label: "Aberto" },
  { status: "em_andamento", label: "Em andamento" },
  { status: "aguardando", label: "Aguardando" },
  { status: "resolvido", label: "Resolvido" },
];

const PRIO: Record<ApiTicket["prioridade"], { label: string; cor: string }> = {
  critica: { label: "Crítica", cor: "var(--st-failed)" },
  alta: { label: "Alta", cor: "var(--st-pending)" },
  media: { label: "Média", cor: "var(--brand-text)" },
  baixa: { label: "Baixa", cor: "var(--text-faint)" },
};

const CATEGORIAS = ["Hardware", "Rede", "Sistema", "Impressora", "E-mail", "Outros"];

function slaInfo(t: ApiTicket): { txt: string; cor: string } | null {
  if (!t.sla_venc || t.status === "resolvido" || t.status === "cancelado") return null;
  const min = Math.floor((new Date(t.sla_venc).getTime() - Date.now()) / 60000);
  if (min < 0) return { txt: `SLA -${fmtMin(-min)}`, cor: "var(--st-failed)" };
  if (min < 120) return { txt: fmtMin(min), cor: "var(--st-pending)" };
  return { txt: fmtMin(min), cor: "var(--st-read)" };
}
function fmtMin(min: number): string {
  if (min < 60) return min + "m";
  const h = Math.floor(min / 60);
  if (h < 24) return h + "h";
  return Math.floor(h / 24) + "d";
}
function fmtData(iso: string): string {
  return new Date(iso).toLocaleString("pt-BR", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" });
}

export default function ChamadosPage() {
  const toast = useToast();
  const [tickets, setTickets] = useState<ApiTicket[]>([]);
  const [metrics, setMetrics] = useState<ApiTicketMetrics | null>(null);
  const [prioFiltro, setPrioFiltro] = useState("");
  const [novoOpen, setNovoOpen] = useState(false);
  const [abertoId, setAbertoId] = useState<number | null>(null);

  const load = useCallback(async () => {
    try {
      const [ts, m] = await Promise.all([
        listTickets(prioFiltro ? { prioridade: prioFiltro } : undefined),
        getTicketMetrics().catch(() => null),
      ]);
      setTickets(ts);
      setMetrics(m);
    } catch (e) {
      toast({ type: "error", title: "Erro ao carregar", message: (e as Error).message });
    }
  }, [prioFiltro, toast]);

  useEffect(() => {
    load();
    const t = setInterval(load, 15000);
    return () => clearInterval(t);
  }, [load]);

  const kpis: [string, number, string][] = metrics
    ? [
        ["Abertos", metrics.abertos, "var(--st-pending)"],
        ["Em andamento", metrics.em_andamento, "var(--brand-text)"],
        ["SLA estourado", metrics.sla_estourado, metrics.sla_estourado > 0 ? "var(--st-failed)" : "var(--st-read)"],
        ["Resolvidos (7d)", metrics.resolvidos_7d, "var(--st-read)"],
      ]
    : [];

  return (
    <>
      <Topbar
        title="Chamados — T.I."
        sub="Abertura, fila e acompanhamento de chamados de suporte"
        actions={
          <>
            <select className="select" style={{ width: "auto" }} value={prioFiltro} onChange={(e) => setPrioFiltro(e.target.value)}>
              <option value="">Todas prioridades</option>
              {Object.entries(PRIO).map(([k, v]) => (
                <option key={k} value={k}>{v.label}</option>
              ))}
            </select>
            <Button variant="secondary" icon="refresh" onClick={load}>Atualizar</Button>
            <Button variant="primary" icon="plus" onClick={() => setNovoOpen(true)}>Novo chamado</Button>
          </>
        }
      />
      <div className="content">
        <div className="content-pad col" style={{ gap: 16 }}>
          {/* KPIs */}
          <div className="kpi-grid">
            {kpis.map(([label, valor, cor]) => (
              <div key={label} className="card kpi" style={{ gap: 6 }}>
                <span className="kpi-label">{label}</span>
                <span className="kpi-value" style={{ fontSize: 30, color: cor }}>{valor}</span>
              </div>
            ))}
          </div>

          {/* Kanban */}
          <div style={{ display: "grid", gridTemplateColumns: "repeat(4, 1fr)", gap: 14, alignItems: "start" }}>
            {COLUNAS.map((col) => {
              const itens = tickets.filter((t) => t.status === col.status);
              return (
                <div key={col.status} className="card" style={{ minHeight: 200 }}>
                  <div className="card-head" style={{ padding: "12px 14px" }}>
                    <div className="card-title" style={{ fontSize: 13.5 }}>
                      {col.label} <span className="faint">({itens.length})</span>
                    </div>
                  </div>
                  <div className="col" style={{ gap: 8, padding: 10 }}>
                    {itens.length === 0 && <div className="muted" style={{ fontSize: 12, textAlign: "center", padding: 14 }}>—</div>}
                    {itens.map((t) => {
                      const sla = slaInfo(t);
                      return (
                        <div
                          key={t.id}
                          className="card card-pad"
                          style={{ cursor: "pointer", padding: 12, borderLeft: `3px solid ${PRIO[t.prioridade].cor}` }}
                          onClick={() => setAbertoId(t.id)}
                        >
                          <div className="row between center" style={{ marginBottom: 5 }}>
                            <span className="mono faint" style={{ fontSize: 11 }}>#{t.numero}</span>
                            {sla && (
                              <span className="mono" style={{ fontSize: 10.5, fontWeight: 700, color: sla.cor }}>⏱ {sla.txt}</span>
                            )}
                          </div>
                          <div className="strong" style={{ fontSize: 13, lineHeight: 1.35, marginBottom: 6 }}>{t.titulo}</div>
                          <div className="row center wrap" style={{ gap: 6 }}>
                            {t.categoria && <span className="badge badge-neutral" style={{ fontSize: 10 }}>{t.categoria}</span>}
                            {t.secretaria && <span className="mono faint" style={{ fontSize: 10 }}>{t.secretaria}</span>}
                          </div>
                          {t.tecnico && (
                            <div className="row center" style={{ gap: 6, marginTop: 7 }}>
                              <Avatar name={t.tecnico} size={18} />
                              <span style={{ fontSize: 11 }}>{t.tecnico.split(" ")[0]}</span>
                            </div>
                          )}
                        </div>
                      );
                    })}
                  </div>
                </div>
              );
            })}
          </div>
        </div>
      </div>

      <NovoChamadoModal open={novoOpen} onClose={() => setNovoOpen(false)} onCriado={() => { setNovoOpen(false); load(); }} />
      {abertoId != null && (
        <DetalheDrawer id={abertoId} onClose={() => setAbertoId(null)} onMudou={load} />
      )}
    </>
  );
}

function NovoChamadoModal({ open, onClose, onCriado }: { open: boolean; onClose: () => void; onCriado: () => void }) {
  const toast = useToast();
  const [titulo, setTitulo] = useState("");
  const [descricao, setDescricao] = useState("");
  const [categoria, setCategoria] = useState(CATEGORIAS[0]);
  const [prioridade, setPrioridade] = useState("media");
  const [nome, setNome] = useState("");
  const [fone, setFone] = useState("");
  const [secretaria, setSecretaria] = useState("");
  const [local, setLocal] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (open) { setTitulo(""); setDescricao(""); setCategoria(CATEGORIAS[0]); setPrioridade("media"); setNome(""); setFone(""); setSecretaria(""); setLocal(""); }
  }, [open]);

  async function salvar() {
    if (!titulo.trim()) return;
    setSaving(true);
    try {
      await createTicket({ titulo, descricao, categoria, prioridade, solicitante_nome: nome, solicitante_fone: fone, secretaria, local });
      toast({ type: "success", title: "Chamado aberto" });
      onCriado();
    } catch (e) {
      toast({ type: "error", title: "Erro", message: (e as Error).message });
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal open={open} onClose={onClose} width={560}>
      <div className="modal-head">
        <div className="modal-title">Novo chamado</div>
        <div className="modal-sub">Abra um chamado de suporte do T.I.</div>
      </div>
      <div className="modal-body col" style={{ gap: 14, maxHeight: "64vh", overflowY: "auto" }}>
        <Field label="Título *">
          <Input value={titulo} onChange={(e) => setTitulo(e.target.value)} placeholder="Resumo do problema" autoFocus />
        </Field>
        <Field label="Descrição">
          <textarea className="input" rows={3} value={descricao} onChange={(e) => setDescricao(e.target.value)} placeholder="Detalhes, mensagens de erro…" />
        </Field>
        <div className="row" style={{ gap: 12 }}>
          <Field label="Categoria">
            <select className="select" value={categoria} onChange={(e) => setCategoria(e.target.value)}>
              {CATEGORIAS.map((c) => <option key={c}>{c}</option>)}
            </select>
          </Field>
          <Field label="Prioridade">
            <select className="select" value={prioridade} onChange={(e) => setPrioridade(e.target.value)}>
              <option value="baixa">Baixa</option>
              <option value="media">Média</option>
              <option value="alta">Alta</option>
              <option value="critica">Crítica</option>
            </select>
          </Field>
        </div>
        <div className="row" style={{ gap: 12 }}>
          <Field label="Solicitante">
            <Input value={nome} onChange={(e) => setNome(e.target.value)} placeholder="Nome" />
          </Field>
          <Field label="WhatsApp (p/ avisar)">
            <Input value={fone} onChange={(e) => setFone(e.target.value)} placeholder="46 99999-9999" />
          </Field>
        </div>
        <div className="row" style={{ gap: 12 }}>
          <Field label="Secretaria">
            <Input value={secretaria} onChange={(e) => setSecretaria(e.target.value)} placeholder="Ex.: Saúde" />
          </Field>
          <Field label="Local">
            <Input value={local} onChange={(e) => setLocal(e.target.value)} placeholder="Sala / setor" />
          </Field>
        </div>
      </div>
      <div className="modal-foot">
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="primary" aria-disabled={saving || !titulo.trim()} onClick={salvar}>
          {saving ? "Abrindo…" : "Abrir chamado"}
        </Button>
      </div>
    </Modal>
  );
}

function DetalheDrawer({ id, onClose, onMudou }: { id: number; onClose: () => void; onMudou: () => void }) {
  const toast = useToast();
  const { user } = useAuth();
  const [ticket, setTicket] = useState<ApiTicket | null>(null);
  const [comments, setComments] = useState<ApiTicketComment[]>([]);
  const [coment, setComent] = useState("");
  const [interno, setInterno] = useState(false);
  const [busy, setBusy] = useState(false);
  const [verConv, setVerConv] = useState(false);

  const carregar = useCallback(async () => {
    try {
      const r = await getTicket(id);
      setTicket(r.ticket);
      setComments(r.comments ?? []);
    } catch { /* ignore */ }
  }, [id]);

  useEffect(() => { carregar(); }, [carregar]);

  async function mudarStatus(status: string) {
    setBusy(true);
    try {
      await setTicketStatus(id, status);
      await carregar(); onMudou();
      toast({ type: "success", title: "Status atualizado" });
    } catch (e) {
      toast({ type: "error", title: "Erro", message: (e as Error).message });
    } finally { setBusy(false); }
  }
  async function assumir() {
    setBusy(true);
    try {
      await assignTicket(id, user?.nome);
      await carregar(); onMudou();
      toast({ type: "success", title: "Chamado atribuído a você" });
    } catch (e) {
      toast({ type: "error", title: "Erro", message: (e as Error).message });
    } finally { setBusy(false); }
  }
  async function comentar() {
    if (!coment.trim()) return;
    setBusy(true);
    try {
      await commentTicket(id, coment, interno);
      setComent("");
      await carregar();
    } catch (e) {
      toast({ type: "error", title: "Erro", message: (e as Error).message });
    } finally { setBusy(false); }
  }

  if (!ticket) return <Drawer open onClose={onClose}><div style={{ padding: 24 }} className="muted">Carregando…</div></Drawer>;
  const sla = slaInfo(ticket);

  return (
    <Drawer open onClose={onClose} width={460}>
      <div className="row between center" style={{ marginBottom: 14 }}>
        <div className="col" style={{ gap: 1, minWidth: 0 }}>
          <span className="mono faint" style={{ fontSize: 11 }}>#{ticket.numero}</span>
          <span className="strong" style={{ fontSize: 15, lineHeight: 1.3 }}>{ticket.titulo}</span>
        </div>
        <button className="btn btn-ghost btn-icon btn-sm" onClick={onClose}><Icon name="x" size={16} /></button>
      </div>
      <div className="col" style={{ gap: 16, padding: 4, overflowY: "auto", maxHeight: "calc(100vh - 120px)" }}>
        <div className="row center wrap" style={{ gap: 8 }}>
          <span className="badge" style={{ background: "transparent", border: `1px solid ${PRIO[ticket.prioridade].cor}`, color: PRIO[ticket.prioridade].cor }}>
            {PRIO[ticket.prioridade].label}
          </span>
          {ticket.categoria && <span className="badge badge-neutral">{ticket.categoria}</span>}
          {sla && <span className="badge" style={{ background: "transparent", border: "1px solid var(--border-subtle)", color: sla.cor }}>⏱ {sla.txt}</span>}
        </div>

        {ticket.descricao && (
          <div style={{ fontSize: 13, lineHeight: 1.55, whiteSpace: "pre-wrap" }}>{ticket.descricao}</div>
        )}

        <div className="card card-pad col" style={{ gap: 7, fontSize: 12.5 }}>
          {ticket.solicitante_nome && <Linha k="Solicitante" v={ticket.solicitante_nome} />}
          {ticket.solicitante_fone && <Linha k="WhatsApp" v={ticket.solicitante_fone} />}
          {ticket.secretaria && <Linha k="Secretaria" v={ticket.secretaria} />}
          {ticket.local && <Linha k="Local" v={ticket.local} />}
          <Linha k="Técnico" v={ticket.tecnico || "—"} />
          <Linha k="Aberto em" v={fmtData(ticket.criado_em)} />
        </div>

        {/* Conversa de origem (chamado aberto pelo WhatsApp) */}
        {ticket.conversation_id && (
          <Button variant="secondary" size="sm" icon="message" onClick={() => setVerConv(true)}>
            Ver conversa de origem (WhatsApp)
          </Button>
        )}

        {/* Ações */}
        <div className="row center wrap" style={{ gap: 8 }}>
          {!ticket.tecnico && <Button size="sm" variant="primary" icon="reply" aria-disabled={busy} onClick={assumir}>Assumir</Button>}
          {ticket.status !== "em_andamento" && ticket.status !== "resolvido" && ticket.status !== "cancelado" && (
            <Button size="sm" variant="secondary" aria-disabled={busy} onClick={() => mudarStatus("em_andamento")}>Em andamento</Button>
          )}
          {(ticket.status === "em_andamento" || ticket.status === "aguardando") && (
            <Button size="sm" variant="secondary" aria-disabled={busy} onClick={() => mudarStatus("aguardando")}>Aguardando</Button>
          )}
          {ticket.status !== "resolvido" && ticket.status !== "cancelado" && (
            <Button size="sm" variant="wa" icon="check" aria-disabled={busy} onClick={() => mudarStatus("resolvido")}>Resolver</Button>
          )}
          {ticket.status === "resolvido" && (
            <Button size="sm" variant="secondary" aria-disabled={busy} onClick={() => mudarStatus("em_andamento")}>Reabrir</Button>
          )}
        </div>

        {/* Timeline */}
        <div className="col" style={{ gap: 10 }}>
          <div className="field-label">Andamentos</div>
          {comments.length === 0 && <div className="muted" style={{ fontSize: 12 }}>Sem andamentos ainda.</div>}
          {comments.map((c) => (
            <div key={c.id} className="col" style={{ gap: 2, padding: "8px 10px", borderRadius: 8, background: c.interno ? "var(--st-pending-soft)" : "var(--bg-sunken)" }}>
              <div className="row between center">
                <span className="strong" style={{ fontSize: 11.5 }}>{c.autor}{c.interno && " · interno"}</span>
                <span className="faint" style={{ fontSize: 10.5 }}>{fmtData(c.criado_em)}</span>
              </div>
              <div style={{ fontSize: 12.5, whiteSpace: "pre-wrap" }}>{c.body}</div>
            </div>
          ))}
        </div>

        {/* Novo comentário */}
        <div className="col" style={{ gap: 8 }}>
          <textarea className="input" rows={2} value={coment} onChange={(e) => setComent(e.target.value)} placeholder="Adicionar andamento…" />
          <div className="row between center">
            <label className="row center" style={{ gap: 6, fontSize: 12, cursor: "pointer" }}>
              <input type="checkbox" className="checkbox" checked={interno} onChange={(e) => setInterno(e.target.checked)} /> Só técnicos
            </label>
            <Button size="sm" variant="primary" aria-disabled={busy || !coment.trim()} onClick={comentar}>Comentar</Button>
          </div>
        </div>
      </div>
      {verConv && ticket.conversation_id && (
        <ConversaHistorico convId={ticket.conversation_id} onClose={() => setVerConv(false)} />
      )}
    </Drawer>
  );
}

function Linha({ k, v }: { k: string; v: string }) {
  return (
    <div className="row between center">
      <span className="faint">{k}</span>
      <span className="strong">{v}</span>
    </div>
  );
}
