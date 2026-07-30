"use client";

import { useEffect, useState } from "react";
import { WaBubble, Icon } from "@/components/ui";
import { fetchMediaBlob, mapStatus, hhmm, type ApiMessage } from "@/lib/api";

const EMOJIS = ["👍", "👎", "❤️", "😂", "😉", "😮", "😢", "🙏", "😊", "👏", "🎉", "✅", "❌", "🔥", "💯", "😍", "🤝", "😡", "🤔"];

// ReactionWrap envolve a bolha: badge da reação atual + ações no hover
// (reagir só em mensagens do cidadão; responder em qualquer mensagem real).
function ReactionWrap({ m, onReact, onReply, children }: { m: ApiMessage; onReact?: (waId: string, emoji: string) => void; onReply?: (m: ApiMessage) => void; children: React.ReactNode }) {
  const [hover, setHover] = useState(false);
  const [open, setOpen] = useState(false);
  const podeReagir = !!onReact && m.direction === "in" && !!m.wa_message_id;
  // Só mensagens com wamid real podem ser citadas (notas internas não).
  const podeResponder = !!onReply && m.direction !== "note" && m.wa_message_id.startsWith("wamid");

  return (
    <div
      style={{ position: "relative" }}
      onMouseEnter={() => setHover(true)}
      onMouseLeave={() => { setHover(false); setOpen(false); }}
    >
      {children}
      {m.reaction && (
        <div className={"wa-row " + (m.direction === "out" ? "out" : "in")} style={{ marginTop: -8 }}>
          <span
            style={{
              fontSize: 13, background: "var(--bg-elevated, var(--bg-sunken))", border: "1px solid var(--border-subtle)",
              borderRadius: 99, padding: "1px 7px", boxShadow: "0 1px 2px rgba(0,0,0,.12)",
              cursor: podeReagir ? "pointer" : "default",
            }}
            title={podeReagir ? "Clique para remover a reação" : undefined}
            onClick={() => podeReagir && onReact!(m.wa_message_id, "")}
          >
            {m.reaction}
          </span>
        </div>
      )}
      {(podeReagir || podeResponder) && hover && (
        <div className="row center" style={{ position: "absolute", top: 2, left: "calc(100% - 270px)", zIndex: 5, gap: 4 }}>
          {podeResponder && !open && (
            <button
              className="btn btn-ghost btn-icon btn-sm"
              style={{ background: "var(--bg-elevated, var(--bg-sunken))", border: "1px solid var(--border-subtle)", borderRadius: 99 }}
              title="Responder (citar)"
              onClick={() => onReply!(m)}
            >
              <Icon name="reply" size={15} />
            </button>
          )}
          {podeReagir && !open && (
            <button
              className="btn btn-ghost btn-icon btn-sm"
              style={{ background: "var(--bg-elevated, var(--bg-sunken))", border: "1px solid var(--border-subtle)", borderRadius: 99 }}
              title="Reagir"
              onClick={() => setOpen(true)}
            >
              <Icon name="smile" size={15} />
            </button>
          )}
          {podeReagir && open && (
            <div
              className="row wrap"
              style={{
                gap: 2, width: 216, background: "var(--bg-elevated, var(--bg-sunken))", border: "1px solid var(--border-subtle)",
                borderRadius: 12, padding: "6px 8px", boxShadow: "0 4px 14px rgba(0,0,0,.18)",
              }}
            >
              {EMOJIS.map((e) => (
                <span
                  key={e}
                  style={{ fontSize: 18, cursor: "pointer", padding: "2px 4px", lineHeight: 1.2 }}
                  onClick={() => { onReact!(m.wa_message_id, e); setOpen(false); }}
                >
                  {e}
                </span>
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
        borderLeft: "3px solid var(--wa)", background: "rgba(127,127,127,.12)", borderRadius: 6,
        padding: "4px 8px", fontSize: 11.5, marginBottom: 5, opacity: 0.85, lineHeight: 1.4,
        maxHeight: 52, overflow: "hidden", whiteSpace: "pre-wrap",
      }}
    >
      {text}
    </div>
  );
}

// AuthorTag: nome do atendente que enviou (só no painel; cidadão não vê).
function AuthorTag({ m }: { m: ApiMessage }) {
  if (m.direction !== "out" || !m.autor) return null;
  return (
    <div className="wa-row out" style={{ marginBottom: 1 }}>
      <span style={{ fontSize: 10.5, fontWeight: 700, color: "var(--wa)", paddingRight: 2 }}>{m.autor}</span>
    </div>
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

// DateDivider: pílula central de data entre blocos de mensagens (estilo WhatsApp).
export function DateDivider({ iso }: { iso: string }) {
  return (
    <div className="wa-row" style={{ justifyContent: "center", margin: "6px 0" }}>
      <span
        style={{
          fontSize: 11, fontWeight: 600, color: "var(--text-tertiary)", background: "var(--bg-sunken)",
          border: "1px solid var(--border-subtle)", borderRadius: 99, padding: "3px 12px", textTransform: "uppercase",
          letterSpacing: 0.3,
        }}
      >
        {dateLabel(iso)}
      </span>
    </div>
  );
}

// Renderiza uma mensagem: evento de sistema, nota interna, texto (WaBubble) ou mídia.
export function MessageItem({ m, onReact, onReply }: { m: ApiMessage; onReact?: (waId: string, emoji: string) => void; onReply?: (m: ApiMessage) => void }) {
  if (m.direction === "evento") {
    // Evento de sistema (resolver/assumir/transferir) — visual de nota interna,
    // centralizado, visível só para a equipe (nunca vai pro cidadão).
    return (
      <div className="wa-row" style={{ justifyContent: "center" }}>
        <div
          style={{
            maxWidth: 360, background: "var(--brand-soft)", border: "1px solid var(--border-subtle)",
            borderRadius: 10, padding: "8px 12px", fontSize: 12.5, lineHeight: 1.5, textAlign: "center",
          }}
          title="Registro do atendimento · visível só para a equipe"
        >
          <div className="row center" style={{ gap: 6, marginBottom: 2, color: "var(--brand-text)", fontWeight: 700, fontSize: 11, justifyContent: "center" }}>
            <Icon name="info" size={12} /> Registro do atendimento
          </div>
          <div style={{ color: "var(--text-primary)" }}>{m.body}</div>
          <div style={{ fontSize: 10, color: "var(--text-faint)", marginTop: 3 }}>{hhmm(m.created_at)}</div>
        </div>
      </div>
    );
  }
  if (m.direction === "note") {
    return (
      <div className="wa-row" style={{ justifyContent: "center" }}>
        <div
          style={{
            maxWidth: 340, background: "var(--st-pending-soft)", border: "1px solid var(--st-pending)",
            borderRadius: 10, padding: "8px 12px", fontSize: 12.5, lineHeight: 1.5, whiteSpace: "pre-wrap",
          }}
        >
          <div className="row center" style={{ gap: 6, marginBottom: 3, color: "var(--st-pending)", fontWeight: 700, fontSize: 11 }}>
            <Icon name="edit" size={12} /> Nota interna{m.autor ? " · " + m.autor : ""}
          </div>
          <div style={{ color: "var(--text-primary)" }}>{m.body}</div>
          <div style={{ fontSize: 10, color: "var(--text-faint)", marginTop: 3, textAlign: "right" }}>{hhmm(m.created_at)}</div>
        </div>
      </div>
    );
  }
  if (!m.media_type) {
    return (
      <ReactionWrap m={m} onReact={onReact} onReply={onReply}>
        <AuthorTag m={m} />
        {m.reply_preview ? (
          // Bolha custom com a citação (WaBubble não suporta children).
          <div className={"wa-row " + (m.direction === "out" ? "out" : "in")}>
            <div className={"wa-bubble " + (m.direction === "out" ? "out" : "in")}>
              <Quote text={m.reply_preview} />
              <div style={{ whiteSpace: "pre-wrap" }}>{m.body}</div>
              <div className="wa-meta">
                {hhmm(m.created_at)}
                {m.direction === "out" && <Icon name="checkDouble" size={14} style={{ color: m.status === "read" ? "var(--st-read)" : "inherit" }} />}
              </div>
            </div>
          </div>
        ) : (
          <WaBubble
            text={m.body}
            dir={m.direction === "out" ? "out" : "in"}
            time={hhmm(m.created_at)}
            status={m.direction === "out" ? mapStatus(m.status) : undefined}
          />
        )}
        <FailNote m={m} />
      </ReactionWrap>
    );
  }
  return (
    <ReactionWrap m={m} onReact={onReact} onReply={onReply}>
      <AuthorTag m={m} />
      <MediaBubble m={m} />
      <FailNote m={m} />
    </ReactionWrap>
  );
}

// FailNote mostra o motivo quando o envio falhou (ex.: número sem WhatsApp).
function FailNote({ m }: { m: ApiMessage }) {
  if (m.status !== "failed" || m.direction !== "out") return null;
  return (
    <div className="wa-row out" style={{ marginTop: 2 }}>
      <span className="row center" style={{ gap: 5, fontSize: 11, color: "var(--st-failed)", fontWeight: 600 }}>
        <Icon name="alertCircle" size={12} />
        {m.error || "Falha no envio"}
      </span>
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
    <div className={"wa-row " + m.direction}>
      <div className={"wa-bubble " + m.direction} style={{ maxWidth: 260 }}>
        {m.reply_preview && <Quote text={m.reply_preview} />}
        {/* Sem media_id (outbound por link) ou erro → chip */}
        {!m.media_id || erro ? (
          <div className="row center" style={{ gap: 8 }}>
            <Icon name={m.media_type === "document" ? "fileText" : "file"} size={16} />
            <span style={{ fontSize: 12.5 }}>{m.media_name || labelTipo[m.media_type] || "Mídia"}</span>
          </div>
        ) : !url ? (
          <div className="muted" style={{ fontSize: 12, padding: "6px 0" }}>Carregando {labelTipo[m.media_type]?.toLowerCase()}…</div>
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
            <span style={{ fontSize: 12.5 }}>{m.media_name || "Documento"}</span>
            <Icon name="download" size={14} />
          </a>
        )}
        {m.body && <div style={{ marginTop: 6, fontSize: 13 }}>{m.body}</div>}
        <div className="wa-meta">
          {hhmm(m.created_at)}
          {m.direction === "out" && <Icon name="checkDouble" size={14} />}
        </div>
      </div>
    </div>
  );
}
