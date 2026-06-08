"use client";

import { useEffect, useState } from "react";
import { WaBubble, Icon } from "@/components/ui";
import { fetchMediaBlob, mapStatus, hhmm, type ApiMessage } from "@/lib/api";

// Renderiza uma mensagem: texto (WaBubble) ou mídia (imagem/áudio/vídeo/documento).
export function MessageItem({ m }: { m: ApiMessage }) {
  if (!m.media_type) {
    return (
      <WaBubble
        text={m.body}
        dir={m.direction}
        time={hhmm(m.created_at)}
        status={m.direction === "out" ? mapStatus(m.status) : undefined}
      />
    );
  }
  return <MediaBubble m={m} />;
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
