"use client";

import { Fragment, useCallback, useEffect, useRef, useState } from "react";
import {
  Alert,
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
import { MessageItem, DateDivider, mesmoDia } from "@/components/inbox/MessageItem";
import { Topbar } from "@/components/shell/Topbar";
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
  presetPhone = "",
}: {
  open: boolean;
  presetPhone?: string;
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
    setQ(presetPhone);
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
  }, [open, presetPhone]);

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
                <span className="strong" style={{ fontSize: 14 }}>{contato.nome}</span>
                <span className="mono faint" style={{ fontSize: 12 }}>{contato.telefone}</span>
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
                  <span className="strong" style={{ fontSize: 14 }}>Número avulso</span>
                  <span className="mono faint" style={{ fontSize: 12 }}>{numero}</span>
                </div>
              </div>
              <button className="btn btn-ghost btn-icon btn-sm" onClick={() => { setNumero(""); setNomeNovo(""); }}>
                <Icon name="x" size={15} />
              </button>
            </div>
            <Input value={nomeNovo} onChange={(e) => setNomeNovo(e.target.value)} placeholder="Nome da pessoa (ex.: Maria Silva) — NÃO repita o número" />
            {pareceTelefone(nomeNovo) && (
              <span style={{ fontSize: 12, color: "var(--st-pending)" }}>
                ⚠ Isso parece um telefone — digite o NOME da pessoa aqui (o número já está acima).
              </span>
            )}
          </div>
        ) : (
          <div style={{ border: "1px solid var(--border-subtle)", borderRadius: 9, overflow: "hidden" }}>
            <div style={{ maxHeight: 150, overflowY: "auto" }}>
              {rows.length === 0 ? (
                <div className="muted" style={{ padding: 16, fontSize: 13, textAlign: "center" }}>
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
                <span className="kpi-ico" style={{ width: 30, height: 30, background: "var(--wa-soft)", color: "var(--wa-deep)" }}>
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
            <div className="muted" style={{ fontSize: 13 }}>Nenhum template aprovado. Crie e aprove em Templates.</div>
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
            <div className="muted" style={{ fontSize: 13 }}>Escolha um template para ver a prévia.</div>
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
      toast({ type: "error", title: "Não foi possível concluir", message: (e as Error).message });
    } finally { setSaving(false); }
  }

  async function remover(id: number) {
    try {
      await deleteQuickReply(id);
      onChanged();
    } catch (e) {
      toast({ type: "error", title: "Não foi possível concluir", message: (e as Error).message });
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
            <span className="muted" style={{ fontSize: 13 }}>Nenhuma resposta pessoal ainda.</span>
          ) : (
            minhas.map((q) => (
              <div key={q.id} className="row center between" style={{ padding: "8px 10px", border: "1px solid var(--border-subtle)", borderRadius: 8, fontSize: 13 }}>
                <div className="col grow" style={{ gap: 1, minWidth: 0 }}>
                  {q.atalho && <span className="strong mono" style={{ fontSize: 12 }}>{q.atalho}</span>}
                  <span className="faint" style={{ fontSize: 12 }}>{q.texto}</span>
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
              <div key={q.id} className="row center between" style={{ padding: "8px 10px", border: "1px solid var(--border-subtle)", borderRadius: 8, fontSize: 13, opacity: 0.8 }}>
                <div className="col grow" style={{ gap: 1, minWidth: 0 }}>
                  {q.atalho && <span className="strong mono" style={{ fontSize: 12 }}>{q.atalho}</span>}
                  <span className="faint" style={{ fontSize: 12 }}>{q.texto}</span>
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
  const { reset: resetNotif, lastEvent } = useNotifications();
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
  const stickRef = useRef(true); // auto-scroll só quando o usuário está no fim do chat
  const [colisao, setColisao] = useState<string | null>(null); // outro atendente na conversa
  const taRef = useRef<HTMLTextAreaElement>(null); // campo de mensagem (auto-resize)
  const [novaPhone, setNovaPhone] = useState(""); // template para a conversa aberta
  // Painel do contato: fixo em ≥1440px, Drawer abaixo disso.
  const [largo, setLargo] = useState(true);
  const [painel, setPainel] = useState(true);
  const [painelSess, setPainelSess] = useState<ApiSession[]>([]);

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

  // Procura na lista principal E nos resultados de busca/Finalizados (server-side):
  // conversas fora das ~100 recentes só existem em finBusca — sem isso o detalhe
  // não abria ao clicar numa conversa vinda da busca.
  const active = convs.find((c) => c.id === activeId) || finBusca.find((c) => c.id === activeId) || null;

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

  // marcarLida=false (atualização em segundo plano sem foco) preserva o contador
  // de não-lidas — o atendente não perde a mensagem por deixar o chat aberto.
  const loadMsgs = useCallback(async (id: number, marcarLida = true) => {
    try {
      setMsgs(await listConversationMessages(id, marcarLida));
    } catch {
      /* silencioso no polling */
    }
  }, []);

  // Ref do id ativo p/ o handler SSE não reconectar a cada troca de conversa.
  const activeIdRef = useRef<number | null>(null);
  activeIdRef.current = activeId;

  // Deep-link: /inbox?conv=ID abre direto a conversa.
  useEffect(() => {
    const p = new URLSearchParams(window.location.search).get("conv");
    if (p) setActiveId(Number(p));
  }, []);

  // Tempo real: recarrega na hora ao receber um evento SSE (mensagem recebida,
  // mudança de conversa/status), sem esperar o polling. Se o evento é da conversa
  // aberta, recarrega o chat também.
  useEffect(() => {
    if (!lastEvent) return;
    loadConvs();
    const aid = activeIdRef.current;
    if (aid && (!lastEvent.conversation_id || lastEvent.conversation_id === aid)) {
      // Janela sem foco → atualiza o chat MAS não marca como lida (mantém o aviso).
      loadMsgs(aid, typeof document !== "undefined" && document.hasFocus());
    }
    // eslint-disable-next-line react-hooks/exhaustive-deps
  }, [lastEvent]);

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
      toast({ type: "error", title: "Não foi possível concluir", message: (e as Error).message });
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

  useEffect(() => {
    const mq = window.matchMedia("(min-width: 1440px)");
    const on = () => {
      setLargo(mq.matches);
      setPainel(mq.matches);
    };
    on();
    mq.addEventListener("change", on);
    return () => mq.removeEventListener("change", on);
  }, []);

  // Atendimentos anteriores do contato (painel lateral).
  useEffect(() => {
    setPainelSess([]);
    if (activeId == null || !painel) return;
    listSessions(activeId).then(setPainelSess).catch(() => {});
  }, [activeId, painel]);

  async function pegar() {
    if (!active) return;
    try {
      await claimConversation(active.id, user?.nome ?? "Atendente");
      await loadMsgs(active.id); // mostra o evento "assumiu o atendimento"
      await loadConvs();
      toast({ type: "success", title: "Conversa atribuída a você" });
    } catch (e) {
      toast({ type: "error", title: "Não foi possível concluir", message: (e as Error).message });
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
      await loadMsgs(active.id); // mostra o evento "transferiu para o setor"
      await loadConvs();
      toast({ type: "success", title: "Conversa transferida", message: "Setor: " + setor });
    } catch (e) {
      toast({ type: "error", title: "Não foi possível concluir", message: (e as Error).message });
    }
  }

  async function resolver() {
    if (!active) return;
    try {
      await resolveConversation(active.id);
      await loadMsgs(active.id); // mostra o evento "encerrou o atendimento"
      await loadConvs();
      toast({ type: "success", title: "Atendimento resolvido" });
    } catch (e) {
      toast({ type: "error", title: "Não foi possível concluir", message: (e as Error).message });
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
      toast({ type: "error", title: "Não foi possível concluir", message: (e as Error).message });
    }
  }

  // Mensagens ao selecionar a conversa (+ fallback lento; o SSE cobre o tempo real).
  useEffect(() => {
    if (activeId == null) return;
    setReplyTo(null); // trocar de conversa descarta a citação pendente
    stickRef.current = true; // ao abrir a conversa, começa no fim
    loadMsgs(activeId);
    const t = setInterval(() => loadMsgs(activeId), 15000);
    return () => clearInterval(t);
  }, [activeId, loadMsgs]);

  useEffect(() => {
    // Só rola pro fim se o usuário já estava lá — senão respeita a leitura do histórico.
    if (stickRef.current && scrollRef.current) scrollRef.current.scrollTop = scrollRef.current.scrollHeight;
  }, [msgs]);

  // Ajusta a altura do campo de mensagem ao conteúdo (cresce até um limite, depois rola).
  useEffect(() => {
    const ta = taRef.current;
    if (!ta) return;
    ta.style.height = "auto";
    ta.style.height = Math.min(ta.scrollHeight, 140) + "px";
  }, [draft]);

  // Inclui atendentes das finalizadas (finBusca) — senão quem só tem conversa
  // resolvida não apareceria no select e não daria pra filtrar na aba Finalizados.
  const atendentes = Array.from(
    new Set([...convs, ...finBusca].map((c) => c.atendente).filter(Boolean)),
  ).sort((a, b) => a.localeCompare(b, "pt-BR"));

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
      const temTermo = q.trim().length >= 2;
      // Aba "Todas": ao pesquisar traz TUDO (inclui finalizadas); sem termo, só ativas.
      // Nas demais abas, sempre o status da aba.
      const status = filter === "" ? (temTermo ? "" : "ativas") : filter;
      listConversations({ status, q: temTermo ? q.trim() : "", setor: setorFiltro, limit: 200 })
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
      toast({ type: "error", title: "Não foi possível concluir", message: (e as Error).message });
    }
  }

  async function enviar(force = false) {
    if (!draft.trim() || !active) return;
    // Colisão: conversa em atendimento por OUTRO atendente. Avisa antes de mandar.
    const outroAtende = !notaMode && active.status === "em_atendimento" && !!active.atendente && active.atendente !== (user?.nome ?? "");
    if (outroAtende && !force) {
      setColisao(active.atendente);
      return;
    }
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

  const meuNome = user?.nome ?? "";
  const janelaAberta = !!active?.window_expires_at && new Date(active.window_expires_at) > new Date();
  const contagem = (st: string) => convs.filter((c) => c.status === st).length;

  // Painel lateral do contato: fixo em telas ≥1440px; abaixo disso abre como Drawer.
  const painelConteudo = active && (
    <>
      <div className="col center" style={{ gap: 6, padding: "20px 16px 16px", borderBottom: "1px solid var(--border-subtle)", textAlign: "center" }}>
        <Avatar name={nomeOuFone(active)} size={48} citizen />
        <div style={{ fontSize: 15, lineHeight: "22px", fontWeight: 600, marginTop: 4 }}>{nomeOuFone(active)}</div>
        <div className="row center" style={{ gap: 4 }}>
          <span className="mono" style={{ fontSize: 13, color: "var(--text-secondary)" }}>{active.contact_phone}</span>
          <button
            className="btn btn-ghost btn-icon btn-sm"
            style={{ width: 24, height: 24 }}
            aria-label="Copiar telefone"
            onClick={() => {
              navigator.clipboard?.writeText(active.contact_phone).then(() => toast({ type: "success", title: "Telefone copiado" }));
            }}
          >
            <Icon name="copy" size={14} />
          </button>
        </div>
        {active.opt_out ? (
          <span className="badge badge-failed"><Icon name="ban" size={14} />Não aceita mensagens</span>
        ) : (
          <span className="badge badge-read"><Icon name="checkCircle" size={14} />Aceita mensagens</span>
        )}
      </div>
      <section style={{ padding: 16, borderBottom: "1px solid var(--border-subtle)" }}>
        <h2 className="section-title">Atendimento</h2>
        <dl className="zg-dl" style={{ gridTemplateColumns: "104px 1fr" }}>
          <dt>Setor</dt>
          <dd>{active.setor || "Sem setor"}</dd>
          <dt>Atendente</dt>
          <dd>{active.atendente ? (active.atendente === meuNome ? "Você" : active.atendente) : "—"}</dd>
          <dt>Janela 24h</dt>
          <dd style={{ color: janelaAberta ? "var(--wa-deep)" : "var(--st-failed)", fontWeight: 600 }}>{janelaAberta ? "Aberta" : "Encerrada"}</dd>
          {Object.entries(active.dados ?? {}).map(([k, v]) => (
            <Fragment key={k}>
              <dt style={{ textTransform: "capitalize" }}>{k}</dt>
              <dd style={{ overflowWrap: "anywhere" }}>{v}</dd>
            </Fragment>
          ))}
        </dl>
      </section>
      <section style={{ padding: 16, borderBottom: "1px solid var(--border-subtle)" }}>
        <h2 className="section-title">Etiquetas</h2>
        <div className="row wrap" style={{ gap: 6 }}>
          {(active.labels ?? []).map((l) => (
            <span key={l.id} className="tag">
              <span className="tag-dot" style={{ background: l.cor }} />
              {l.nome}
            </span>
          ))}
          <Dropdown
            trigger={
              <button className="btn btn-ghost btn-sm" style={{ height: 24, padding: "0 6px" }}>
                <Icon name="plus" size={14} />
                Adicionar
              </button>
            }
          >
            <div className="dropdown-label">Etiquetas</div>
            {labels.length === 0 ? (
              <div className="dropdown-item muted">Nenhuma etiqueta cadastrada</div>
            ) : (
              labels.map((l) => {
                const on = (active.labels ?? []).some((x) => x.id === l.id);
                return (
                  <div key={l.id} className="dropdown-item" onClick={() => toggleLabel(l.id)}>
                    <span style={{ width: 8, height: 8, borderRadius: 99, background: l.cor, flex: "none" }} />
                    <span className="grow">{l.nome}</span>
                    {on && <Icon name="check" size={14} />}
                  </div>
                );
              })
            )}
          </Dropdown>
        </div>
      </section>
      <section style={{ padding: 16 }}>
        <div className="row between center" style={{ marginBottom: 8 }}>
          <h2 className="section-title" style={{ margin: 0 }}>Atendimentos anteriores</h2>
          <button className="btn btn-ghost btn-sm" style={{ height: 24, padding: "0 6px" }} onClick={abrirHistorico}>
            Ver tudo
          </button>
        </div>
        {painelSess.length === 0 ? (
          <p style={{ fontSize: 13, color: "var(--text-tertiary)" }}>Nenhum atendimento anterior.</p>
        ) : (
          <div className="col" style={{ gap: 2 }}>
            {painelSess.slice(0, 5).map((s) => (
              <button
                key={s.id}
                className="col"
                onClick={() => {
                  setHistOpen(true);
                  abrirSessao(s);
                }}
                style={{ gap: 2, padding: "8px 10px", margin: "0 -10px", border: 0, borderRadius: "var(--radius-md)", background: "transparent", textAlign: "left", color: "var(--text-primary)" }}
                onMouseEnter={(e) => (e.currentTarget.style.background = "var(--bg-hover)")}
                onMouseLeave={(e) => (e.currentTarget.style.background = "transparent")}
              >
                <span className="row center" style={{ gap: 8, width: "100%" }}>
                  <span className="mono" style={{ fontSize: 12, color: "var(--text-tertiary)" }}>#{s.id}</span>
                  <span style={{ marginLeft: "auto", fontSize: 12, color: "var(--text-tertiary)" }}>{new Date(s.iniciada_em).toLocaleDateString("pt-BR")}</span>
                </span>
                <span style={{ fontSize: 12, color: "var(--text-secondary)" }}>
                  {[s.setor, s.atendente].filter(Boolean).join(" · ") || "Sem setor"}
                </span>
              </button>
            ))}
          </div>
        )}
      </section>
    </>
  );

  return (
    <div className="col" style={{ height: "100%" }}>
      <Topbar
        title="Caixa de Entrada"
        actions={
          <Button variant="primary" size="sm" icon="plus" onClick={() => setNovaOpen(true)}>
            Nova conversa
          </Button>
        }
      />
    <div className="split" style={{ flex: 1, minHeight: 0, height: "auto" }}>
      {/* Lista de conversas */}
      <section className="split-list" aria-label="Conversas">
        <div className="col" style={{ gap: 10, padding: "12px 12px 10px", borderBottom: "1px solid var(--border)" }}>
          <div className="row" style={{ gap: 8 }}>
            <div className="grow">
              <Input icon="search" aria-label="Buscar conversa" placeholder="Buscar nome, telefone, CPF ou mensagem" value={q} onChange={(e) => setQ(e.target.value)} style={{ height: 32 }} />
            </div>
          </div>
          <div className="segmented" role="group" aria-label="Status da conversa" style={{ display: "flex" }}>
            {[
              ...(canTodas ? [{ value: "", label: "Todas", n: contagem("aguardando") + contagem("em_atendimento") }] : []),
              { value: "aguardando", label: "Na fila", n: contagem("aguardando") },
              { value: "em_atendimento", label: "Atendendo", n: contagem("em_atendimento") },
              { value: "resolvido", label: "Finalizadas", n: -1 },
            ].map((t) => (
              <button
                key={t.value}
                className={filter === t.value ? "active" : ""}
                aria-pressed={filter === t.value}
                onClick={() => setFilter(t.value)}
                style={{ flex: 1, padding: "0 6px" }}
              >
                {t.label}
                {t.n >= 0 && <span className="zg-count">{t.n}</span>}
              </button>
            ))}
          </div>
          <div className="row center" style={{ gap: 6 }}>
            <select className="select" aria-label="Setor" style={{ width: "auto", height: 28, fontSize: 13, flex: 1, minWidth: 0 }} value={setorFiltro} onChange={(e) => setSetorFiltro(e.target.value)}>
              <option value="Todos">Setor: todos</option>
              {deps.map((d) => (
                <option key={d.id} value={d.nome}>
                  {d.nome}
                </option>
              ))}
            </select>
            {atendentes.length > 0 && (
              <select className="select" aria-label="Atendente" style={{ width: "auto", height: 28, fontSize: 13, flex: 1, minWidth: 0 }} value={atendenteFiltro} onChange={(e) => setAtendenteFiltro(e.target.value)}>
                <option value="Todos">Atendente: todos</option>
                {atendentes.map((a) => (
                  <option key={a} value={a}>
                    {a}
                  </option>
                ))}
              </select>
            )}
          </div>
          {(filaAguardando.length > 0 || (canTodas && ativasFiltradas.length > 0)) && (
            <div className="row center" style={{ gap: 8, fontSize: 12, color: "var(--text-tertiary)" }}>
              {filaAguardando.length > 0 && (
                <span className="row center" style={{ gap: 4, fontWeight: 600, color: slaCor(maisAntigaMin) }}>
                  <Icon name="clock" size={14} />
                  Mais antiga na fila há {maisAntigaMin} min
                </span>
              )}
              {canTodas && ativasFiltradas.length > 0 && (
                <button className="btn btn-ghost btn-sm" style={{ marginLeft: "auto", height: 24, padding: "0 6px" }} onClick={() => setBulkConfirm(true)}>
                  <Icon name="archive" size={14} />
                  Finalizar todas ({ativasFiltradas.length})
                </button>
              )}
            </div>
          )}
        </div>
        <div className="scroll-y grow">
          {erro ? (
            <EmptyState error title="Não foi possível carregar as conversas" sub={erro} action={<Button size="sm" icon="refresh" onClick={loadConvs}>Tentar novamente</Button>} />
          ) : lista.length === 0 ? (
            <EmptyState
              icon="inbox"
              title={buscandoServer && q ? "Nada encontrado" : filter === "aguardando" ? "Ninguém na fila" : "Nenhuma conversa"}
              sub={buscandoServer && q ? "Nada com esse nome, telefone, CPF ou conteúdo." : "Quando um cidadão escrever, a conversa aparece aqui."}
            />
          ) : (
            lista.map((c) => {
              const unread = c.unread_count > 0;
              const outro = c.status === "em_atendimento" && c.atendente && c.atendente !== meuNome;
              return (
                <button
                  key={c.id}
                  className={"conv-item" + (c.id === activeId ? " active" : "")}
                  aria-current={c.id === activeId ? "true" : undefined}
                  style={{ width: "100%", border: 0, borderBottom: "1px solid var(--border-subtle)", background: c.id === activeId ? undefined : "transparent", textAlign: "left", color: "inherit", font: "inherit" }}
                  onClick={() => {
                    setActiveId(c.id);
                    // Zera o contador na hora (otimista); o backend marca como lida ao carregar.
                    if (unread) setConvs((cs) => cs.map((x) => (x.id === c.id ? { ...x, unread_count: 0 } : x)));
                  }}
                >
                  <Avatar name={nomeOuFone(c)} size={36} citizen />
                  <span className="col grow" style={{ gap: 2, minWidth: 0 }}>
                    <span className="row" style={{ gap: 8, alignItems: "baseline" }}>
                      <span className="grow" style={{ minWidth: 0, fontSize: 14, lineHeight: "20px", fontWeight: unread ? 700 : 500, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
                        {nomeOuFone(c)}
                      </span>
                      <span className="mono" style={{ fontSize: 11, color: unread ? "var(--wa-deep)" : "var(--text-tertiary)", fontWeight: unread ? 600 : 400, flex: "none" }}>
                        {hhmm(c.last_msg_at)}
                      </span>
                    </span>
                    <span className="row center" style={{ gap: 4, fontSize: 13, lineHeight: "18px", color: unread ? "var(--text-primary)" : "var(--text-secondary)" }}>
                      <span className="grow" style={{ minWidth: 0, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>{c.last_message || "—"}</span>
                      {unread && (
                        <span className="zg-unread" aria-label={c.unread_count + " não lidas"}>
                          {c.unread_count}
                        </span>
                      )}
                    </span>
                    <span className="row center wrap" style={{ gap: 6, marginTop: 4, minHeight: 22 }}>
                      {c.setor && (
                        <span className="tag">
                          <span className="tag-dot" style={{ background: deps.find((d) => d.nome === c.setor)?.cor || "var(--text-faint)" }} />
                          {c.setor}
                        </span>
                      )}
                      {(c.labels ?? []).map((l) => (
                        <span key={l.id} className="tag" title={l.nome}>
                          <span className="tag-dot" style={{ background: l.cor }} />
                          {l.nome}
                        </span>
                      ))}
                      {c.status === "aguardando" && c.queued_at && (
                        <span
                          className="row center"
                          style={{ gap: 4, fontSize: 12, fontWeight: 600, color: slaCor(esperaMin(c)) }}
                          title={`Aguardando há ${esperaMin(c)} min (meta: ${slaMin} min)`}
                        >
                          <Icon name="clock" size={14} />
                          {esperaMin(c)} min
                        </span>
                      )}
                      {c.status === "resolvido" && <span className="badge badge-read" style={{ height: 20 }}>Finalizada</span>}
                      {/* Quem está atendendo: bate o olho e vê de quem é a conversa. */}
                      {c.status === "em_atendimento" && c.atendente && (
                        <span className="row center" style={{ marginLeft: "auto", gap: 6, fontSize: 12, color: outro ? "var(--text-tertiary)" : "var(--brand-text)", fontWeight: outro ? 400 : 600 }} title={"Em atendimento por " + c.atendente}>
                          {outro ? c.atendente.split(" ")[0] : "Você"}
                          <Avatar name={c.atendente} size={24} />
                        </span>
                      )}
                    </span>
                  </span>
                </button>
              );
            })
          )}
        </div>
      </section>

      {/* Detalhe da conversa */}
      {active ? (
        <section className="split-detail" aria-label={"Conversa com " + nomeOuFone(active)} style={{ minWidth: 0 }}>
          <div className="row center" style={{ gap: 12, height: 64, flex: "none", padding: "0 16px", borderBottom: "1px solid var(--border)" }}>
            <Avatar name={nomeOuFone(active)} size={36} citizen />
            <div className="grow" style={{ minWidth: 0 }}>
              <div className="row center" style={{ gap: 8 }}>
                <span style={{ fontSize: 15, lineHeight: "22px", fontWeight: 600, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>{nomeOuFone(active)}</span>
                {active.status === "aguardando" && <span className="badge badge-pending">Na fila{active.queued_at ? " · " + esperaMin(active) + " min" : ""}</span>}
                {active.status === "em_atendimento" &&
                  (active.atendente === meuNome ? (
                    <span className="badge badge-brand">Em atendimento · você</span>
                  ) : (
                    <span className="badge badge-outline">Com {active.atendente || "outro atendente"}</span>
                  ))}
                {active.status === "resolvido" && <span className="badge badge-read">Finalizada</span>}
                {active.opt_out && (
                  <span className="badge badge-failed">
                    <Icon name="ban" size={14} />
                    Opt-out
                  </span>
                )}
              </div>
              <div className="mono" style={{ fontSize: 12, lineHeight: "16px", color: "var(--text-tertiary)", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
                {active.contact_phone}
                {active.setor ? " · " + active.setor : ""}
              </div>
            </div>
            {active.status === "aguardando" && (
              <Button variant="primary" icon="userCheck" onClick={pegar}>
                Assumir conversa
              </Button>
            )}
            {active.status === "em_atendimento" && active.atendente !== meuNome && (
              <Button variant="secondary" size="sm" icon="userCheck" onClick={pegar} title="Puxar a conversa para você">
                Assumir
              </Button>
            )}
            {active.status !== "resolvido" && deps.length > 0 && (
              <Dropdown
                trigger={
                  <Button variant="secondary" size="sm" icon="transfer">
                    Transferir
                  </Button>
                }
              >
                <div className="dropdown-label">Transferir para</div>
                {deps
                  .filter((d) => d.nome !== active.setor)
                  .map((d) => (
                    <div key={d.id} className="dropdown-item" onClick={() => transferir(d.nome)}>
                      <span style={{ width: 8, height: 8, borderRadius: 99, background: d.cor, flex: "none" }} />
                      {d.nome}
                    </div>
                  ))}
              </Dropdown>
            )}
            {active.status === "em_atendimento" && (
              <Button variant="secondary" size="sm" icon="archive" onClick={resolver}>
                Finalizar
              </Button>
            )}
            <Dropdown
              trigger={
                <button className="btn btn-ghost btn-icon btn-sm" aria-label="Mais ações">
                  <Icon name="more" size={16} />
                </button>
              }
            >
              <div className="dropdown-item" onClick={() => loadMsgs(active.id)}>
                <Icon name="refresh" size={16} /> Atualizar mensagens
              </div>
              <div className="dropdown-item" onClick={abrirHistorico}>
                <Icon name="history" size={16} /> Histórico de atendimentos
              </div>
              <div className="dropdown-item" onClick={() => setActiveId(null)}>
                <Icon name="x" size={16} /> Fechar conversa
              </div>
              <div className="dropdown-sep" />
              <div className={"dropdown-item" + (active.opt_out ? "" : " danger")} onClick={toggleOptOut}>
                <Icon name={active.opt_out ? "check" : "ban"} size={16} />
                {active.opt_out ? "Recadastrar contato" : "Descadastrar (opt-out)"}
              </div>
            </Dropdown>
            <button
              className="btn btn-ghost btn-icon btn-sm"
              aria-label="Painel do contato"
              aria-pressed={painel}
              onClick={() => setPainel((v) => !v)}
              style={{ background: painel ? "var(--bg-active)" : undefined }}
            >
              <Icon name="panelRight" size={16} />
            </button>
          </div>

          <div
            ref={scrollRef}
            className="zg-chat scroll-y grow"
            style={{ padding: "16px 24px" }}
            onScroll={(e) => {
              const el = e.currentTarget;
              stickRef.current = el.scrollHeight - el.scrollTop - el.clientHeight < 100;
            }}
          >
            {msgs.length === 0 ? (
              <div className="muted" style={{ textAlign: "center", padding: 24, fontSize: 13 }}>
                Sem mensagens ainda.
              </div>
            ) : (
              msgs.flatMap((m, i) => {
                const prev = msgs[i - 1];
                const els = [];
                if (!prev || !mesmoDia(prev.created_at, m.created_at)) {
                  els.push(<DateDivider key={"d" + m.id} iso={m.created_at} />);
                }
                els.push(<MessageItem key={m.id} m={m} onReact={reagir} onReply={setReplyTo} />);
                return els;
              })
            )}
          </div>

          <div style={{ flex: "none", padding: "10px 16px 14px", borderTop: "1px solid var(--border)", background: "var(--bg-surface)" }}>
            {active.opt_out ? (
              <Alert tone="err" icon="ban">
                <b>Este contato pediu para não receber mensagens (opt-out).</b> Recadastre pelo menu de ações somente se ele autorizar.
              </Alert>
            ) : active.status === "aguardando" ? (
              <div className="row center" style={{ gap: 12, padding: "4px 0" }}>
                <div className="grow" style={{ fontSize: 13, lineHeight: "18px", color: "var(--text-secondary)" }}>
                  <b style={{ color: "var(--text-primary)", fontWeight: 600 }}>
                    Esta conversa está na fila{active.setor ? " de " + active.setor : ""}
                    {active.queued_at ? " há " + esperaMin(active) + " min" : ""}.
                  </b>{" "}
                  Assuma para responder; o cidadão vê o seu nome a partir da primeira mensagem.
                </div>
                <Button variant="primary" icon="userCheck" onClick={pegar}>
                  Assumir conversa
                </Button>
              </div>
            ) : (
              <>
                {!janelaAberta && !notaMode && (
                  <Alert tone="warn" icon="clock" style={{ marginBottom: 10 }}>
                    <b>A janela de 24 horas foi encerrada.</b> A Meta só permite responder com um template aprovado até o cidadão escrever de novo.
                  </Alert>
                )}
                <div className="row center" style={{ gap: 8, marginBottom: 8 }}>
                  <div className="segmented" role="group" aria-label="Tipo de mensagem">
                    <button className={!notaMode ? "active" : ""} aria-pressed={!notaMode} onClick={() => setNotaMode(false)}>
                      <Icon name="reply" size={14} />
                      Responder
                    </button>
                    <button className={notaMode ? "active" : ""} aria-pressed={notaMode} onClick={() => setNotaMode(true)}>
                      <Icon name="note" size={14} />
                      Nota interna
                    </button>
                  </div>
                  <span style={{ fontSize: 12, color: "var(--text-tertiary)" }}>
                    {notaMode ? "Só a equipe vê. Não é enviada ao cidadão." : janelaAberta ? "Janela de 24 horas aberta." : "Fora da janela: use um template."}
                  </span>
                </div>
                {replyTo && !notaMode && (
                  <div className="row center" style={{ gap: 10, marginBottom: 8, padding: "7px 10px", borderRadius: "var(--radius-md)", background: "var(--bg-sunken)" }}>
                    <div className="col grow" style={{ gap: 1, borderLeft: "3px solid var(--brand)", paddingLeft: 9, minWidth: 0 }}>
                      <span style={{ fontSize: 12, fontWeight: 600, color: "var(--brand-text)" }}>
                        Respondendo a {replyTo.direction === "in" ? nomeOuFone(active) : "você"}
                      </span>
                      <span style={{ fontSize: 12, color: "var(--text-tertiary)", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
                        {replyTo.body || (replyTo.media_type ? "[" + replyTo.media_type + "]" : "")}
                      </span>
                    </div>
                    <button className="btn btn-ghost btn-icon btn-sm" aria-label="Cancelar resposta" onClick={() => setReplyTo(null)}>
                      <Icon name="x" size={14} />
                    </button>
                  </div>
                )}
                {gravando ? (
                  <div className="row center" style={{ gap: 12, height: 48, padding: "0 12px", border: "1px solid var(--border)", borderRadius: "var(--radius-lg)" }}>
                    <span style={{ width: 10, height: 10, borderRadius: 99, background: "var(--st-failed)", animation: "pulse 1s infinite" }} />
                    <span className="mono strong" style={{ fontSize: 14 }}>
                      {String(Math.floor(recSeg / 60)).padStart(2, "0")}:{String(recSeg % 60).padStart(2, "0")}
                    </span>
                    <span className="grow" style={{ fontSize: 13, color: "var(--text-secondary)" }}>Gravando áudio…</span>
                    <Button variant="ghost" size="sm" onClick={() => pararGravacao(true)}>
                      Cancelar
                    </Button>
                    <Button variant="primary" size="sm" icon="send" onClick={() => pararGravacao(false)}>
                      Enviar áudio
                    </Button>
                  </div>
                ) : (
                  <div
                    style={{
                      position: "relative",
                      border: "1px solid " + (notaMode ? "var(--st-pending)" : "var(--border)"),
                      borderRadius: "var(--radius-lg)",
                      background: notaMode ? "var(--bubble-note)" : "var(--bg-surface)",
                    }}
                  >
                    {/* Autocomplete de respostas rápidas: digite / + atalho */}
                    {slashMatches.length > 0 && (
                      <div className="dropdown" style={{ bottom: "calc(100% + 6px)", left: 0, right: 0, maxHeight: 240, overflowY: "auto" }}>
                        {slashMatches.map((qr, i) => (
                          <div
                            key={qr.id}
                            className="dropdown-item"
                            onClick={() => {
                              setDraft(qr.texto);
                              setSlashOff(true);
                            }}
                            style={{ flexDirection: "column", alignItems: "stretch", gap: 2, padding: "8px 10px", background: i === 0 ? "var(--bg-hover)" : undefined }}
                          >
                            <div className="row center" style={{ gap: 7 }}>
                              {qr.atalho && <span className="mono strong" style={{ fontSize: 12 }}>{qr.atalho}</span>}
                              {qr.minha && <span className="badge badge-neutral" style={{ height: 18, fontSize: 11 }}>minha</span>}
                              {i === 0 && <span className="kbd" style={{ marginLeft: "auto" }}>Enter</span>}
                            </div>
                            <div style={{ fontSize: 12, color: "var(--text-tertiary)", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>{qr.texto}</div>
                          </div>
                        ))}
                      </div>
                    )}
                    <textarea
                      ref={taRef}
                      rows={2}
                      aria-label={notaMode ? "Nota interna" : "Mensagem para o cidadão"}
                      style={{ display: "block", width: "100%", minHeight: 52, maxHeight: 160, border: 0, resize: "none", background: "transparent", padding: "10px 12px 4px", font: "400 15px/22px var(--font-sans)", color: "var(--text-primary)", outline: "none", overflowY: "auto" }}
                      placeholder={notaMode ? "Escreva uma nota para a equipe" : "Escreva uma resposta. Digite / para respostas rápidas"}
                      value={draft}
                      onChange={(e) => {
                        setDraft(e.target.value);
                        setSlashOff(false);
                      }}
                      onKeyDown={(e) => {
                        if (slashMatches.length > 0 && (e.key === "Enter" || e.key === "Tab") && !e.shiftKey) {
                          e.preventDefault();
                          setDraft(slashMatches[0].texto);
                          setSlashOff(true);
                          return;
                        }
                        if (e.key === "Escape") {
                          setSlashOff(true);
                          return;
                        }
                        // Enter envia; Shift+Enter quebra linha (padrão WhatsApp).
                        if (e.key === "Enter" && !e.shiftKey) {
                          e.preventDefault();
                          enviar();
                        }
                      }}
                    />
                    <div className="row center" style={{ gap: 2, padding: "4px 6px 6px" }}>
                      {!notaMode && (
                        <button className="btn btn-ghost btn-icon btn-sm" aria-label="Anexar arquivo" title="Anexar arquivo" onClick={() => setMidiaOpen(true)}>
                          <Icon name="attach" size={16} />
                        </button>
                      )}
                      {!notaMode && (
                        <button className="btn btn-ghost btn-icon btn-sm" aria-label="Gravar áudio" title="Gravar áudio" onClick={iniciarGravacao}>
                          <Icon name="mic" size={16} />
                        </button>
                      )}
                      {!notaMode && (
                        <Dropdown
                          up
                          align="left"
                          trigger={
                            <button className="btn btn-ghost btn-sm">
                              <Icon name="zap" size={14} />
                              Respostas rápidas
                              <span className="kbd">/</span>
                            </button>
                          }
                        >
                          <div className="dropdown-label">Respostas rápidas</div>
                          <div style={{ maxHeight: 260, overflowY: "auto", width: 280 }}>
                            {quickReplies.length === 0 && <div className="dropdown-item muted">Nenhuma resposta cadastrada.</div>}
                            {quickReplies.map((qr) => (
                              <div key={qr.id} className="dropdown-item" style={{ alignItems: "flex-start", padding: "8px 10px" }} onClick={() => setDraft((d) => (d ? d + " " : "") + qr.texto)}>
                                <div className="col" style={{ gap: 1, minWidth: 0 }}>
                                  <span className="row center" style={{ gap: 6 }}>
                                    {qr.atalho && <span className="mono strong" style={{ fontSize: 12 }}>{qr.atalho}</span>}
                                    {qr.minha && <span className="badge badge-neutral" style={{ height: 18, fontSize: 11 }}>minha</span>}
                                  </span>
                                  <span style={{ fontSize: 12, color: "var(--text-tertiary)", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis", maxWidth: 240 }}>{qr.texto}</span>
                                </div>
                              </div>
                            ))}
                          </div>
                          <div className="dropdown-sep" />
                          <div className="dropdown-item" onClick={() => setQrOpen(true)}>
                            <Icon name="settings" size={16} /> Gerenciar minhas respostas
                          </div>
                        </Dropdown>
                      )}
                      {!notaMode && (
                        <button className="btn btn-ghost btn-sm" onClick={() => { setNovaPhone(active.contact_phone); setNovaOpen(true); }}>
                          <Icon name="template" size={14} />
                          Template
                        </button>
                      )}
                      <span className="grow" />
                      <span style={{ fontSize: 12, color: "var(--text-tertiary)", marginRight: 8, whiteSpace: "nowrap" }}>Enter envia · Shift+Enter quebra linha</span>
                      <Button variant="primary" size="sm" icon={notaMode ? "note" : "send"} disabled={sending || !draft.trim()} onClick={() => enviar()}>
                        {notaMode ? "Salvar nota" : "Enviar"}
                      </Button>
                    </div>
                  </div>
                )}
              </>
            )}
          </div>
        </section>
      ) : (
        <div className="split-detail" style={{ justifyContent: "center" }}>
          <EmptyState icon="message" title="Selecione uma conversa" sub="Escolha um contato na lista para ver o histórico e responder." />
        </div>
      )}

      {active && painel && largo && (
        <aside aria-label="Dados do contato" className="scroll-y" style={{ width: 300, flex: "none", background: "var(--bg-surface)", borderLeft: "1px solid var(--border)" }}>
          {painelConteudo}
        </aside>
      )}
    </div>

      {active && !largo && (
        <Drawer open={painel} onClose={() => setPainel(false)} width={320}>
          <div className="row between center" style={{ padding: "12px 16px", borderBottom: "1px solid var(--border)" }}>
            <span className="card-title">Dados do contato</span>
            <button className="btn btn-ghost btn-icon btn-sm" aria-label="Fechar painel" onClick={() => setPainel(false)}>
              <Icon name="x" size={16} />
            </button>
          </div>
          <div className="scroll-y grow">{painelConteudo}</div>
        </Drawer>
      )}

      <MinhasRespostasModal
        open={qrOpen}
        onClose={() => setQrOpen(false)}
        items={quickReplies}
        onChanged={() => listQuickReplies().then(setQuickReplies).catch(() => {})}
      />

      <NovaConversaModal
        open={novaOpen}
        presetPhone={novaPhone}
        onClose={() => {
          setNovaOpen(false);
          setNovaPhone("");
        }}
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

      {/* Colisão: avisa antes de responder uma conversa de outro atendente. */}
      <ConfirmModal
        open={colisao != null}
        onClose={() => setColisao(null)}
        onConfirm={() => { setColisao(null); enviar(true); }}
        icon="reply"
        title="Conversa em atendimento"
        message={`Esta conversa está sendo atendida por ${colisao}. Ao enviar, ela passa para você.`}
        confirmLabel="Assumir e enviar"
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
                  <div className="row center" style={{ gap: 8, fontSize: 12, color: "var(--text-tertiary)" }}>
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
                  <div key={k} className="row between" style={{ fontSize: 13, padding: "2px 0" }}>
                    <span className="muted">{k}</span>
                    <span className="strong">{v}</span>
                  </div>
                ))}
              </div>
            )}
            <div className="wa-canvas">
              {sessMsgs.flatMap((m, i) => {
                const prev = sessMsgs[i - 1];
                const els = [];
                if (!prev || !mesmoDia(prev.created_at, m.created_at)) {
                  els.push(<DateDivider key={"d" + m.id} iso={m.created_at} />);
                }
                els.push(<MessageItem key={m.id} m={m} />);
                return els;
              })}
            </div>
          </div>
        )}
      </Drawer>
    </div>
  );
}
