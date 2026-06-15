"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import {
  Avatar,
  Button,
  ConfirmModal,
  Drawer,
  Dropdown,
  EmptyState,
  Field,
  Icon,
  Input,
  Modal,
  Segmented,
  WaBubble,
  useToast,
} from "@/components/ui";
import { MessageItem } from "@/components/inbox/MessageItem";
import {
  listConversations,
  listConversationMessages,
  listSessions,
  listSessionMessages,
  sendMessage,
  addNote,
  reactMessage,
  listQuickReplies,
  getRoteamento,
  createQuickReply,
  deleteQuickReply,
  type ApiQuickReply,
  sendMedia,
  uploadMedia,
  sendTemplate,
  listContacts,
  listTemplates,
  claimConversation,
  transferConversation,
  resolveConversation,
  optOutConversation,
  optInConversation,
  resolveBulk,
  listDepartments,
  listLabels,
  setConversationLabels,
  getToken,
  hhmm,
  type ApiConversation,
  type ApiMessage,
  type ApiDepartment,
  type ApiSession,
  type ApiLabel,
  type ApiContact,
  type ApiTemplate,
} from "@/lib/api";
import { useAuth } from "@/lib/auth";
import { useNotifications } from "@/lib/notifications";
import { SETOR_COR } from "@/lib/data";

function nomeOuFone(c: ApiConversation) {
  return c.contact_name || c.contact_phone;
}

// pareceTelefone: "nome" que é só dígitos/máscara (atendente colou o número de
// novo no campo de nome) — não vale como nome de contato.
function pareceTelefone(s: string) {
  const t = s.trim();
  if (!t) return false;
  const digitos = t.replace(/\D/g, "").length;
  const naoDigitos = t.replace(/[\d\s()\-+.]/g, "").length;
  return digitos >= 8 && naoDigitos === 0;
}

// status da conversa → badge visual
const STATUS_BADGE: Record<string, { cls: string; label: string }> = {
  aguardando: { cls: "badge-pending", label: "Na fila" },
  em_atendimento: { cls: "badge-delivered", label: "Em atendimento" },
  resolvido: { cls: "badge-read", label: "Resolvido" },
};

