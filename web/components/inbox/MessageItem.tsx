"use client";

import { useEffect, useState } from "react";
import { Icon } from "@/components/ui";
import { fetchMediaBlob, mapStatus, hhmm, type ApiMessage } from "@/lib/api";

const EMOJIS = ["👍", "👎", "❤️", "😂", "😉", "😮", "😢", "🙏", "😊", "👏", "🎉", "✅", "❌", "🔥", "💯", "😍", "🤝", "😡", "🤔"];

// Ticks: tiques/ícone de status ao lado do horário (mesma cor do status em todo o app).
export function Ticks({ status }: { status: string }) {
  const st = mapStatus(status);
  const map: Record<string, { cls: string; icon: string; label: string }> = {
    enviado: { cls: "sent", icon: "check", label: "Enviado" },
    entregue: { cls: "delivered", icon: "checkDouble", label: "Entregue" },
    lido: { cls: "read", icon: "checkDouble", label: "Lido" },
    falhou: { cls: "failed", icon: "alertCircle", label: "Falhou" },
    pendente: { cls: "pending", icon: "clock", label: "Pendente" },
  };
  const t = map[st] ?? map.enviado;
  return (
    <span className={"zg-ticks zg-ticks--" + t.cls} title={t.label} aria-label={t.label}>
      <Icon name={t.icon} size={16} stroke={2} />
    </span>
  );
}

// ReactionWrap envolve a bolha: reação atual + ações no hover
// (reagir só em mensagens do cidadão; responder em qualquer mensagem real).
function ReactionWrap({ m, onReact, onReply, children }: { m: ApiMessage; onReact?: (waId: string, emoji: string) => void; onReply?: (m: ApiMessage) => void; children: React.ReactNode }) {
  const [hover, setHover] = useState(false);
  const [open, setOpen] = useState(false);
  const podeReagir = !!onReact && m.direction === "in" && !!m.wa_message_id;
  // Só mensagens com wamid real podem ser citadas (notas internas não).
  const podeResponder = !!onReply && m.direction !== "note" && m.wa_message_id.startsWith("wamid");
  const out = m.direction === "out";

  return (
    <div
      className={"zg-msg " + (out ? "zg-msg--out" : "zg-msg--in")}
      style={{ position: "relative" }}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => {
        setHover(false);
        setOpen(false);
      }}
    >
      {children}
      {m.reaction && (
        <button
          className="zg-msg-react"
          style={{ cursor: podeReagir ? "pointer" : "default" }}
          title={podeReagir ? "Clique para remover a reação" : undefined}
          aria-label={"Reação " + m.reaction}
          onClick={() => podeReagir && onReact!(m.wa_message_id, "")}
        >
          {m.reaction}
        </button>
      )}
      {(podeReagir || podeResponder) && hover && (
        <div className="row center" style={{ position: "absolute", top: 0, [out ? "right" : "left"]: "calc(100% + 6px)", zIndex: 5, gap: 4 }}>
          {podeResponder && !open && (
            <button className="btn btn-secondary btn-icon btn-sm" style={{ borderRadius: 99 }} aria-label="Responder citando" title="Responder (citar)" onClick={() => onReply!(m)}>
              <Icon name="reply" size={14} />
            </button>
          )}
          {podeReagir && !open && (
            <button className="btn btn-secondary btn-icon btn-sm" style={{ borderRadius: 99 }} aria-label="Reagir" title="Reagir" onClick={() => setOpen(true)}>
              <Icon name="smile" size={14} />
            </button>
          )}
          {podeReagir && open && (
            <div className="row wrap" style={{ gap: 2, width: 216, background: "var(--bg-elevated)", border: "1px solid var(--border)", borderRadius: "var(--radius-lg)", padding: "6px 8px", boxShadow: "var(--shadow-lg)" }}>
              {EMOJIS.map((e) => (
                <button
                  key={e}
                  style={{ fontSize: 18, border: 0, background: "transparent", padding: "2px 4px", lineHeight: 1.2, borderRadius: "var(--radius-sm)" }}
                  aria-label={"Reagir com " + e}
                  onClick={() => {
                    onReact!(m.wa_message_id, e);
                    setOpen(false);
                  }}
                >
                  {e}
                </button>
              ))}
            </div>
          )}
        </div>
      )}
    </div>
  );
}

