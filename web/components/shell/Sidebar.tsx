"use client";

import { useEffect, useState } from "react";
import Link from "next/link";
import { usePathname } from "next/navigation";
import { Icon, Avatar, Dropdown } from "@/components/ui";
import { useAuth } from "@/lib/auth";
import { useNotifications } from "@/lib/notifications";
import { IconName } from "@/lib/icons";
import { type Papel } from "@/lib/api";
import { useMetaConn, useOrgName } from "@/lib/org";
import { initials } from "@/components/ui/Avatar";

interface NavDef {
  href: string;
  label: string;
  icon: IconName;
  roles?: Papel[]; // ausente = todos os papéis
  modulo?: string; // ausente = sempre visível; senão exige o módulo do tenant
}

// Grupos e ícones fixos do design system (Sidebar).
const GROUPS: { label: string; items: NavDef[] }[] = [
  {
    label: "Atendimento",
    items: [
      { href: "/dashboard", label: "Dashboard", icon: "dashboard", roles: ["admin", "supervisor"] },
      { href: "/inbox", label: "Caixa de Entrada", icon: "inbox" },
      { href: "/fila", label: "Fila ao Vivo", icon: "activity", modulo: "fila" },
    ],
  },
  {
    label: "Suporte T.I.",
    items: [{ href: "/base", label: "Base de Conhecimento", icon: "book", roles: ["admin", "supervisor"], modulo: "base" }],
  },
  {
    label: "Comunicação",
    items: [
      { href: "/chatbot", label: "Chatbot", icon: "bot", roles: ["admin", "supervisor"], modulo: "chatbot" },
      { href: "/disparo", label: "Novo Disparo", icon: "send", roles: ["admin", "supervisor"], modulo: "disparo" },
      { href: "/historico", label: "Campanhas", icon: "megaphone" },
      { href: "/contatos", label: "Contatos", icon: "contacts" },
      { href: "/templates", label: "Templates", icon: "template" },
    ],
  },
  {
    label: "Gestão",
    items: [
      { href: "/departamentos", label: "Departamentos", icon: "building", roles: ["admin", "supervisor"] },
      { href: "/relatorios", label: "Relatórios", icon: "chart", roles: ["admin", "supervisor"] },
      { href: "/auditoria", label: "Auditoria", icon: "shieldCheck", roles: ["admin", "supervisor"] },
      { href: "/config", label: "Configurações", icon: "settings", roles: ["admin", "supervisor"] },
    ],
  },
];

const PAPEL_LABEL: Record<Papel, string> = {
  admin: "Administrador",
  supervisor: "Supervisor",
  atendente: "Atendente",
};

