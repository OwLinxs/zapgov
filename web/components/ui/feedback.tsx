import { CSSProperties, ReactNode } from "react";
import { Icon } from "./Icon";
import { IconName } from "@/lib/icons";

export function Skel({
  w = "100%",
  h = 14,
  r = 6,
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
}: {
  icon?: IconName;
  title: string;
  sub?: string;
  action?: ReactNode;
}) {
  return (
    <div className="empty-state">
      <div className="empty-ico">
        <Icon name={icon} size={24} />
      </div>
      <div style={{ fontSize: 14, fontWeight: 700, color: "var(--text-secondary)" }}>{title}</div>
      {sub && <div style={{ fontSize: 12.5, maxWidth: 280 }}>{sub}</div>}
      {action && <div style={{ marginTop: 8 }}>{action}</div>}
    </div>
  );
}
