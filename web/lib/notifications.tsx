"use client";

import { createContext, useCallback, useContext, useEffect, useRef, useState, ReactNode } from "react";
import { getToken } from "@/lib/api";

// Evento de tempo real (SSE) — o inbox observa para recarregar na hora.
export interface SseEvent {
  type: string; // message | conversation | status
  conversation_id?: number;
  name?: string;
  preview?: string;
  seq: number; // muda a cada evento (gatilho de efeito)
}

interface NotifState {
  unread: number;
  reset: () => void;
  lastEvent: SseEvent | null;
}

const Ctx = createContext<NotifState>({ unread: 0, reset: () => {}, lastEvent: null });

// Beep curto via WebAudio (sem asset). Best-effort — alguns browsers exigem gesto.
function beep() {
  try {
    const AC = window.AudioContext || (window as unknown as { webkitAudioContext: typeof AudioContext }).webkitAudioContext;
    const ctx = new AC();
    const o = ctx.createOscillator();
    const g = ctx.createGain();
    o.connect(g);
    g.connect(ctx.destination);
    o.frequency.value = 660;
    g.gain.value = 0.04;
    o.start();
    o.stop(ctx.currentTime + 0.15);
    setTimeout(() => ctx.close(), 300);
  } catch {
    /* sem áudio */
  }
}

export function NotificationsProvider({ children }: { children: ReactNode }) {
  const [unread, setUnread] = useState(0);
  const [lastEvent, setLastEvent] = useState<SseEvent | null>(null);
  const seqRef = useRef(0);

  useEffect(() => {
    if (typeof window === "undefined") return;
    if ("Notification" in window && Notification.permission === "default") {
      Notification.requestPermission().catch(() => {});
    }
    const token = getToken();
    if (!token) return;
    const base = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";
    const es = new EventSource(`${base}/events?token=${encodeURIComponent(token)}`);
    es.onmessage = (ev) => {
      try {
        const e = JSON.parse(ev.data);
        // Publica TODO evento para o inbox recarregar na hora (tempo real).
        seqRef.current += 1;
        setLastEvent({ type: e.type, conversation_id: e.conversation_id, name: e.name, preview: e.preview, seq: seqRef.current });
        // Notifica só mensagem RECEBIDA (tem nome). Envio do atendente não tem.
        if (e.type === "message" && e.name) {
          setUnread((u) => u + 1);
          beep();
          if (document.hidden && "Notification" in window && Notification.permission === "granted") {
            new Notification(e.name, {
              body: e.preview || "Nova mensagem",
              tag: "zapgov-" + (e.conversation_id ?? ""),
            });
          }
        }
      } catch {
        /* ignora */
      }
    };
    es.onerror = () => {};
    return () => es.close();
  }, []);

  const reset = useCallback(() => setUnread(0), []);
  return <Ctx.Provider value={{ unread, reset, lastEvent }}>{children}</Ctx.Provider>;
}

export const useNotifications = () => useContext(Ctx);