// Inicia uma conversa 1:1: escolhe contato + template aprovado (a Meta exige
// template para iniciar fora da janela de 24h).
function NovaConversaModal({
  open,
  onClose,
  onSent,
}: {
  open: boolean;
  onClose: () => void;
  onSent: (phone: string) => void;
}) {
  const toast = useToast();
  const [contatos, setContatos] = useState<ApiContact[]>([]);
  const [aprovados, setAprovados] = useState<ApiTemplate[]>([]);
  const [q, setQ] = useState("");
  const [contato, setContato] = useState<ApiContact | null>(null);
  const [numero, setNumero] = useState(""); // número avulso (dígitos), sem contato
  const [nomeNovo, setNomeNovo] = useState(""); // nome opcional p/ auto-cadastro
  const [tpl, setTpl] = useState<ApiTemplate | null>(null);
  const [vars, setVars] = useState<Record<string, string>>({});
  const [sending, setSending] = useState(false);

  useEffect(() => {
    if (!open) return;
    setQ("");
    setContato(null);
    setNumero("");
    setNomeNovo("");
    setTpl(null);
    setVars({});
    listContacts().then(setContatos).catch(() => setContatos([]));
    listTemplates()
      .then((all) => {
        const ap = all.filter((t) => t.status === "aprovado");
        setAprovados(ap);
        setTpl(ap[0] ?? null);
      })
      .catch(() => setAprovados([]));
  }, [open]);

  // Ao trocar template/destinatário, reseta variáveis e auto-preenche a {{1}} com
  // o nome COMPLETO do contato (atendente pode editar se quiser só o 1º nome).
  useEffect(() => {
    const nome = contato?.nome || nomeNovo;
    setVars(nome ? { "1": nome.trim() } : {});
  }, [tpl, contato, nomeNovo]);

  const rows = contatos
    .filter((c) => !q || c.nome.toLowerCase().includes(q.toLowerCase()) || c.cpf.includes(q) || c.telefone.includes(q))
    .slice(0, 6);

  // Número avulso: se a busca tem só dígitos suficientes e não casa com contato.
  // Prefixa 55 (BR) quando faltar DDI, senão a Meta recusa.
  const normBR = (d: string) => (d.startsWith("55") || d.length > 11 ? d : d.length >= 10 ? "55" + d : d);
  const qDigits = normBR(q.replace(/\D/g, ""));
  const podeAvulso = q.replace(/\D/g, "").length >= 10 && rows.length === 0;
  // Destinatário final (telefone): contato OU número avulso.
  const destTel = contato?.telefone || numero;

  const nVars = tpl?.variables || 0;
  const corpo = tpl ? tpl.body.replace(/\{\{(\d+)\}\}/g, (m, n) => vars[n] || m) : "";
  const varsOk = Array.from({ length: nVars }, (_, i) => vars[String(i + 1)]?.trim()).every(Boolean);

  async function iniciar() {
    if (!destTel || !tpl || !varsOk) return;
    setSending(true);
    try {
      const params = Array.from({ length: nVars }, (_, i) => vars[String(i + 1)] || "");
      // O backend cadastra o contato e grava o nome COMPLETO na conversa.
      // Nome que é só o telefone repetido não vale como nome.
      const nomeValido = pareceTelefone(nomeNovo) ? "" : nomeNovo.trim();
      await sendTemplate({
        to: destTel,
        nome: contato?.nome || nomeValido,
        template: tpl.name,
        language: tpl.language,
        params,
        preview: corpo,
      });
      const quem = contato?.nome || nomeNovo.trim() || numero;
      toast({ type: "wa", title: "Conversa iniciada", message: "Template enviado a " + quem });
      onSent(destTel);
      onClose();
    } catch (e) {
      toast({ type: "error", title: "Falha ao iniciar", message: (e as Error).message });
    } finally {
      setSending(false);
    }
  }

  return (
    <Modal open={open} onClose={onClose} width={760}>
      <div className="modal-head">
        <div className="modal-title">Nova conversa</div>
        <div className="modal-sub">Inicie um atendimento individual com um cidadão.</div>
      </div>
      <div className="modal-body" style={{ display: "grid", gridTemplateColumns: "1fr 280px", gap: 20, maxHeight: "62vh", overflowY: "auto" }}>
        <div className="col" style={{ gap: 16 }}>
        <Field label="1 · Para quem?">
          <Input icon="search" placeholder="Buscar contato ou digitar um número (DDD + número)…" value={q} onChange={(e) => setQ(e.target.value)} autoFocus />
        </Field>
        {contato ? (
          <div className="row center between" style={{ padding: "10px 12px", borderRadius: 9, background: "var(--brand-soft)", border: "1px solid var(--brand)" }}>
            <div className="row center" style={{ gap: 10 }}>
              <Avatar name={contato.nome} size={32} />
              <div className="col" style={{ gap: 0 }}>
                <span className="strong" style={{ fontSize: 13.5 }}>{contato.nome}</span>
                <span className="mono faint" style={{ fontSize: 11.5 }}>{contato.telefone}</span>
              </div>
            </div>
            <button className="btn btn-ghost btn-icon btn-sm" onClick={() => setContato(null)}>
              <Icon name="x" size={15} />
            </button>
          </div>
        ) : numero ? (
          <div className="col" style={{ gap: 10, padding: "10px 12px", borderRadius: 9, background: "var(--brand-soft)", border: "1px solid var(--brand)" }}>
            <div className="row center between">
              <div className="row center" style={{ gap: 10 }}>
                <Avatar name={nomeNovo || "?"} size={32} />
                <div className="col" style={{ gap: 0 }}>
                  <span className="strong" style={{ fontSize: 13.5 }}>Número avulso</span>
                  <span className="mono faint" style={{ fontSize: 11.5 }}>{numero}</span>
                </div>
              </div>
              <button className="btn btn-ghost btn-icon btn-sm" onClick={() => { setNumero(""); setNomeNovo(""); }}>
                <Icon name="x" size={15} />
              </button>
            </div>
            <Input value={nomeNovo} onChange={(e) => setNomeNovo(e.target.value)} placeholder="Nome da pessoa (ex.: Maria Silva) — NÃO repita o número" />
            {pareceTelefone(nomeNovo) && (
              <span style={{ fontSize: 11.5, color: "var(--st-pending)" }}>
                ⚠ Isso parece um telefone — digite o NOME da pessoa aqui (o número já está acima).
              </span>
            )}
          </div>
        ) : (
          <div style={{ border: "1px solid var(--border-subtle)", borderRadius: 9, overflow: "hidden" }}>
            <div style={{ maxHeight: 150, overflowY: "auto" }}>
              {rows.length === 0 ? (
                <div className="muted" style={{ padding: 16, fontSize: 12.5, textAlign: "center" }}>
                  {podeAvulso ? "Nenhum contato — use o número avulso abaixo." : "Nenhum contato encontrado."}
                </div>
              ) : (
                rows.map((c) => (
                  <div key={c.id} className="recip-row" style={{ cursor: "pointer" }} onClick={() => setContato(c)}>
                    <Avatar name={c.nome} size={30} />
                    <div className="col grow" style={{ gap: 0, minWidth: 0 }}>
                      <span className="strong" style={{ fontSize: 13 }}>{c.nome}</span>
                      <span className="mono faint" style={{ fontSize: 11 }}>{c.telefone}</span>
                    </div>
                    {c.setor && (
                      <span className="tag">
                        <span className="tag-dot" style={{ background: SETOR_COR[c.setor] || "var(--text-faint)" }} /> {c.setor}
                      </span>
                    )}
                  </div>
                ))
              )}
            </div>
            {podeAvulso && (
              <div
                className="recip-row"
                style={{ cursor: "pointer", borderTop: "1px solid var(--border-subtle)", background: "var(--bg-sunken)" }}
                onClick={() => setNumero(qDigits)}
              >
                <span className="kpi-ico" style={{ width: 30, height: 30, background: "var(--wa-soft)", color: "var(--wa)" }}>
                  <Icon name="send" size={14} />
                </span>
                <div className="col grow" style={{ gap: 0, minWidth: 0 }}>
                  <span className="strong" style={{ fontSize: 13 }}>Usar número avulso</span>
                  <span className="mono faint" style={{ fontSize: 11 }}>{qDigits}</span>
                </div>
              </div>
            )}
          </div>
        )}

        <div className="divider" />
        <Field label="2 · Mensagem inicial (template)" hint="Fora da janela de 24h, a Meta exige um template aprovado para iniciar.">
          {aprovados.length === 0 ? (
            <div className="muted" style={{ fontSize: 12.5 }}>Nenhum template aprovado. Crie e aprove em Templates.</div>
          ) : (
            <select className="select" value={tpl?.name ?? ""} onChange={(e) => setTpl(aprovados.find((t) => t.name === e.target.value) ?? null)}>
              {/* agrupado por grupo de templates (secretaria) */}
              {Array.from(new Set(aprovados.map((t) => t.grupo || ""))).sort().map((g) =>
                g ? (
                  <optgroup key={g} label={g}>
                    {aprovados.filter((t) => (t.grupo || "") === g).map((t) => (
                      <option key={t.name + t.language} value={t.name}>{t.name}</option>
                    ))}
                  </optgroup>
                ) : (
                  aprovados.filter((t) => !t.grupo).map((t) => (
                    <option key={t.name + t.language} value={t.name}>{t.name}</option>
                  ))
                ),
              )}
            </select>
          )}
        </Field>

        {tpl && nVars > 0 && (
          <div className="col" style={{ gap: 10 }}>
            <span className="field-label">3 · Variáveis do template</span>
            {Array.from({ length: nVars }).map((_, i) => {
              const n = String(i + 1);
              return (
                <div key={n} className="row center" style={{ gap: 8 }}>
                  <span className="wa-var" style={{ flex: "none", minWidth: 34, textAlign: "center" }}>{"{{" + n + "}}"}</span>
                  <Input value={vars[n] || ""} onChange={(e) => setVars((v) => ({ ...v, [n]: e.target.value }))} placeholder={n === "1" ? "Nome do contato" : "Valor"} />
                  {n === "1" && contato && (
                    <Button variant="secondary" size="sm" onClick={() => setVars((v) => ({ ...v, "1": contato.nome.trim() }))}>
                      Auto
                    </Button>
                  )}
                </div>
              );
            })}
          </div>
        )}
        </div>

        {/* Preview na lateral (não empurra o botão de enviar) */}
        <div className="col" style={{ gap: 8 }}>
          <div className="field-label">Pré-visualização</div>
          {tpl ? (
            <div className="wa-canvas" style={{ flex: 1 }}>
              {tpl.header && (
                <div className="wa-row">
                  <div className="wa-bubble in" style={{ fontWeight: 700, fontSize: 12 }}>{tpl.header}</div>
                </div>
              )}
              <WaBubble text={corpo} dir="out" highlight status="enviado" time="agora" buttons={tpl.buttons?.length ? tpl.buttons : undefined} />
            </div>
          ) : (
            <div className="muted" style={{ fontSize: 12.5 }}>Escolha um template para ver a prévia.</div>
          )}
        </div>
      </div>
      <div className="modal-foot">
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="wa" icon="send" aria-disabled={!destTel || !tpl || !varsOk || sending} onClick={iniciar}>
          {sending ? "Iniciando…" : "Iniciar conversa"}
        </Button>
      </div>
    </Modal>
  );
}