// Quote: bloco da mensagem citada dentro da bolha (responder).
function Quote({ text }: { text: string }) {
  return (
    <div
      style={{
        borderLeft: "3px solid var(--brand)", background: "var(--bg-sunken)", borderRadius: "var(--radius-sm)",
        padding: "4px 8px", fontSize: 13, lineHeight: "18px", marginBottom: 5, color: "var(--text-secondary)",
        maxHeight: 54, overflow: "hidden", whiteSpace: "pre-wrap",
      }}
    >
      {text}
    </div>
  );
}

// Author: nome do atendente que enviou (só no painel; cidadão não vê).
function Author({ m }: { m: ApiMessage }) {
  if (m.direction !== "out" || !m.autor) return null;
  return <div className="zg-msg-author">{m.autor}</div>;
}

function Meta({ m }: { m: ApiMessage }) {
  return (
    <span className="zg-msg-meta">
      {hhmm(m.created_at)}
      {m.direction === "out" && <Ticks status={m.status} />}
    </span>
  );
}

// dateLabel: rótulo do separador de data (Hoje / Ontem / dd/mm/aaaa).
export function dateLabel(iso: string): string {
  const d = new Date(iso);
  const hoje = new Date();
  const ontem = new Date();
  ontem.setDate(hoje.getDate() - 1);
  if (d.toDateString() === hoje.toDateString()) return "Hoje";
  if (d.toDateString() === ontem.toDateString()) return "Ontem";
  return d.toLocaleDateString("pt-BR");
}

// mesmoDia: true se dois ISO caem no mesmo dia (para decidir o separador).
export function mesmoDia(a: string, b: string): boolean {
  return new Date(a).toDateString() === new Date(b).toDateString();
}

// DateDivider: pílula central de data entre blocos de mensagens.
export function DateDivider({ iso }: { iso: string }) {
  return <span className="zg-day">{dateLabel(iso)}</span>;
}

// Ícone do evento de sistema conforme o texto (assumiu / transferiu / bot / encerrou).
function eventoIcon(body: string): string {
  const b = body.toLowerCase();
  if (b.includes("transfer")) return "transfer";
  if (b.includes("assumiu") || b.includes("atribu")) return "userCheck";
  if (b.includes("encerr") || b.includes("finaliz") || b.includes("resolv")) return "archive";
  if (b.includes("bot") || b.includes("encaminh")) return "bot";
  return "info";
}

// Renderiza uma mensagem: evento de sistema, nota interna, texto ou mídia.
export function MessageItem({ m, onReact, onReply }: { m: ApiMessage; onReact?: (waId: string, emoji: string) => void; onReply?: (m: ApiMessage) => void }) {
  if (m.direction === "evento") {
    // Registro do atendimento (resolver/assumir/transferir): só a equipe vê.
    return (
      <span className="zg-sys" title={"Registro do atendimento · " + hhmm(m.created_at) + " · visível só para a equipe"}>
        <Icon name={eventoIcon(m.body)} size={14} />
        {m.body}
      </span>
    );
  }
  if (m.direction === "note") {
    return (
      <div className="zg-msg zg-msg--note">
        <div className="zg-bubble" style={{ whiteSpace: "pre-wrap" }}>
          <div className="zg-msg-author">
            <Icon name="note" size={14} />
            Nota interna{m.autor ? " · " + m.autor : ""} · só a equipe vê
          </div>
          {m.body}
          <span className="zg-msg-meta">{hhmm(m.created_at)}</span>
        </div>
      </div>
    );
  }
  return (
    <ReactionWrap m={m} onReact={onReact} onReply={onReply}>
      {m.media_type ? (
        <MediaBubble m={m} />
      ) : (
        <div className="zg-bubble" style={{ whiteSpace: "pre-wrap" }}>
          <Author m={m} />
          {m.reply_preview && <Quote text={m.reply_preview} />}
          {m.body}
          <Meta m={m} />
        </div>
      )}
      <FailNote m={m} />
    </ReactionWrap>
  );
}

// FailNote mostra o motivo quando o envio falhou (ex.: número sem WhatsApp).
function FailNote({ m }: { m: ApiMessage }) {
  if (m.status !== "failed" || m.direction !== "out") return null;
  return (
    <div className="zg-msg-fail">
      <Icon name="alertCircle" size={14} />
      {m.error || "Não foi possível entregar. Tente reenviar."}
    </div>
  );
}

