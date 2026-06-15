"use client";

import { useEffect, useState } from "react";
import { Drawer, EmptyState, Icon } from "@/components/ui";
import { MessageItem } from "@/components/inbox/MessageItem";
import { listSessions, listSessionMessages, type ApiSession, type ApiMessage } from "@/lib/api";

// ConversaHistorico mostra o histórico de uma conversa por sessão (janela de
// atendimento). Reutilizado pela Auditoria e pelos Chamados (conversa de origem).
export function ConversaHistorico({ convId, onClose }: { convId: number; onClose: () => void }) {
  const [sessions, setSessions] = useState<ApiSession[]>([]);
  const [sel, setSel] = useState<number | null>(null);
  const [msgs, setMsgs] = useState<ApiMessage[]>([]);
  const [loading, setLoading] = useState(true);

  useEffect(() => {
    listSessions(convId)
      .then((ss) => {
        setSessions(ss);
        if (ss[0]) setSel(ss[0].id); // janela mais recente primeiro
      })
      .catch(() => setSessions([]))
      .finally(() => setLoading(false));
  }, [convId]);

  useEffect(() => {
    if (sel == null) return;
    listSessionMessages(sel).then(setMsgs).catch(() => setMsgs([]));
  }, [sel]);

  function fmt(iso: string) {
    return new Date(iso).toLocaleString("pt-BR", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" });
  }

  return (
    <Drawer open onClose={onClose} width={500}>
      <div className="row between center" style={{ marginBottom: 12 }}>
        <span className="strong" style={{ fontSize: 15 }}>Histórico da conversa</span>
        <button className="btn btn-ghost btn-icon btn-sm" onClick={onClose}><Icon name="x" size={16} /></button>
      </div>

      {loading ? (
        <div className="muted" style={{ padding: 20 }}>Carregando…</div>
      ) : sessions.length === 0 ? (
        <EmptyState icon="message" title="Sem histórico" sub="Esta conversa não tem sessões registradas." />
      ) : (
        <div className="col" style={{ gap: 12 }}>
          <select className="select" value={sel ?? ""} onChange={(e) => setSel(Number(e.target.value))}>
            {sessions.map((s) => (
              <option key={s.id} value={s.id}>
                {fmt(s.iniciada_em)}{s.encerrada_em ? " → " + fmt(s.encerrada_em) : " (aberta)"}
                {s.atendente ? " · " + s.atendente : ""}{s.setor ? " · " + s.setor : ""}
              </option>
            ))}
          </select>

          <div className="wa-canvas scroll-y" style={{ maxHeight: "calc(100vh - 180px)", borderRadius: 8, padding: 10 }}>
            {msgs.length === 0 ? (
              <div className="muted" style={{ textAlign: "center", padding: 18, fontSize: 12.5 }}>Sem mensagens nesta janela.</div>
            ) : (
              msgs.map((m) => <MessageItem key={m.id} m={m} />)
            )}
          </div>
        </div>
      )}
    </Drawer>
  );
}
