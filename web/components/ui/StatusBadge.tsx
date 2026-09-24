import { Icon } from "./Icon";
import { IconName } from "@/lib/icons";

// Fonte única de verdade para cor + ícone de status (mensagens, templates, tenants).
export const STATUS: Record<string, { label: string; cls: string; icon: IconName | null }> = {
  revisao: { label: "Conferir envio", cls: "badge-pending", icon: "alertCircle" },
  enviando: { label: "Enviando", cls: "badge-pending", icon: "clock" },
  enviado: { label: "Enviado", cls: "badge-sent", icon: "check" },
  entregue: { label: "Entregue", cls: "badge-delivered", icon: "checkDouble" },
  lido: { label: "Lido", cls: "badge-read", icon: "checkDouble" },
  falhou: { label: "Falhou", cls: "badge-failed", icon: "alertCircle" },
  pendente: { label: "Pendente", cls: "badge-pending", icon: "clock" },
  agendado: { label: "Agendado", cls: "badge-scheduled", icon: "calendar" },
  fila: { label: "Na fila", cls: "badge-pending", icon: "clock" },
  aprovado: { label: "Aprovado", cls: "badge-read", icon: "checkCircle" },
  rejeitado: { label: "Rejeitado", cls: "badge-failed", icon: "x" },
  ativo: { label: "Ativo", cls: "badge-read", icon: null },
  suspenso: { label: "Suspenso", cls: "badge-failed", icon: "ban" },
  trial: { label: "Trial", cls: "badge-scheduled", icon: null },
  bloqueado: { label: "Bloqueado", cls: "badge-failed", icon: "ban" },
};

export function StatusBadge({ status, dot }: { status: string; dot?: boolean }) {
  const s = STATUS[status] || { label: status, cls: "badge-neutral", icon: null };
  return (
    <span className={"badge " + s.cls}>
      {dot ? <span className="dot" /> : s.icon ? <Icon name={s.icon} size={12} /> : null}
      {s.label}
    </span>
  );
}
