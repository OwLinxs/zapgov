"use client";

import { ReactNode } from "react";
import Link from "next/link";
import { Icon } from "@/components/ui";
import { useTheme } from "@/lib/theme";
import { useOrgName } from "@/lib/org";
import { useNotifications } from "@/lib/notifications";

// Topbar de 56px: órgão/caminho em cima, título da página, ações da tela,
// notificações e alternância de tema.
export function Topbar({
  title,
  sub,
  actions,
  breadcrumb,
}: {
  title: ReactNode;
  sub?: string;
  actions?: ReactNode;
  breadcrumb?: string[];
}) {
  const { theme, toggleTheme } = useTheme();
  const { unread } = useNotifications();
  const org = useOrgName();
  const crumb = [org, ...(breadcrumb ?? [])].filter(Boolean).join(" · ");

  return (
    <header className="topbar">
      <div className="col" style={{ minWidth: 0, flex: "1 1 200px" }}>
        {crumb && <span className="topbar-crumb">{crumb}</span>}
        <div className="row" style={{ gap: 10, alignItems: "baseline", minWidth: 0 }}>
          <h1 className="topbar-title">{title}</h1>
          {sub && (
            <span className="topbar-sub" style={{ whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
              {sub}
            </span>
          )}
        </div>
      </div>
      <div className="row center" style={{ gap: 8, flex: "none" }}>
        {actions}
        <Link
          href="/inbox"
          className="btn btn-ghost btn-icon"
          aria-label={unread > 0 ? `Notificações, ${unread} novas` : "Notificações"}
          style={{ position: "relative" }}
        >
          <Icon name="bell" size={16} />
          {unread > 0 && (
            <span
              style={{ position: "absolute", top: 7, right: 8, width: 8, height: 8, borderRadius: "50%", background: "var(--st-failed)", boxShadow: "0 0 0 2px var(--bg-surface)" }}
            />
          )}
        </Link>
        <button
          className="btn btn-ghost btn-icon"
          onClick={toggleTheme}
          aria-label={theme === "dark" ? "Usar tema claro" : "Usar tema escuro"}
          title={theme === "dark" ? "Tema claro" : "Tema escuro"}
        >
          <Icon name={theme === "dark" ? "sun" : "moon"} size={16} />
        </button>
      </div>
    </header>
  );
}
