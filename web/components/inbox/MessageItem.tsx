"use client";

import { useEffect, useState } from "react";
import { WaBubble, Icon } from "@/components/ui";
import { fetchMediaBlob, mapStatus, hhmm, type ApiMessage } from "@/lib/api";

const EMOJIS = ["👍", "❤️", "😂", "😮", "😢", "🙏"];

// ReactionWrap envolve a bolha: badge da reação atual + picker no hover
// (reagir só em mensagens do cidadão; onReact ausente = sem picker).
function ReactionWrap({ m, onReact, children }: { m: ApiMessage; onReact?: (waId: string, emoji: string) => void; children: React.ReactNode }) {
  const [hover, setHover] = useState(false);
  const [open, setOpen] = useState(false);
  const podeReagir = !!onReact && m.direction === "in" && !!m.wa_message_id;

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
      {podeReagir && hover && (
        <div style={{ position: "absolute", top: 2, left: "calc(100% - 270px)", zIndex: 5 }}>
          {!open ? (
            <button
              className="btn btn-ghost btn-icon btn-sm"
              style={{ background: "var(--bg-elevated, var(--bg-sunken))", border: "1px solid var(--border-subtle)", borderRadius: 99 }}
              title="Reagir"
              onClick={() => setOpen(true)}
            >
              <Icon name="smile" size={15} />
            </button>
          ) : (
            <div
              className="row center"
              style={{
                gap: 2, background: "var(--bg-elevated, var(--bg-sunken))", border: "1px solid var(--border-subtle)",
                borderRadius: 99, padding: "3px 6px", boxShadow: "0 4px 14px rgba(0,0,0,.18)",
              }}
            >
              {EMOJIS.map((e) => (
                <span
                  key={e}
                  style={{ fontSize: 17, cursor: "pointer", padding: "0 3px" }}
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

// Renderiza uma mensagem: nota interna, texto (WaBubble) ou mídia.
export function MessageItem({ m, onReact }: { m: ApiMessage; onReact?: (waId: string, emoji: string) => void }) {
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
      <ReactionWrap m={m} onReact={onReact}>
        <WaBubble
          text={m.body}
          dir={m.direction === "out" ? "out" : "in"}
          time={hhmm(m.created_at)}
          status={m.direction === "out" ? mapStatus(m.status) : undefined}
        />
        <FailNote m={m} />
      </ReactionWrap>
    );
  }
  return (
    <ReactionWrap m={m} onReact={onReact}>
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

function MediaBubble({ m }: { m: ApiMessage }) {
  const [url, setUrl] = useState<string | null>(null);
  const [erro, setErro] = useState(false);

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
        {/* Sem media_id (outbound por link) ou erro → chip */}
        {!m.media_id || erro ? (
          <div className="row center" style={{ gap: 8 }}>
            <Icon name={m.media_type === "document" ? "fileText" : "file"} size={16} />
            <span style={{ fontSize: 12.5 }}>{m.media_name || labelTipo[m.media_type] || "Mídia"}</span>
          </div>
        ) : !url ? (
          <div className="muted" style={{ fontSize: 12, padding: "6px 0" }}>Carregando {labelTipo[m.media_type]?.toLowerCase()}…</div>
        ) : isImg ? (
          // eslint-disable-next-line @next/next/no-img-element
          <img src={url} alt={labelTipo[m.media_type]} style={{ maxWidth: "100%", borderRadius: 6, display: "block" }} />
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