function MidiaModal({
  open,
  onClose,
  phone,
  onSent,
}: {
  open: boolean;
  onClose: () => void;
  phone: string;
  onSent: () => void;
}) {
  const toast = useToast();
  const [file, setFile] = useState<File | null>(null);
  const [caption, setCaption] = useState("");
  const [sending, setSending] = useState(false);
  const [porLink, setPorLink] = useState(false);
  const [kind, setKind] = useState("image");
  const [link, setLink] = useState("");
  const [name, setName] = useState("");
  const fileRef = useRef<HTMLInputElement>(null);

  useEffect(() => {
    if (open) {
      setFile(null); setCaption(""); setPorLink(false);
      setKind("image"); setLink(""); setName("");
    }
  }, [open]);

  async function enviar() {
    setSending(true);
    try {
      if (porLink) {
        if (!link.trim()) return;
        await sendMedia({ to: phone, kind, link, caption, name });
      } else {
        if (!file) return;
        await uploadMedia(phone, file, caption);
      }
      toast({ type: "wa", title: "Mídia enviada" });
      onSent();
      onClose();
    } catch (e) {
      toast({ type: "error", title: "Falha ao enviar", message: (e as Error).message });
    } finally {
      setSending(false);
    }
  }

  const podeEnviar = porLink ? !!link.trim() : !!file;

  return (
    <Modal open={open} onClose={onClose} width={460}>
      <div className="modal-head">
        <div className="modal-title">Enviar arquivo</div>
        <div className="modal-sub">Imagem, documento, áudio ou vídeo.</div>
      </div>
      <div className="modal-body col" style={{ gap: 14 }}>
        {!porLink ? (
          <>
            <input ref={fileRef} type="file" hidden onChange={(e) => setFile(e.target.files?.[0] ?? null)} />
            <div
              onClick={() => fileRef.current?.click()}
              style={{
                border: "1.5px dashed var(--border)", borderRadius: 10, padding: "22px 16px",
                textAlign: "center", cursor: "pointer", background: "var(--bg-sunken)",
              }}
            >
              {file ? (
                <div className="col center" style={{ gap: 4 }}>
                  <Icon name="fileText" size={22} />
                  <span className="strong" style={{ fontSize: 13 }}>{file.name}</span>
                  <span className="faint" style={{ fontSize: 11 }}>{(file.size / 1024).toFixed(0)} KB · clique para trocar</span>
                </div>
              ) : (
                <div className="col center" style={{ gap: 4, color: "var(--text-tertiary)" }}>
                  <Icon name="attach" size={22} />
                  <span style={{ fontSize: 13 }}>Clique para escolher um arquivo</span>
                  <span className="faint" style={{ fontSize: 11 }}>até ~16 MB</span>
                </div>
              )}
            </div>
            <Field label="Legenda (opcional)">
              <Input value={caption} onChange={(e) => setCaption(e.target.value)} />
            </Field>
          </>
        ) : (
          <>
            <Field label="Tipo">
              <select className="select" value={kind} onChange={(e) => setKind(e.target.value)}>
                <option value="image">Imagem</option>
                <option value="document">Documento</option>
                <option value="video">Vídeo</option>
                <option value="audio">Áudio</option>
              </select>
            </Field>
            <Field label="URL do arquivo">
              <Input icon="link" value={link} onChange={(e) => setLink(e.target.value)} placeholder="https://…" />
            </Field>
            {kind === "document" && (
              <Field label="Nome do arquivo">
                <Input value={name} onChange={(e) => setName(e.target.value)} placeholder="documento.pdf" />
              </Field>
            )}
            {kind !== "audio" && (
              <Field label="Legenda (opcional)">
                <Input value={caption} onChange={(e) => setCaption(e.target.value)} />
              </Field>
            )}
          </>
        )}
        <a style={{ fontSize: 12, color: "var(--brand-text)", cursor: "pointer" }} onClick={() => setPorLink((v) => !v)}>
          {porLink ? "← Enviar arquivo do computador" : "ou enviar por link público"}
        </a>
      </div>
      <div className="modal-foot">
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="wa" icon="send" aria-disabled={sending || !podeEnviar} onClick={enviar}>
          {sending ? "Enviando…" : "Enviar"}
        </Button>
      </div>
    </Modal>
  );
}

// Gerência das respostas rápidas pessoais do atendente (globais são só leitura).
function MinhasRespostasModal({
  open,
  onClose,
  items,
  onChanged,
}: {
  open: boolean;
  onClose: () => void;
  items: ApiQuickReply[];
  onChanged: () => void;
}) {
  const toast = useToast();
  const [atalho, setAtalho] = useState("");
  const [texto, setTexto] = useState("");
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (open) { setAtalho(""); setTexto(""); }
  }, [open]);

  async function adicionar() {
    if (!texto.trim()) return;
    setSaving(true);
    try {
      await createQuickReply(atalho.trim(), texto.trim(), true); // pessoal (modal do chat)
      setAtalho(""); setTexto("");
      onChanged();
      toast({ type: "success", title: "Resposta criada" });
    } catch (e) {
      toast({ type: "error", title: "Erro", message: (e as Error).message });
    } finally { setSaving(false); }
  }

  async function remover(id: number) {
    try {
      await deleteQuickReply(id);
      onChanged();
    } catch (e) {
      toast({ type: "error", title: "Erro", message: (e as Error).message });
    }
  }

  const minhas = items.filter((q) => q.minha);
  const globais = items.filter((q) => !q.minha);

  return (
    <Modal open={open} onClose={onClose} width={520}>
      <div className="modal-head">
        <div className="modal-title">Minhas respostas rápidas</div>
        <div className="modal-sub">Crie atalhos pessoais — só você vê os seus.</div>
      </div>
      <div className="modal-body col" style={{ gap: 14, maxHeight: "62vh", overflowY: "auto" }}>
        <div className="col" style={{ gap: 10, padding: 12, border: "1px solid var(--border-subtle)", borderRadius: 9, background: "var(--bg-sunken)" }}>
          <Field label="Atalho (opcional)" hint="Ex.: /2via">
            <Input value={atalho} onChange={(e) => setAtalho(e.target.value)} placeholder="/saudacao" />
          </Field>
          <Field label="Texto">
            <textarea className="input" rows={3} value={texto} onChange={(e) => setTexto(e.target.value)} placeholder="Olá! Como posso ajudar?" style={{ resize: "vertical", lineHeight: 1.5 }} />
          </Field>
          <div className="row" style={{ justifyContent: "flex-end" }}>
            <Button variant="primary" size="sm" icon="plus" aria-disabled={saving || !texto.trim()} onClick={adicionar}>
              {saving ? "Salvando…" : "Adicionar"}
            </Button>
          </div>
        </div>

        <div className="col" style={{ gap: 6 }}>
          <span className="field-label">Minhas ({minhas.length})</span>
          {minhas.length === 0 ? (
            <span className="muted" style={{ fontSize: 12.5 }}>Nenhuma resposta pessoal ainda.</span>
          ) : (
            minhas.map((q) => (
              <div key={q.id} className="row center between" style={{ padding: "8px 10px", border: "1px solid var(--border-subtle)", borderRadius: 8, fontSize: 12.5 }}>
                <div className="col grow" style={{ gap: 1, minWidth: 0 }}>
                  {q.atalho && <span className="strong mono" style={{ fontSize: 11.5 }}>{q.atalho}</span>}
                  <span className="faint" style={{ fontSize: 11.5 }}>{q.texto}</span>
                </div>
                <button className="btn btn-ghost btn-icon btn-sm" title="Excluir" onClick={() => remover(q.id)}>
                  <Icon name="trash" size={13} />
                </button>
              </div>
            ))
          )}
        </div>

        {globais.length > 0 && (
          <div className="col" style={{ gap: 6 }}>
            <span className="field-label">Do órgão ({globais.length}) — somente leitura</span>
            {globais.map((q) => (
              <div key={q.id} className="row center between" style={{ padding: "8px 10px", border: "1px solid var(--border-subtle)", borderRadius: 8, fontSize: 12.5, opacity: 0.8 }}>
                <div className="col grow" style={{ gap: 1, minWidth: 0 }}>
                  {q.atalho && <span className="strong mono" style={{ fontSize: 11.5 }}>{q.atalho}</span>}
                  <span className="faint" style={{ fontSize: 11.5 }}>{q.texto}</span>
                </div>
              </div>
            ))}
          </div>
        )}
      </div>
      <div className="modal-foot">
        <Button variant="primary" className="btn-block" onClick={onClose}>Fechar</Button>
      </div>
    </Modal>
  );
}

