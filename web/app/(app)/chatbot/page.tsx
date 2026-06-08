"use client";

import dynamic from "next/dynamic";
import { Topbar } from "@/components/shell/Topbar";
import { Icon } from "@/components/ui";

// React Flow precisa de window → carrega só no cliente.
const FlowCanvas = dynamic(() => import("@/components/flow/FlowCanvas"), {
  ssr: false,
  loading: () => (
    <div className="empty-state" style={{ height: "100%" }}>
      <div className="empty-ico">
        <Icon name="layers" size={24} />
      </div>
      <div className="strong">Carregando editor…</div>
    </div>
  ),
});

export default function ChatbotPage() {
  return (
    <>
      <Topbar title="Chatbot" sub="Monte o fluxo de atendimento arrastando e ligando blocos" />
      <div style={{ flex: 1, minHeight: 0 }}>
        <FlowCanvas />
      </div>
    </>
  );
}