export function Sidebar() {
  const pathname = usePathname();
  const { user, logout } = useAuth();
  const { unread } = useNotifications();
  const [pref, setPref] = useState(false);
  const [narrow, setNarrow] = useState(false);
  const org = useOrgName();
  const conn = useMetaConn();

  useEffect(() => {
    try {
      setPref(localStorage.getItem("zg-sidebar") === "1");
    } catch {}
    const mq = window.matchMedia("(max-width: 1279px)");
    const on = () => setNarrow(mq.matches);
    on();
    mq.addEventListener("change", on);
    return () => mq.removeEventListener("change", on);
  }, []);

  // Na Caixa de Entrada a sidebar fica sempre recolhida (mais espaço para o chat).
  const forced = pathname === "/inbox" || pathname.startsWith("/inbox/");
  const collapsed = forced || narrow || pref;
  const toggle = () => {
    const v = !pref;
    setPref(v);
    try {
      localStorage.setItem("zg-sidebar", v ? "1" : "0");
    } catch {}
  };

  const papel = user?.papel ?? "atendente";
  const temModulo = (m?: string) => !m || !!user?.modulos?.[m];
  const can = (n: NavDef) => (!n.roles || n.roles.includes(papel)) && temModulo(n.modulo);

  const item = (n: NavDef) => {
    const active = pathname === n.href || pathname.startsWith(n.href + "/");
    const badge = n.href === "/inbox" && unread > 0 ? unread : 0;
    return (
      <Link
        key={n.href}
        href={n.href}
        className={"nav-item" + (active ? " active" : "")}
        aria-current={active ? "page" : undefined}
        aria-label={collapsed ? n.label : undefined}
        title={collapsed ? n.label : undefined}
      >
        <Icon name={n.icon} size={collapsed ? 20 : 16} />
        {!collapsed && <span className="grow">{n.label}</span>}
        {badge ? (
          <span className="nav-badge" aria-label={badge + " não lidas"}>
            {badge}
          </span>
        ) : null}
      </Link>
    );
  };

  const orgName = org || "ZapGov";
  const connLabel = conn === "err" ? "WhatsApp desconectado" : "WhatsApp conectado";

  const userMenu = (
    <Dropdown
      up
      align="left"
      trigger={
        collapsed ? (
          <button className="nav-icon-btn" style={{ width: 36, height: 36 }} aria-label={"Conta de " + (user?.nome ?? "")}>
            <Avatar name={user?.nome ?? "?"} size={32} presence="on" />
          </button>
        ) : (
          <button className="nav-icon-btn" aria-label="Menu da conta">
            <Icon name="moreH" size={16} />
          </button>
        )
      }
    >
      <div className="dropdown-label">{user?.email ?? ""}</div>
      {papel === "admin" && (
        <Link href="/config" className="dropdown-item">
          <Icon name="settings" size={16} />
          Configurações
        </Link>
      )}
      {!forced && !narrow && (
        <div className="dropdown-item" onClick={toggle}>
          <Icon name={pref ? "chevronRight" : "chevronLeft"} size={16} />
          {pref ? "Expandir menu" : "Recolher menu"}
        </div>
      )}
      <div className="dropdown-sep" />
      <div className="dropdown-item danger" onClick={logout}>
        <Icon name="logout" size={16} />
        Sair
      </div>
    </Dropdown>
  );

  const visible = GROUPS.map((g) => ({ ...g, items: g.items.filter(can) })).filter((g) => g.items.length);

  return (
    <aside className={"sidebar" + (collapsed ? " collapsed" : "")} aria-label="Principal">
      <div className="sidebar-org">
        <span className="sidebar-org-mark" title={orgName}>
          {initials(orgName) || "ZG"}
        </span>
        {!collapsed && (
          <div style={{ minWidth: 0 }}>
            <b className="sidebar-org-name">{orgName}</b>
            <span className="sidebar-org-sub">ZapGov · Atendimento</span>
          </div>
        )}
      </div>

      <nav className="nav-group grow scroll-y">
        {visible.map((g, i) => (
          <div key={g.label} className="col" style={{ gap: 2 }}>
            {collapsed ? i > 0 && <span className="nav-sep" /> : <div className="nav-section-label">{g.label}</div>}
            {g.items.map(item)}
          </div>
        ))}
      </nav>

      <div className="sidebar-foot">
        {conn &&
          (collapsed ? (
            <span className={"conn-dot" + (conn === "err" ? " err" : "")} title={connLabel} aria-label={connLabel} />
          ) : (
            <div className="sidebar-conn">
              <span className={"conn-dot" + (conn === "err" ? " err" : "")} />
              <span>
                <b>{connLabel}</b>
                <br />
                Meta Cloud API
              </span>
            </div>
          ))}
        {collapsed ? (
          userMenu
        ) : (
          <div className="sidebar-user">
            <Avatar name={user?.nome ?? "?"} size={32} presence="on" />
            <div className="grow" style={{ minWidth: 0 }}>
              <b className="sidebar-user-name">{user?.nome ?? "—"}</b>
              <span className="sidebar-user-role">{user ? PAPEL_LABEL[user.papel] : ""}</span>
            </div>
            {userMenu}
          </div>
        )}
      </div>
    </aside>
  );
}