// Lightbox: imagem ampliada em overlay (clique fora fecha; Esc fecha).
function Lightbox({ url, onClose }: { url: string; onClose: () => void }) {
  useEffect(() => {
    const onKey = (e: KeyboardEvent) => e.key === "Escape" && onClose();
    window.addEventListener("keydown", onKey);
    return () => window.removeEventListener("keydown", onKey);
  }, [onClose]);
  return (
    <div
      onClick={onClose}
      style={{
        position: "fixed", inset: 0, zIndex: 1000, background: "rgba(0,0,0,.82)",
        display: "flex", alignItems: "center", justifyContent: "center", cursor: "zoom-out",
      }}
    >
      {/* eslint-disable-next-line @next/next/no-img-element */}
      <img
        src={url}
        alt="Imagem"
        style={{ maxWidth: "92vw", maxHeight: "92vh", borderRadius: 8, boxShadow: "0 10px 40px rgba(0,0,0,.5)" }}
        onClick={(e) => e.stopPropagation()}
      />
      <a
        href={url}
        download="imagem"
        onClick={(e) => e.stopPropagation()}
        className="btn btn-secondary btn-sm"
        style={{ position: "fixed", top: 18, right: 56 }}
      >
        <Icon name="download" size={15} /> Baixar
      </a>
      <button className="btn btn-secondary btn-icon btn-sm" style={{ position: "fixed", top: 18, right: 18 }} onClick={onClose}>
        <Icon name="x" size={15} />
      </button>
    </div>
  );
}

function MediaBubble({ m }: { m: ApiMessage }) {
  const [url, setUrl] = useState<string | null>(null);
  const [erro, setErro] = useState(false);
  const [zoom, setZoom] = useState(false);

  const isImg = m.media_type === "image" || m.media_type === "sticker";
  const isAudio = m.media_type === "audio";
  const isVideo = m.media_type === "video";

  // Inbound tem media_id (busca os bytes via proxy). Outbound (enviado por link) não.
  useEffect(() => {
    if (!m.media_id) return;
    let revoked = false;
    let obj = "";
    fetchMediaBlob(m.media_id)
      .then((u) => {
        if (revoked) {
          URL.revokeObjectURL(u);
          return;
        }
        obj = u;
        setUrl(u);
      })
      .catch(() => setErro(true));
    return () => {
      revoked = true;
      if (obj) URL.revokeObjectURL(obj);
    };
  }, [m.media_id]);

  const labelTipo: Record<string, string> = { image: "Imagem", audio: "Áudio", video: "Vídeo", document: "Documento", sticker: "Figurinha" };

  return (
    <div className="zg-bubble" style={{ maxWidth: 280 }}>
        <Author m={m} />
        {m.reply_preview && <Quote text={m.reply_preview} />}
        {/* Sem media_id (outbound por link) ou erro → chip */}
        {!m.media_id || erro ? (
          <div className="row center" style={{ gap: 8 }}>
            <Icon name={m.media_type === "document" ? "fileText" : "file"} size={16} />
            <span style={{ fontSize: 13 }}>{m.media_name || labelTipo[m.media_type] || "Mídia"}</span>
          </div>
        ) : !url ? (
          <div className="zg-media" style={{ width: 240, height: 120, fontSize: 13 }}>Carregando {labelTipo[m.media_type]?.toLowerCase()}…</div>
        ) : isImg ? (
          <>
            {/* eslint-disable-next-line @next/next/no-img-element */}
            <img
              src={url}
              alt={labelTipo[m.media_type]}
              style={{ maxWidth: "100%", borderRadius: 6, display: "block", cursor: "zoom-in" }}
              onClick={() => setZoom(true)}
              title="Clique para ampliar"
            />
            {zoom && <Lightbox url={url} onClose={() => setZoom(false)} />}
          </>
        ) : isAudio ? (
          <audio controls src={url} style={{ width: 220 }} />
        ) : isVideo ? (
          <video controls src={url} style={{ maxWidth: "100%", borderRadius: 6 }} />
        ) : (
          <a href={url} download={m.media_name || "arquivo"} className="row center" style={{ gap: 8, color: "inherit" }}>
            <Icon name="fileText" size={16} />
            <span style={{ fontSize: 13 }}>{m.media_name || "Documento"}</span>
            <Icon name="download" size={14} />
          </a>
        )}
        {m.body && <div style={{ marginTop: 6, whiteSpace: "pre-wrap" }}>{m.body}</div>}
        <Meta m={m} />
    </div>
  );
}