export default function InboxPage() {
  const toast = useToast();
  const { user } = useAuth();
  const { reset: resetNotif } = useNotifications();
  const [convs, setConvs] = useState<ApiConversation[]>([]);
  const [activeId, setActiveId] = useState<number | null>(null);
  const [msgs, setMsgs] = useState<ApiMessage[]>([]);
  const [filter, setFilter] = useState(""); // '' | aguardando | em_atendimento
  const [setorFiltro, setSetorFiltro] = useState("Todos");
  const [atendenteFiltro, setAtendenteFiltro] = useState("Todos");
  const [deps, setDeps] = useState<ApiDepartment[]>([]);
  const [labels, setLabels] = useState<ApiLabel[]>([]);
  const [bulkConfirm, setBulkConfirm] = useState(false);
  const [q, setQ] = useState("");
  const [draft, setDraft] = useState("");
  const [sending, setSending] = useState(false);
  const [notaMode, setNotaMode] = useState(false);
  const [replyTo, setReplyTo] = useState<ApiMessage | null>(null); // mensagem citada (responder)
  const [quickReplies, setQuickReplies] = useState<ApiQuickReply[]>([]);
  const [qrOpen, setQrOpen] = useState(false); // modal "minhas respostas rápidas"
  const [slashOff, setSlashOff] = useState(false); // autocomplete / dispensado (Esc/selecionou)
  const [slaMin, setSlaMin] = useState(15); // SLA da fila (minutos)
  // Gravação de áudio (MediaRecorder).
  const [gravando, setGravando] = useState(false);
  const [recSeg, setRecSeg] = useState(0);
  const recRef = useRef<MediaRecorder | null>(null);
  const recChunks = useRef<Blob[]>([]);
  const recCancel = useRef(false);
  const recTimer = useRef<ReturnType<typeof setInterval> | null>(null);
  const [finBusca, setFinBusca] = useState<ApiConversation[]>([]); // busca server-side em Finalizados
  const [novaOpen, setNovaOpen] = useState(false);
  const [midiaOpen, setMidiaOpen] = useState(false);
  const [erro, setErro] = useState<string | null>(null);
  const scrollRef = useRef<HTMLDivElement>(null);

  // Histórico por sessão (drawer)
  const [histOpen, setHistOpen] = useState(false);
  const [sessions, setSessions] = useState<ApiSession[]>([]);
  const [sessSel, setSessSel] = useState<ApiSession | null>(null);
  const [sessMsgs, setSessMsgs] = useState<ApiMessage[]>([]);

  async function abrirHistorico() {
    if (!active) return;
    setSessSel(null);
    setHistOpen(true);
    try {
      setSessions(await listSessions(active.id));
    } catch {
      setSessions([]);
    }
  }
  async function abrirSessao(s: ApiSession) {
    setSessSel(s);
    try {
      setSessMsgs(await listSessionMessages(s.id));
    } catch {
      setSessMsgs([]);
    }
  }

  // Atendente só vê fila + em atendimento; "Todas" é de admin/supervisor.
  const canTodas = user?.papel === "admin" || user?.papel === "supervisor";

  useEffect(() => {
    if (!canTodas) setFilter((f) => (f === "" ? "aguardando" : f));
  }, [canTodas]);

  const active = convs.find((c) => c.id === activeId) || null;

  const loadConvs = useCallback(async () => {
    try {
      const data = await listConversations();
      setConvs(data);
      setErro(null);
      setActiveId((prev) => prev ?? (data.length ? data[0].id : null));
    } catch (e) {
      setErro((e as Error).message);
    }
  }, []);

  const loadMsgs = useCallback(async (id: number) => {
    try {
      setMsgs(await listConversationMessages(id));
    } catch {
      /* silencioso no polling */
    }
  }, []);

  // Ref do id ativo p/ o handler SSE não reconectar a cada troca de conversa.
  const activeIdRef = useRef<number | null>(null);
  activeIdRef.current = activeId;

  // Carga inicial + polling lento (fallback do SSE).
  useEffect(() => {
    loadConvs();
    listDepartments().then(setDeps).catch(() => {});
    listLabels().then(setLabels).catch(() => {});
    listQuickReplies().then(setQuickReplies).catch(() => {});
    getRoteamento().then((r) => setSlaMin(r.sla_minutos || 15)).catch(() => {});
    const t = setInterval(loadConvs, 20000);
    return () => clearInterval(t);
  }, [loadConvs]);

  async function toggleLabel(labelId: number) {
    if (!active) return;
    const atuais = (active.labels ?? []).map((l) => l.id);
    const next = atuais.includes(labelId) ? atuais.filter((id) => id !== labelId) : [...atuais, labelId];
    try {
      await setConversationLabels(active.id, next);
      await loadConvs();
    } catch (e) {
      toast({ type: "error", title: "Erro", message: (e as Error).message });
    }
  }

  // Tempo real via SSE: nova mensagem / status → recarrega na hora.
  useEffect(() => {
    const token = getToken();
    if (!token) return;
    const base = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
    const es = new EventSource(`${base}/events?token=${encodeURIComponent(token)}`);
    es.onmessage = () => {
      loadConvs();
      resetNotif(); // está olhando o inbox → zera o contador
      const id = activeIdRef.current;
      if (id != null) loadMsgs(id);
    };
    es.onerror = () => {}; // EventSource reconecta sozinho
    return () => es.close();
  }, [loadConvs, loadMsgs, resetNotif]);

  // Zera o contador ao abrir o inbox.
  useEffect(() => {
    resetNotif();
  }, [resetNotif]);

  async function pegar() {
    if (!active) return;
    try {
      await claimConversation(active.id, user?.nome ?? "Atendente");
      await loadConvs();
      toast({ type: "success", title: "Conversa atribuída a você" });
    } catch (e) {
      toast({ type: "error", title: "Erro", message: (e as Error).message });
    }
  }

  // ---- Gravação de áudio (envia como mensagem de voz) ----
  async function iniciarGravacao() {
    if (!active) return;
    try {
      const stream = await navigator.mediaDevices.getUserMedia({ audio: true });
      // Preferência: ogg/opus (Firefox, aceito direto) > mp4 (Safari) > webm (Chrome; backend converte).
      const cand = ["audio/ogg;codecs=opus", "audio/mp4", "audio/webm;codecs=opus", "audio/webm"];
      const mime = cand.find((m) => MediaRecorder.isTypeSupported(m)) || "";
      const rec = new MediaRecorder(stream, mime ? { mimeType: mime } : undefined);
      recChunks.current = [];
      recCancel.current = false;
      rec.ondataavailable = (e) => e.data.size > 0 && recChunks.current.push(e.data);
      rec.onstop = async () => {
        stream.getTracks().forEach((t) => t.stop());
        if (recTimer.current) clearInterval(recTimer.current);
        setGravando(false);
        if (recCancel.current || recChunks.current.length === 0 || !active) return;
        const tipo = rec.mimeType || "audio/webm";
        const ext = tipo.includes("ogg") ? "ogg" : tipo.includes("mp4") ? "m4a" : "webm";
        const blob = new Blob(recChunks.current, { type: tipo.split(";")[0] });
        const file = new File([blob], `audio.${ext}`, { type: tipo.split(";")[0] });
        setSending(true);
        try {
          await uploadMedia(active.contact_phone, file);
          await loadMsgs(active.id);
          loadConvs();
        } catch (e) {
          toast({ type: "error", title: "Falha ao enviar áudio", message: (e as Error).message });
        } finally {
          setSending(false);
        }
      };
      recRef.current = rec;
      rec.start();
      setRecSeg(0);
      setGravando(true);
      recTimer.current = setInterval(() => setRecSeg((s) => s + 1), 1000);
    } catch {
      toast({ type: "error", title: "Microfone indisponível", message: "Permita o acesso ao microfone no navegador." });
    }
  }
  function pararGravacao(cancelar: boolean) {
    recCancel.current = cancelar;
    recRef.current?.stop();
  }

  // Autocomplete "/atalho": digitou / no início → filtra respostas rápidas.
  const slashMatches = (() => {
    if (slashOff || notaMode || !draft.startsWith("/")) return [] as ApiQuickReply[];
    const termo = draft.toLowerCase();
    return quickReplies
      .filter((qr) =>
        (qr.atalho && qr.atalho.toLowerCase().startsWith(termo)) ||
        (termo.length > 2 && qr.texto.toLowerCase().includes(termo.slice(1))),
      )
      .slice(0, 6);
  })();

  async function reagir(waId: string, emoji: string) {
    if (!active) return;
    try {
      await reactMessage(active.id, waId, emoji);
      await loadMsgs(active.id);
    } catch (e) {
      toast({ type: "error", title: "Falha ao reagir", message: (e as Error).message });
    }
  }

  async function transferir(setor: string) {
    if (!active) return;
    try {
      await transferConversation(active.id, setor);
      await loadConvs();
      toast({ type: "success", title: "Conversa transferida", message: "Setor: " + setor });
    } catch (e) {
      toast({ type: "error", title: "Erro", message: (e as Error).message });
    }
  }

  async function resolver() {
    if (!active) return;
    try {
      await resolveConversation(active.id);
      await loadConvs();
      toast({ type: "success", title: "Atendimento resolvido" });
    } catch (e) {
      toast({ type: "error", title: "Erro", message: (e as Error).message });
    }
  }

  async function toggleOptOut() {
    if (!active) return;
    try {
      if (active.opt_out) {
        await optInConversation(active.id);
        toast({ type: "success", title: "Contato recadastrado" });
      } else {
        await optOutConversation(active.id);
        toast({ type: "info", title: "Contato descadastrado (opt-out)" });
      }
      await loadConvs();
    } catch (e) {
      toast({ type: "error", title: "Erro", message: (e as Error).message });
    }
  }

  // Mensagens ao selecionar a conversa (+ fallback lento; o SSE cobre o tempo real).
  useEffect(() => {
    if (activeId == null) return;
    setReplyTo(null); // trocar de conversa descarta a citação pendente
    loadMsgs(activeId);
    const t = setInterval(() => loadMsgs(activeId), 15000);
    return () => clearInterval(t);
  }, [activeId, loadMsgs]);

  useEffect(() => {
    if (scrollRef.current) scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
  }, [msgs]);

  const atendentes = Array.from(new Set(convs.map((c) => c.atendente).filter(Boolean)));

  const filtered = convs.filter(
    (c) =>
      // "Todas" mostra só as ativas; finalizadas ficam na aba Finalizados.
      (filter === "" ? c.status !== "resolvido" : c.status === filter) &&
      (setorFiltro === "Todos" || c.setor === setorFiltro) &&
      (atendenteFiltro === "Todos" || c.atendente === atendenteFiltro) &&
      (!q || nomeOuFone(c).toLowerCase().includes(q.toLowerCase()) || c.contact_phone.includes(q)),
  );

  // Conversas ativas no filtro atual (podem ser finalizadas em massa).
  const ativasFiltradas = filtered.filter((c) => c.status !== "resolvido");

  // Server-side: busca com 2+ caracteres (qualquer aba) — nome, telefone, CPF
  // e conteúdo das mensagens — E a aba Finalizados SEMPRE (senão ela mostraria
  // só as resolvidas dentro das ~100 conversas recentes carregadas).
  const buscandoServer = q.trim().length >= 2 || filter === "resolvido";
  useEffect(() => {
    if (!buscandoServer) { setFinBusca([]); return; }
    const t = setTimeout(() => {
      // 'ativas' = todas menos resolvidas (aba "Todas"); senão o status da aba.
      const status = filter === "" ? "ativas" : filter;
      listConversations({ status, q: q.trim().length >= 2 ? q.trim() : "", setor: setorFiltro, limit: 200 })
        .then(setFinBusca)
        .catch(() => setFinBusca([]));
    }, 350);
    return () => clearTimeout(t);
  }, [buscandoServer, q, filter, setorFiltro]);

  // Lista exibida: busca server-side quando há termo, senão o filtro local.
  const listaBase = buscandoServer
    ? finBusca.filter((c) => atendenteFiltro === "Todos" || c.atendente === atendenteFiltro)
    : filtered;
  // Aba Fila: ordena pela espera (mais antiga primeiro) — SLA primeiro a estourar no topo.
  const lista = filter === "aguardando"
    ? [...listaBase].sort((a, b) => new Date(a.queued_at || a.last_msg_at).getTime() - new Date(b.queued_at || b.last_msg_at).getTime())
    : listaBase;

  // SLA: minutos de espera + cor (verde <50%, amarelo até 100%, vermelho estourado).
  const esperaMin = (c: ApiConversation) =>
    c.queued_at ? Math.max(0, Math.floor((Date.now() - new Date(c.queued_at).getTime()) / 60000)) : 0;
  const slaCor = (min: number) =>
    min >= slaMin ? "var(--st-failed)" : min >= slaMin / 2 ? "var(--st-pending)" : "var(--st-read)";
  const filaAguardando = convs.filter((c) => c.status === "aguardando");
  const maisAntigaMin = filaAguardando.length
    ? Math.max(...filaAguardando.map(esperaMin))
    : 0;

  async function finalizarEmMassa() {
    setBulkConfirm(false);
    try {
      const n = await resolveBulk(ativasFiltradas.map((c) => c.id));
      toast({ type: "success", title: `${n} conversas finalizadas` });
      await loadConvs();
    } catch (e) {
      toast({ type: "error", title: "Erro", message: (e as Error).message });
    }
  }

  async function enviar() {
    if (!draft.trim() || !active) return;
    const texto = draft;
    setDraft("");
    setSending(true);
    try {
      if (notaMode) {
        await addNote(active.id, texto);
      } else {
        await sendMessage(active.contact_phone, texto, replyTo?.wa_message_id);
        setReplyTo(null);
      }
      await loadMsgs(active.id);
      loadConvs();
    } catch (e) {
      toast({ type: "error", title: notaMode ? "Falha ao salvar nota" : "Falha no envio", message: (e as Error).message });
      setDraft(texto); // devolve o rascunho
    } finally {
      setSending(false);
    }
  }

  return (
    <div className="split">
      {/* Lista de conversas */}
      <div className="split-list">
        <div style={{ padding: "14px 16px 10px", borderBottom: "1px solid var(--border-subtle)" }}>
          <div className="row between center" style={{ marginBottom: 12 }}>
            <h1 style={{ fontSize: 17, fontWeight: 700 }}>Caixa de Entrada</h1>
            <Button variant="wa" size="sm" icon="plus" onClick={() => setNovaOpen(true)}>
              Nova conversa
            </Button>
          </div>
          <div className="row center" style={{ gap: 8, marginBottom: 12 }}>
            <span className="badge badge-pending">{filaAguardando.length} na fila</span>
            {filaAguardando.length > 0 && (
              <span className="badge" style={{ background: "transparent", border: "1px solid var(--border-subtle)", color: slaCor(maisAntigaMin) }}>
                ⏱ mais antiga há {maisAntigaMin} min
              </span>
            )}
            <span className="muted" style={{ fontSize: 11.5 }}>
              {convs.filter((c) => c.status === "aguardando" || c.status === "em_atendimento").length} abertas
            </span>
          </div>
          <div className="row" style={{ gap: 8 }}>
            <div className="grow">
              <Input icon="search" placeholder="Buscar nome, telefone, CPF ou mensagem…" value={q} onChange={(e) => setQ(e.target.value)} />
            </div>
            <select className="select" style={{ width: "auto" }} value={setorFiltro} onChange={(e) => setSetorFiltro(e.target.value)}>
              <option>Todos</option>
              {deps.map((d) => (
                <option key={d.id}>{d.nome}</option>
              ))}
            </select>
          </div>
          <div className="row between center" style={{ gap: 6, marginTop: 10 }}>
            <Segmented
              value={filter}
              onChange={setFilter}
              options={[
                ...(canTodas ? [{ value: "", label: "Todas" }] : []),
                { value: "aguardando", label: "Fila" },
                { value: "em_atendimento", label: "Atendendo" },
                { value: "resolvido", label: "Finalizados" },
              ]}
            />
            {atendentes.length > 0 && (
              <select className="select" style={{ width: "auto" }} value={atendenteFiltro} onChange={(e) => setAtendenteFiltro(e.target.value)}>
                <option>Todos</option>
                {atendentes.map((a) => (
                  <option key={a}>{a}</option>
                ))}
              </select>
            )}
          </div>
          {canTodas && ativasFiltradas.length > 0 && (
            <Button variant="secondary" size="sm" icon="check" className="btn-block" style={{ marginTop: 8 }} onClick={() => setBulkConfirm(true)}>
              Finalizar todas ({ativasFiltradas.length})
            </Button>
          )}
        </div>
        <div className="scroll-y grow">
          {erro ? (
            <EmptyState icon="alertCircle" title="API indisponível" sub={erro} />
          ) : lista.length === 0 ? (
            <EmptyState icon="message" title={buscandoServer ? "Nada encontrado" : "Nenhuma conversa"} sub={buscandoServer ? "Nada com esse nome, telefone, CPF ou conteúdo." : "Envie ou receba uma mensagem para começar."} />
          ) : (
            lista.map((c) => (
              <div
                key={c.id}
                className={"conv-item" + (c.id === activeId ? " active" : "")}
                onClick={() => {
                  setActiveId(c.id);
                  // Zera o contador na hora (otimista); o backend marca como lida ao carregar.
                  if (c.unread_count > 0) {
                    setConvs((cs) => cs.map((x) => (x.id === c.id ? { ...x, unread_count: 0 } : x)));
                  }
                }}
              >
                <Avatar name={nomeOuFone(c)} size={42} />
                <div className="col grow" style={{ gap: 3, minWidth: 0 }}>
                  <div className="row between center">
                    <span className="strong" style={{ fontSize: 13.5, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
                      {nomeOuFone(c)}
                    </span>
                    <span style={{ fontSize: 11, color: c.unread_count > 0 ? "var(--wa)" : "var(--text-faint)", fontWeight: c.unread_count > 0 ? 700 : 400, flex: "none" }}>
                      {hhmm(c.last_msg_at)}
                    </span>
                  </div>
                  <div className="row between center" style={{ gap: 8 }}>
                    <span style={{ fontSize: 12.5, color: c.unread_count > 0 ? "var(--text-primary)" : "var(--text-tertiary)", fontWeight: c.unread_count > 0 ? 600 : 400, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
                      {c.last_message || "—"}
                    </span>
                    {c.unread_count > 0 && (
                      <span style={{ background: "var(--wa-deep)", color: "white", fontSize: 10.5, fontWeight: 700, minWidth: 18, height: 18, borderRadius: 99, display: "grid", placeItems: "center", padding: "0 5px", flex: "none" }}>
                        {c.unread_count}
                      </span>
                    )}
                  </div>
                  <div className="row center wrap" style={{ gap: 6 }}>
                    {c.status && STATUS_BADGE[c.status] && (
                      <span className={"badge " + STATUS_BADGE[c.status].cls} style={{ height: 18, fontSize: 10.5 }}>
                        {STATUS_BADGE[c.status].label}
                      </span>
                    )}
                    {c.status === "aguardando" && c.queued_at && (
                      <span
                        className="badge"
                        style={{ height: 18, fontSize: 10.5, background: "transparent", border: "1px solid var(--border-subtle)", color: slaCor(esperaMin(c)), fontWeight: 700 }}
                        title={`Aguardando há ${esperaMin(c)} min (SLA: ${slaMin} min)`}
                      >
                        ⏱ {esperaMin(c)}m
                      </span>
                    )}
                    {(c.labels ?? []).map((l) => (
                      <span
                        key={l.id}
                        title={l.nome}
                        style={{ display: "inline-flex", alignItems: "center", gap: 4, height: 17, padding: "0 6px", borderRadius: 99, fontSize: 10, fontWeight: 600, color: l.cor, background: "color-mix(in oklch, " + l.cor + " 16%, transparent)" }}
                      >
                        <span style={{ width: 6, height: 6, borderRadius: 99, background: l.cor }} />
                        {l.nome}
                      </span>
                    ))}
                    {c.setor && (
                      <span className="mono faint" style={{ fontSize: 10.5 }}>
                        {c.setor}
                      </span>
                    )}
                  </div>
                </div>
              </div>
            ))
          )}
        </div>
      </div>

      {/* Detalhe da conversa */}
      {active ? (
        <div className="split-detail">
          <div className="row between center" style={{ padding: "12px 20px", borderBottom: "1px solid var(--border-subtle)", flex: "none" }}>
            <div className="row center" style={{ gap: 12 }}>
              <Avatar name={nomeOuFone(active)} size={38} />
              <div className="col" style={{ gap: 1 }}>
                <span style={{ fontWeight: 700, fontSize: 14.5 }}>{nomeOuFone(active)}</span>
                <span className="mono" style={{ fontSize: 11.5, color: "var(--text-tertiary)" }}>
                  {active.contact_phone}
                  {active.window_expires_at && new Date(active.window_expires_at) > new Date() ? (
                    <span style={{ color: "var(--wa)" }}> · janela aberta</span>
                  ) : (
                    <span style={{ color: "var(--st-failed)" }}> · janela fechada (use template)</span>
                  )}
                </span>
              </div>
            </div>
            <div className="row center" style={{ gap: 8 }}>
              {active.status === "aguardando" && (
                <Button variant="primary" size="sm" icon="reply" onClick={pegar}>
                  Pegar conversa
                </Button>
              )}
              {active.status === "em_atendimento" && (
                <>
                  <span className="muted" style={{ fontSize: 12 }}>
                    Atendente: <span className="strong">{active.atendente || "—"}</span>
                  </span>
                  {active.atendente !== (user?.nome ?? "") && (
                    <Button variant="primary" size="sm" icon="reply" onClick={pegar} title="Puxar a conversa para você">
                      Assumir
                    </Button>
                  )}
                  <Button variant="secondary" size="sm" icon="check" onClick={resolver}>
                    Resolver
                  </Button>
                </>
              )}
              {active.status === "resolvido" && <span className="badge badge-read">Resolvido</span>}
              {active.opt_out && (
                <span className="badge badge-failed">
                  <Icon name="ban" size={11} /> Opt-out
                </span>
              )}
              <Button variant="ghost" size="sm" icon="history" onClick={abrirHistorico}>
                Histórico
              </Button>
              <Dropdown
                trigger={
                  <button className="btn btn-ghost btn-icon btn-sm" title="Etiquetas">
                    <Icon name="tag" size={16} />
                  </button>
                }
              >
                <div className="dropdown-label">Etiquetas</div>
                {labels.length === 0 ? (
                  <div className="dropdown-item muted">Nenhuma etiqueta</div>
                ) : (
                  labels.map((l) => {
                    const on = (active.labels ?? []).some((x) => x.id === l.id);
                    return (
                      <div key={l.id} className="dropdown-item" onClick={() => toggleLabel(l.id)}>
                        <span style={{ width: 10, height: 10, borderRadius: 3, background: l.cor, flex: "none" }} />
                        <span className="grow">{l.nome}</span>
                        {on && <Icon name="check" size={14} />}
                      </div>
                    );
                  })
                )}
              </Dropdown>
              <Dropdown
                trigger={
                  <button className="btn btn-ghost btn-icon btn-sm">
                    <Icon name="more" size={16} />
                  </button>
                }
              >
                <div className="dropdown-item" onClick={() => loadMsgs(active.id)}>
                  <Icon name="refresh" size={15} /> Atualizar
                </div>
                {deps.length > 0 && active.status !== "resolvido" && (
                  <>
                    <div className="dropdown-sep" />
                    <div className="dropdown-label">Transferir para</div>
                    {deps
                      .filter((d) => d.nome !== active.setor)
                      .map((d) => (
                        <div key={d.id} className="dropdown-item" onClick={() => transferir(d.nome)}>
                          <span style={{ width: 10, height: 10, borderRadius: 3, background: d.cor, flex: "none" }} />
                          {d.nome}
                        </div>
                      ))}
                  </>
                )}
                {active.status !== "resolvido" && (
                  <>
                    <div className="dropdown-sep" />
                    <div className="dropdown-item" onClick={resolver}>
                      <Icon name="check" size={15} /> Finalizar atendimento
                    </div>
                  </>
                )}
                <div className="dropdown-sep" />
                <div className={"dropdown-item" + (active.opt_out ? "" : " danger")} onClick={toggleOptOut}>
                  <Icon name={active.opt_out ? "check" : "ban"} size={15} />
                  {active.opt_out ? "Recadastrar contato" : "Descadastrar (opt-out)"}
                </div>
              </Dropdown>
            </div>
          </div>

          {/* Dados coletados pelo bot + setor encaminhado (entregue ao atendente) */}
          {(active.setor || (active.dados && Object.keys(active.dados).length > 0)) && (
            <div style={{ padding: "10px 20px", borderBottom: "1px solid var(--border-subtle)", background: "var(--bg-sunken)", flex: "none" }}>
              <div className="row center" style={{ gap: 8, marginBottom: active.dados && Object.keys(active.dados).length > 0 ? 8 : 0 }}>
                <span className="field-label">Atendimento</span>
                {active.setor && (
                  <span className="badge badge-brand">
                    <Icon name="building" size={11} /> {active.setor}
                  </span>
                )}
              </div>
              {active.dados && Object.keys(active.dados).length > 0 && (
                <div className="row wrap" style={{ gap: 14 }}>
                  {Object.entries(active.dados).map(([k, v]) => (
                    <div key={k} className="col" style={{ gap: 1 }}>
                      <span className="faint" style={{ fontSize: 10.5, textTransform: "uppercase", letterSpacing: "0.04em" }}>
                        {k}
                      </span>
                      <span className="strong" style={{ fontSize: 12.5 }}>
                        {v}
                      </span>
                    </div>
                  ))}
                </div>
              )}
            </div>
          )}

          <div ref={scrollRef} className="wa-canvas scroll-y grow" style={{ borderRadius: 0, margin: 0 }}>
            {msgs.length === 0 ? (
              <div className="muted" style={{ textAlign: "center", padding: 24, fontSize: 12.5 }}>
                Sem mensagens ainda.
              </div>
            ) : (
              msgs.map((m) => <MessageItem key={m.id} m={m} onReact={reagir} onReply={setReplyTo} />)
            )}
          </div>

          <div style={{ padding: "12px 16px", borderTop: "1px solid var(--border-subtle)", flex: "none" }}>
            {active.opt_out ? (
              <div className="row center" style={{ gap: 8, padding: "10px 12px", borderRadius: 8, background: "var(--st-failed-soft)", color: "var(--st-failed)", fontSize: 12.5, fontWeight: 600 }}>
                <Icon name="ban" size={15} /> Contato descadastrou (opt-out) — não é possível enviar mensagens. Recadastre pelo menu se ele autorizar.
              </div>
            ) : (
              <>
                {replyTo && !notaMode && (
                  <div className="row center" style={{ gap: 10, marginBottom: 8, padding: "7px 10px", borderRadius: 9, background: "var(--bg-sunken)", border: "1px solid var(--border-subtle)" }}>
                    <div className="col grow" style={{ gap: 1, borderLeft: "3px solid var(--wa)", paddingLeft: 9, minWidth: 0 }}>
                      <span style={{ fontSize: 10.5, fontWeight: 700, color: "var(--wa)" }}>
                        Respondendo a {replyTo.direction === "in" ? nomeOuFone(active) : "você"}
                      </span>
                      <span className="faint" style={{ fontSize: 12, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
                        {replyTo.body || (replyTo.media_type ? "[" + replyTo.media_type + "]" : "")}
                      </span>
                    </div>
                    <button className="btn btn-ghost btn-icon btn-sm" title="Cancelar resposta" onClick={() => setReplyTo(null)}>
                      <Icon name="x" size={14} />
                    </button>
                  </div>
                )}
                {gravando ? (
                  <div className="row center" style={{ gap: 12, height: 42 }}>
                    <span style={{ width: 10, height: 10, borderRadius: 99, background: "var(--st-failed)", animation: "pulse 1s infinite" }} />
                    <span className="mono strong" style={{ fontSize: 14 }}>
                      {String(Math.floor(recSeg / 60)).padStart(2, "0")}:{String(recSeg % 60).padStart(2, "0")}
                    </span>
                    <span className="muted grow" style={{ fontSize: 12.5 }}>Gravando áudio…</span>
                    <Button variant="ghost" size="sm" onClick={() => pararGravacao(true)}>Cancelar</Button>
                    <Button variant="wa" style={{ height: 42, width: 42, padding: 0 }} onClick={() => pararGravacao(false)} title="Enviar áudio">
                      <Icon name="send" size={18} />
                    </Button>
                  </div>
                ) : (
                <div className="row center" style={{ gap: 9 }}>
                  <button className="btn btn-ghost btn-icon" style={{ height: 42, width: 42 }} title="Enviar mídia" onClick={() => setMidiaOpen(true)}>
                    <Icon name="attach" size={18} />
                  </button>
                  <Dropdown
                    up
                    trigger={
                      <button className="btn btn-ghost btn-icon" style={{ height: 42, width: 42 }} title="Respostas rápidas">
                        <Icon name="zap" size={18} />
                      </button>
                    }
                  >
                    <div className="dropdown-label">Respostas rápidas</div>
                    <div style={{ maxHeight: 260, overflowY: "auto" }}>
                      {quickReplies.length === 0 && (
                        <div className="dropdown-item muted">Nenhuma ainda — crie a sua ↓</div>
                      )}
                      {quickReplies.map((qr) => (
                        <div key={qr.id} className="dropdown-item" style={{ alignItems: "flex-start" }} onClick={() => setDraft((d) => (d ? d + " " : "") + qr.texto)}>
                          <div className="col" style={{ gap: 1, minWidth: 0 }}>
                            <span className="row center" style={{ gap: 6 }}>
                              {qr.atalho && <span className="strong" style={{ fontSize: 12 }}>{qr.atalho}</span>}
                              {qr.minha && <span className="badge badge-neutral" style={{ fontSize: 9 }}>minha</span>}
                            </span>
                            <span className="faint" style={{ fontSize: 11.5, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis", maxWidth: 220 }}>{qr.texto}</span>
                          </div>
                        </div>
                      ))}
                    </div>
                    <div className="dropdown-sep" />
                    <div className="dropdown-item" onClick={() => setQrOpen(true)}>
                      <Icon name="settings" size={14} /> Gerenciar minhas respostas
                    </div>
                  </Dropdown>
                  <button
                    className={"btn btn-icon" + (notaMode ? "" : " btn-ghost")}
                    style={{ height: 42, width: 42, background: notaMode ? "var(--st-pending-soft)" : undefined, color: notaMode ? "var(--st-pending)" : undefined }}
                    title="Nota interna (só a equipe vê)"
                    onClick={() => setNotaMode((v) => !v)}
                  >
                    <Icon name="edit" size={18} />
                  </button>
                  <div className="grow" style={{ position: "relative" }}>
                    {/* Autocomplete de respostas rápidas: digite / + atalho */}
                    {slashMatches.length > 0 && (
                      <div
                        style={{
                          position: "absolute", bottom: "calc(100% + 6px)", left: 0, right: 0, zIndex: 30,
                          background: "var(--bg-elevated, var(--bg-surface))", border: "1px solid var(--border)",
                          borderRadius: 10, boxShadow: "0 8px 28px rgba(0,0,0,.25)", overflow: "hidden", maxHeight: 240, overflowY: "auto",
                        }}
                      >
                        {slashMatches.map((qr, i) => (
                          <div
                            key={qr.id}
                            onClick={() => { setDraft(qr.texto); setSlashOff(true); }}
                            style={{
                              padding: "9px 12px", cursor: "pointer", fontSize: 12.5,
                              background: i === 0 ? "var(--brand-soft)" : "transparent",
                              borderBottom: "1px solid var(--border-subtle)",
                            }}
                          >
                            <div className="row center" style={{ gap: 7 }}>
                              {qr.atalho && <span className="strong mono" style={{ fontSize: 11.5 }}>{qr.atalho}</span>}
                              {qr.minha && <span className="badge badge-neutral" style={{ fontSize: 9 }}>minha</span>}
                              {i === 0 && <span className="faint" style={{ fontSize: 10, marginLeft: "auto" }}>Tab ou Enter ↵</span>}
                            </div>
                            <div className="faint" style={{ fontSize: 11.5, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>{qr.texto}</div>
                          </div>
                        ))}
                      </div>
                    )}
                    <input
                      className="input"
                      style={{ width: "100%", height: 42, background: notaMode ? "var(--st-pending-soft)" : undefined, borderColor: notaMode ? "var(--st-pending)" : undefined }}
                      placeholder={notaMode ? "Nota interna (não vai pro cidadão)…" : "Digite uma resposta… ( / = atalhos)"}
                      value={draft}
                      onChange={(e) => { setDraft(e.target.value); setSlashOff(false); }}
                      onKeyDown={(e) => {
                        if (slashMatches.length > 0 && (e.key === "Enter" || e.key === "Tab")) {
                          e.preventDefault();
                          setDraft(slashMatches[0].texto);
                          setSlashOff(true);
                          return;
                        }
                        if (e.key === "Escape") { setSlashOff(true); return; }
                        if (e.key === "Enter") enviar();
                      }}
                    />
                  </div>
                  {!notaMode && !draft.trim() && (
                    <button className="btn btn-ghost btn-icon" style={{ height: 42, width: 42 }} title="Gravar áudio" onClick={iniciarGravacao}>
                      <Icon name="mic" size={18} />
                    </button>
                  )}
                  <Button variant={notaMode ? "secondary" : "wa"} style={{ height: 42, width: 42, padding: 0 }} aria-disabled={sending} onClick={enviar}>
                    <Icon name={notaMode ? "edit" : "send"} size={18} />
                  </Button>
                </div>
                )}
                <div className="row center" style={{ gap: 6, marginTop: 8, fontSize: 11, color: notaMode ? "var(--st-pending)" : "var(--text-faint)" }}>
                  <Icon name="info" size={12} />
                  {notaMode ? "Modo nota: salva só pra equipe, não envia ao cidadão." : "Resposta livre só dentro da janela de 24h (regra da Meta)."}
                </div>
              </>
            )}
          </div>
        </div>
      ) : (
        <div className="split-detail">
          <EmptyState icon="message" title="Selecione uma conversa" sub="Escolha um contato na lista para ver o histórico e responder." />
        </div>
      )}

      <MinhasRespostasModal
        open={qrOpen}
        onClose={() => setQrOpen(false)}
        items={quickReplies}
        onChanged={() => listQuickReplies().then(setQuickReplies).catch(() => {})}
      />

      <NovaConversaModal
        open={novaOpen}
        onClose={() => setNovaOpen(false)}
        onSent={async () => {
          const data = await listConversations();
          setConvs(data);
          if (data.length) setActiveId(data[0].id); // a recém-criada é a mais recente
        }}
      />

      {active && (
        <MidiaModal
          open={midiaOpen}
          onClose={() => setMidiaOpen(false)}
          phone={active.contact_phone}
          onSent={() => loadMsgs(active.id)}
        />
      )}

      <ConfirmModal
        open={bulkConfirm}
        onClose={() => setBulkConfirm(false)}
        onConfirm={finalizarEmMassa}
        icon="check"
        title={`Finalizar ${ativasFiltradas.length} conversas?`}
        message="Todas as conversas ativas do filtro atual serão marcadas como resolvidas. Esta ação não pode ser desfeita."
        confirmLabel="Finalizar todas"
      />

      {/* Histórico por sessão de atendimento */}
      <Drawer open={histOpen} onClose={() => setHistOpen(false)} width={440}>
        <div className="row between center" style={{ padding: "16px 20px", borderBottom: "1px solid var(--border-subtle)" }}>
          <div className="row center" style={{ gap: 8 }}>
            {sessSel && (
              <button className="btn btn-ghost btn-icon btn-sm" onClick={() => setSessSel(null)}>
                <Icon name="arrowLeft" size={16} />
              </button>
            )}
            <div className="card-title">{sessSel ? "Atendimento" : "Histórico de atendimentos"}</div>
          </div>
          <button className="btn btn-ghost btn-icon btn-sm" onClick={() => setHistOpen(false)}>
            <Icon name="x" size={16} />
          </button>
        </div>

        {!sessSel ? (
          <div className="scroll-y grow">
            {sessions.length === 0 ? (
              <EmptyState icon="history" title="Sem sessões" sub="Os atendimentos aparecem aqui quando a conversa avança." />
            ) : (
              sessions.map((s) => (
                <div
                  key={s.id}
                  onClick={() => abrirSessao(s)}
                  style={{ padding: "13px 18px", borderBottom: "1px solid var(--border-subtle)", cursor: "pointer" }}
                >
                  <div className="row between center" style={{ marginBottom: 4 }}>
                    <span className="strong" style={{ fontSize: 13 }}>
                      {new Date(s.iniciada_em).toLocaleString("pt-BR", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" })}
                    </span>
                    <span className={"badge " + (s.status === "resolvida" ? "badge-read" : "badge-pending")}>
                      {s.status === "resolvida" ? "Resolvida" : "Aberta"}
                    </span>
                  </div>
                  <div className="row center" style={{ gap: 8, fontSize: 11.5, color: "var(--text-tertiary)" }}>
                    {s.setor && (
                      <span className="tag">
                        <span className="tag-dot" style={{ background: SETOR_COR[s.setor] || "var(--text-faint)" }} /> {s.setor}
                      </span>
                    )}
                    <span>{s.mensagens} msgs</span>
                    {s.atendente && <span>· {s.atendente}</span>}
                  </div>
                </div>
              ))
            )}
          </div>
        ) : (
          <div className="scroll-y grow" style={{ padding: 16 }}>
            {sessSel.dados && Object.keys(sessSel.dados).length > 0 && (
              <div className="card card-pad" style={{ marginBottom: 12 }}>
                {Object.entries(sessSel.dados).map(([k, v]) => (
                  <div key={k} className="row between" style={{ fontSize: 12.5, padding: "2px 0" }}>
                    <span className="muted">{k}</span>
                    <span className="strong">{v}</span>
                  </div>
                ))}
              </div>
            )}
            <div className="wa-canvas">
              {sessMsgs.map((m) => (
                <MessageItem key={m.id} m={m} />
              ))}
            </div>
          </div>
        )}
      </Drawer>
    </div>
  );
}
