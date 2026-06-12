"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Logo } from "./Logo";
import { Icon, Avatar, Dropdown } from "@/components/ui";
import { useTheme } from "@/lib/theme";
import { useAuth } from "@/lib/auth";
import { useNotifications } from "@/lib/notifications";
import { TENANT } from "@/lib/data";
import { IconName } from "@/lib/icons";
import { type Papel } from "@/lib/api";

interface NavDef {
  href: string;
  label: string;
  icon: IconName;
  badge?: number;
  roles?: Papel[]; // ausente = todos os papéis
}

const NAV: NavDef[] = [
  { href: "/dashboard", label: "Dashboard", icon: "dashboard" },
  { href: "/inbox", label: "Caixa de Entrada", icon: "inbox" },
  { href: "/fila", label: "Fila ao Vivo", icon: "clock" },
  { href: "/chatbot", label: "Chatbot", icon: "message", roles: ["admin"] },
  { href: "/disparo", label: "Novo Disparo", icon: "send", roles: ["admin", "supervisor"] },
  { href: "/historico", label: "Histórico", icon: "history" },
  { href: "/contatos", label: "Contatos", icon: "contacts" },
  { href: "/templates", label: "Templates", icon: "template" },
];

const NAV_FOOT: NavDef[] = [
  { href: "/departamentos", label: "Departamentos", icon: "building", roles: ["admin"] },
  { href: "/relatorios", label: "Relatórios", icon: "download", roles: ["admin", "supervisor"] },
  { href: "/auditoria", label: "Auditoria", icon: "list", roles: ["admin", "supervisor"] },
  { href: "/config", label: "Configurações", icon: "settings", roles: ["admin"] },
];

const PAPEL_LABEL: Record<Papel, string> = {
  admin: "Administrador",
  supervisor: "Supervisor",
  atendente: "Atendente",
};

function MetaStatusPill({ status }: { status: "ativo" | "atencao" | "erro" }) {
  const map = {
    ativo: { c: "var(--st-read)", b: "var(--st-read-soft)", t: "API conectada" },
    atencao: { c: "var(--st-pending)", b: "var(--st-pending-soft)", t: "Latência alta" },
    erro: { c: "var(--st-failed)", b: "var(--st-failed-soft)", t: "Erro na conexão" },
  };
  const m = map[status] || map.ativo;
  return (
    <div
      className="row center"
      style={{ gap: 9, padding: "8px 11px", borderRadius: 8, background: m.b, border: "1px solid var(--border-subtle)" }}
    >
      <span style={{ width: 7, height: 7, borderRadius: 99, background: m.c, boxShadow: "0 0 0 3px " + m.b, flex: "none" }} />
      <div className="col" style={{ gap: 1, lineHeight: 1.25, minWidth: 0 }}>
        <div style={{ fontSize: 11.5, fontWeight: 700, color: m.c, whiteSpace: "nowrap" }}>{m.t}</div>
        <div style={{ fontSize: 10, color: "var(--text-faint)", whiteSpace: "nowrap" }}>Meta Cloud API</div>
      </div>
    </div>
  );
}

export function Sidebar() {
  const pathname = usePathname();
  const { theme, toggleTheme } = useTheme();
  const { user, logout } = useAuth();
  const { unread } = useNotifications();
  const [collapsed, setCollapsed] = useState(false);

  useEffect(() => {
    setCollapsed(localStorage.getItem("zg-sidebar") === "1");
  }, []);
  const toggle = () => {
    const v = !collapsed;
    setCollapsed(v);
    localStorage.setItem("zg-sidebar", v ? "1" : "0");
  };

  const papel = user?.papel ?? "atendente";
  const can = (n: NavDef) => !n.roles || n.roles.includes(papel);

  const item = (n: NavDef) => {
    const active = pathname === n.href || pathname.startsWith(n.href + "/");
    const badge = n.href === "/inbox" && unread > 0 ? unread : n.badge;
    return (
      <Link key={n.href} href={n.href} className={"nav-item" + (active ? " active" : "")} title={collapsed ? n.label : undefined}>
        <Icon name={n.icon} size={18} />
        {!collapsed && (
          <span className="grow" style={{ textAlign: "left" }}>
            {n.label}
          </span>
        )}
        {!collapsed && badge ? <span className="nav-badge">{badge}</span> : null}
      </Link>
    );
  };

  const userMenu = (
    <Dropdown
      up
      align={collapsed ? "left" : "right"}
      trigger={
        collapsed ? (
          <button className="btn btn-ghost btn-icon" title={user?.nome ?? ""}>
            <Avatar name={user?.nome ?? "?"} size={30} />
          </button>
        ) : (
          <button className="btn btn-ghost btn-icon btn-sm">
            <Icon name="more" size={16} />
          </button>
        )
      }
    >
      <div className="dropdown-label">{user?.email ?? ""}</div>
      {papel === "admin" && (
        <Link href="/config" className="dropdown-item">
          <Icon name="settings" size={15} />
          Configurações
        </Link>
      )}
      <div className="dropdown-sep" />
      <div className="dropdown-item danger" onClick={logout}>
        <Icon name="logout" size={15} />
        Sair
      </div>
    </Dropdown>
  );

  return (
    <aside className={"sidebar" + (collapsed ? " collapsed" : "")}>
      <div style={{ padding: "14px 12px", display: "flex", alignItems: "center", gap: 8, justifyContent: collapsed ? "center" : "space-between" }}>
        {!collapsed && <Logo />}
        <button className="btn btn-ghost btn-icon btn-sm" onClick={toggle} title={collapsed ? "Expandir menu" : "Recolher menu"}>
          <Icon name={collapsed ? "chevronRight" : "chevronLeft"} size={16} />
        </button>
      </div>

      {!collapsed && (
        <div style={{ padding: "0 14px 12px" }}>
          <MetaStatusPill status={TENANT.metaStatus} />
        </div>
      )}

      <nav className="nav-group grow scroll-y">
        {!collapsed && <div className="nav-section-label">Operação</div>}
        {NAV.filter(can).map(item)}
        {!collapsed && NAV_FOOT.some(can) && (
          <div className="nav-section-label" style={{ marginTop: 14 }}>
            Administração
          </div>
        )}
        {NAV_FOOT.filter(can).map(item)}
      </nav>

      <div className="sidebar-foot">
        <button className="theme-toggle" onClick={toggleTheme} title="Alternar tema">
          <Icon name={theme === "dark" ? "sun" : "moon"} size={16} />
          {!collapsed && <span>{theme === "dark" ? "Modo claro" : "Modo escuro"}</span>}
        </button>
        <div className="user-card">
          {collapsed ? (
            userMenu
          ) : (
            <>
              <Avatar name={user?.nome ?? "?"} size={32} />
              <div className="col grow" style={{ gap: 0, lineHeight: 1.25, minWidth: 0 }}>
                <div style={{ fontSize: 12.5, fontWeight: 700, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
                  {user?.nome ?? "—"}
                </div>
                <div style={{ fontSize: 10.5, color: "var(--text-faint)", whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>
                  {user ? PAPEL_LABEL[user.papel] : ""}
                </div>
              </div>
              {userMenu}
            </>
          )}
        </div>
      </div>
    </aside>
  );
}
