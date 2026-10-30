import { CSSProperties, ReactNode } from "react";
import { Icon } from "./Icon";
import { IconName } from "@/lib/icons";

export function Skel({
  w = "100%",
  h = 14,
  r = 4,
  style,
}: {
  w?: number | string;
  h?: number;
  r?: number;
  style?: CSSProperties;
}) {
  return <div className="skel" style={{ width: w, height: h, borderRadius: r, ...style }} />;
}

export function EmptyState({
  icon = "inbox",
  title,
  sub,
  action,
  error,
}: {
  icon?: IconName;
  title: string;
  sub?: string;
  action?: ReactNode;
  error?: boolean;
}) {
  return (
    <div className="empty-state">
      <div className={"empty-ico" + (error ? " err" : "")}>
        <Icon name={error ? "alertCircle" : icon} size={20} />
      </div>
      <div className="empty-title">{title}</div>
      {sub && <div className="empty-sub">{sub}</div>}
      {action && <div style={{ marginTop: 8 }}>{action}</div>}
    </div>
  );
}

// Alerta fixo no lugar do problema (janela de 24h, conexão com a Meta…).
export function Alert({
  tone = "info",
  icon,
  children,
  action,
  style,
}: {
  tone?: "info" | "warn" | "err" | "ok";
  icon?: IconName;
  children: ReactNode;
  action?: ReactNode;
  style?: CSSProperties;
}) {
  const ic: IconName = icon ?? (tone === "err" ? "alertCircle" : tone === "warn" ? "alert" : tone === "ok" ? "checkCircle" : "info");
  return (
    <div className={"alert alert-" + tone} role={tone === "err" ? "alert" : "status"} style={style}>
      <Icon name={ic} size={16} />
      <div className="grow">{children}</div>
      {action}
    </div>
  );
}
