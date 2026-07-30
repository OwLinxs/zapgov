"use client";

import { useCallback, useEffect, useState } from "react";
import { Topbar } from "@/components/shell/Topbar";
import { Button, Dropdown, Field, Icon, Input, Modal, Skel, useToast } from "@/components/ui";
import {
  listTenants, createTenant, updateTenantPlano, updateTenantModulos, setTenantMeta, suspendTenant, activateTenant,
  type ApiTenant, type TenantMeta,
} from "@/lib/api";

// Módulos que podem ser ligados/desligados por órgão (gate do menu + rotas).
const MODULOS: { key: string; label: string; hint: string }[] = [
  { key: "chatbot", label: "Chatbot / Fluxo", hint: "Editor do fluxo do bot e config de atendimento automático" },
  { key: "fila", label: "Fila ao Vivo", hint: "Painel de fila em tempo real" },
  { key: "chamados", label: "Chamados T.I.", hint: "Helpdesk interno (kanban, despacho a técnicos)" },
  { key: "base", label: "Base de Conhecimento", hint: "Artigos de solução dos atendimentos" },
  { key: "disparo", label: "Disparo em massa", hint: "Campanhas de template para contatos" },
];

const PLANOS = ["Gov Pro", "Business", "Enterprise"];
const PLANO_CLS: Record<string, string> = {
  "Gov Pro": "badge-brand",
  Business: "badge-delivered",
  Enterprise: "badge-read",
};

export default function SuperAdminPage() {
  const toast = useToast();
  const [list, setList] = useState<ApiTenant[]>([]);
  const [loading, setLoading] = useState(true);
  const [q, setQ] = useState("");
  const [novo, setNovo] = useState(false);
  const [plano, setPlano] = useState<ApiTenant | null>(null);
  const [modulos, setModulos] = useState<ApiTenant | null>(null);
  const [meta, setMeta] = useState<ApiTenant | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try { setList(await listTenants()); }
    catch (e) { toast({ type: "error", title: "Erro", message: (e as Error).message }); }
    finally { setLoading(false); }
  }, [toast]);
  useEffect(() => { load(); }, [load]);

  async function toggle(t: ApiTenant) {
    try {
      await (t.ativo ? suspendTenant(t.id) : activateTenant(t.id));
      load();
    } catch (e) { toast({ type: "error", title: "Erro", message: (e as Error).message }); }
  }

  const rows = list.filter((t) =>
    !q || t.nome.toLowerCase().includes(q.toLowerCase()) || t.subdominio.includes(q.toLowerCase()),
  );
  const ativos = list.filter((t) => t.ativo).length;
  const suspensos = list.length - ativos;

  const stats: [string, string, string, string][] = [
    ["Órgãos", String(list.length), "var(--brand-text)", "var(--brand-soft)"],
    ["Ativos", String(ativos), "var(--st-read)", "var(--st-read-soft)"],
    ["Suspensos", String(suspensos), "var(--st-failed)", "var(--st-failed-soft)"],
  ];

  return (
    <>
      <Topbar
        title="Super Admin"
        sub="Gestão de órgãos (tenants) e credenciais da API Meta"
        actions={
          <>
            <Button variant="secondary" icon="refresh" onClick={load}>Atualizar</Button>
            <Button variant="primary" icon="plus" onClick={() => setNovo(true)}>Novo órgão</Button>
          </>
        }
      />
      <div className="content">
        <div className="content-pad col" style={{ gap: 16 }}>
          <div className="kpi-grid" style={{ gridTemplateColumns: "repeat(3, 1fr)" }}>
            {stats.map(([label, val, c, b]) => (
              <div key={label} className="card kpi">
                <div className="kpi-top"><span className="kpi-label">{label}</span>
                  <span className="kpi-ico" style={{ background: b, color: c }}><Icon name="building" size={16} /></span>
                </div>
                <div className="kpi-value">{val}</div>
              </div>
            ))}
          </div>

          <div style={{ maxWidth: 360 }}>
            <Input placeholder="Buscar por nome ou subdomínio…" value={q} onChange={(e) => setQ(e.target.value)} />
          </div>

          <div className="card" style={{ overflow: "visible" }}>
            {loading ? (
              <div className="card-pad col" style={{ gap: 10 }}>{[0, 1, 2].map((i) => <Skel key={i} h={24} />)}</div>
            ) : (
              <table className="ds-table">
                <thead>
                  <tr><th>Órgão</th><th>Plano</th><th>API Meta</th><th>Status</th><th style={{ width: 40 }}></th></tr>
                </thead>
                <tbody>
                  {rows.map((t) => (
                    <tr key={t.id}>
                      <td className="strong">
                        <div className="col" style={{ gap: 0 }}>
                          <span>{t.nome}</span>
                          <span className="faint" style={{ fontSize: 11 }}>{t.subdominio}.zapgov.com.br</span>
                        </div>
                      </td>
                      <td><span className={"badge " + (PLANO_CLS[t.plano] || "badge-neutral")}>{t.plano}</span></td>
                      <td><MetaStatus tenant={t} /></td>
                      <td>
                        <span className={"badge " + (t.ativo ? "badge-read" : "badge-neutral")}>
                          <span className="dot" /> {t.ativo ? "Ativo" : "Suspenso"}
                        </span>
                      </td>
                      <td>
                        <Dropdown trigger={<button className="btn btn-ghost btn-icon btn-sm"><Icon name="more" size={15} /></button>}>
                          <div className="dropdown-item" onClick={() => setMeta(t)}><Icon name="phone" size={15} /> Credenciais Meta</div>
                          <div className="dropdown-item" onClick={() => setModulos(t)}><Icon name="grid" size={15} /> Módulos</div>
                          <div className="dropdown-item" onClick={() => setPlano(t)}><Icon name="dollar" size={15} /> Plano e limite</div>
                          <div className="dropdown-sep" />
                          {t.id === 1 ? (
                            <div className="dropdown-item" style={{ opacity: 0.5, cursor: "not-allowed" }}><Icon name="shield" size={15} /> Plataforma</div>
                          ) : (
                            <div className="dropdown-item danger" onClick={() => toggle(t)}>
                              <Icon name="ban" size={15} /> {t.ativo ? "Suspender" : "Reativar"}
                            </div>
                          )}
                        </Dropdown>
                      </td>
                    </tr>
                  ))}
                </tbody>
              </table>
            )}
          </div>
        </div>
      </div>

      <NovoTenantModal open={novo} onClose={() => setNovo(false)} onCreated={load} />
      <ModulosModal tenant={modulos} onClose={() => setModulos(null)} onSaved={load} />
      <PlanoModal tenant={plano} onClose={() => setPlano(null)} onSaved={load} />
      <MetaModal tenant={meta} onClose={() => setMeta(null)} onSaved={load} />
    </>
  );
}

function MetaStatus({ tenant }: { tenant: ApiTenant }) {
  // Tenant 1 usa as credenciais do .env (plataforma).
  if (tenant.id === 1) return <span className="faint" style={{ fontSize: 12 }}>.env (plataforma)</span>;
  return <span className="faint" style={{ fontSize: 12 }}>configurar →</span>;
}

function NovoTenantModal({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: () => void }) {
  const toast = useToast();
  const [f, setF] = useState({ nome: "", subdominio: "", plano: "Gov Pro", admin_email: "", admin_senha: "" });
  const [saving, setSaving] = useState(false);
  useEffect(() => { if (open) setF({ nome: "", subdominio: "", plano: "Gov Pro", admin_email: "", admin_senha: "" }); }, [open]);

  async function salvar() {
    if (!f.nome || !f.subdominio) return;
    setSaving(true);
    try {
      await createTenant(f);
      toast({ type: "success", title: "Órgão criado", message: f.subdominio + ".zapgov.com.br" });
      onCreated();
      onClose();
    } catch (e) { toast({ type: "error", title: "Erro", message: (e as Error).message }); }
    finally { setSaving(false); }
  }

  return (
    <Modal open={open} onClose={onClose} width={480}>
      <div className="modal-head"><div className="modal-title">Novo órgão</div><div className="modal-sub">Cria o tenant e o administrador inicial.</div></div>
      <div className="modal-body col" style={{ gap: 14 }}>
        <Field label="Nome do órgão"><Input value={f.nome} onChange={(e) => setF({ ...f, nome: e.target.value })} /></Field>
        <Field label="Subdomínio" hint="Usado no login: <subdomínio>.zapgov.com.br">
          <Input value={f.subdominio} onChange={(e) => setF({ ...f, subdominio: e.target.value.toLowerCase().replace(/[^a-z0-9-]/g, "") })} placeholder="prefeitura-x" />
        </Field>
        <Field label="Plano">
          <select className="select" value={f.plano} onChange={(e) => setF({ ...f, plano: e.target.value })}>
            {PLANOS.map((p) => <option key={p}>{p}</option>)}
          </select>
        </Field>
        <div className="row" style={{ gap: 12 }}>
          <Field label="E-mail do admin" hint="Opcional. Padrão: admin@zapgov.local">
            <Input type="email" value={f.admin_email} onChange={(e) => setF({ ...f, admin_email: e.target.value })} />
          </Field>
          <Field label="Senha provisória" hint="Opcional. Padrão: admin123">
            <Input type="password" value={f.admin_senha} onChange={(e) => setF({ ...f, admin_senha: e.target.value })} />
          </Field>
        </div>
      </div>
      <div className="modal-foot">
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="primary" icon="check" aria-disabled={saving || !f.nome || !f.subdominio} onClick={salvar}>{saving ? "Criando…" : "Criar órgão"}</Button>
      </div>
    </Modal>
  );
}

function ModulosModal({ tenant, onClose, onSaved }: { tenant: ApiTenant | null; onClose: () => void; onSaved: () => void }) {
  const toast = useToast();
  const [sel, setSel] = useState<Record<string, boolean>>({});
  const [saving, setSaving] = useState(false);
  useEffect(() => { if (tenant) setSel({ ...(tenant.modulos ?? {}) }); }, [tenant]);

  async function salvar() {
    if (!tenant) return;
    setSaving(true);
    try {
      await updateTenantModulos(tenant.id, sel);
      toast({ type: "success", title: "Módulos atualizados", message: "O atendente precisa sair e entrar de novo para ver a mudança." });
      onSaved(); onClose();
    } catch (e) { toast({ type: "error", title: "Erro", message: (e as Error).message }); }
    finally { setSaving(false); }
  }

  return (
    <Modal open={!!tenant} onClose={onClose} width={460}>
      <div className="modal-head">
        <div className="modal-title">Módulos do órgão</div>
        <div className="modal-sub">{tenant?.nome} — liga/desliga funcionalidades neste painel</div>
      </div>
      <div className="modal-body col" style={{ gap: 10 }}>
        {MODULOS.map((mod) => (
          <label key={mod.key} className="card card-pad row between center" style={{ gap: 12, cursor: "pointer" }}>
            <div className="col" style={{ gap: 2 }}>
              <span className="strong" style={{ fontSize: 13.5 }}>{mod.label}</span>
              <span className="faint" style={{ fontSize: 11.5 }}>{mod.hint}</span>
            </div>
            <input
              type="checkbox"
              checked={!!sel[mod.key]}
              onChange={(e) => setSel({ ...sel, [mod.key]: e.target.checked })}
            />
          </label>
        ))}
      </div>
      <div className="modal-foot">
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="primary" icon="check" aria-disabled={saving} onClick={salvar}>{saving ? "Salvando…" : "Salvar"}</Button>
      </div>
    </Modal>
  );
}

function PlanoModal({ tenant, onClose, onSaved }: { tenant: ApiTenant | null; onClose: () => void; onSaved: () => void }) {
  const toast = useToast();
  const [plano, setPlano] = useState("Gov Pro");
  const [limite, setLimite] = useState(250000);
  const [saving, setSaving] = useState(false);
  useEffect(() => { if (tenant) { setPlano(tenant.plano); setLimite(tenant.limite_mensal); } }, [tenant]);

  async function salvar() {
    if (!tenant) return;
    setSaving(true);
    try {
      await updateTenantPlano(tenant.id, plano, limite);
      toast({ type: "success", title: "Plano atualizado" });
      onSaved(); onClose();
    } catch (e) { toast({ type: "error", title: "Erro", message: (e as Error).message }); }
    finally { setSaving(false); }
  }

  return (
    <Modal open={!!tenant} onClose={onClose} width={420}>
      <div className="modal-head"><div className="modal-title">Plano e limite</div><div className="modal-sub">{tenant?.nome}</div></div>
      <div className="modal-body col" style={{ gap: 14 }}>
        <Field label="Plano">
          <select className="select" value={plano} onChange={(e) => setPlano(e.target.value)}>
            {PLANOS.map((p) => <option key={p}>{p}</option>)}
          </select>
        </Field>
        <Field label="Limite mensal de mensagens">
          <Input type="number" value={String(limite)} onChange={(e) => setLimite(Number(e.target.value) || 0)} />
        </Field>
      </div>
      <div className="modal-foot">
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="primary" icon="check" aria-disabled={saving} onClick={salvar}>{saving ? "Salvando…" : "Salvar"}</Button>
      </div>
    </Modal>
  );
}

function MetaModal({ tenant, onClose, onSaved }: { tenant: ApiTenant | null; onClose: () => void; onSaved: () => void }) {
  const toast = useToast();
  const empty: TenantMeta = { phone_number_id: "", waba_id: "", token: "", app_secret: "", api_version: "v21.0", webhook_verify_token: "" };
  const [m, setM] = useState<TenantMeta>(empty);
  const [saving, setSaving] = useState(false);
  useEffect(() => { if (tenant) setM(empty); /* não retornamos os segredos atuais por segurança */ }, [tenant]); // eslint-disable-line

  async function salvar() {
    if (!tenant) return;
    setSaving(true);
    try {
      await setTenantMeta(tenant.id, m);
      toast({ type: "success", title: "Credenciais salvas", message: "A API deste órgão foi atualizada." });
      onSaved(); onClose();
    } catch (e) { toast({ type: "error", title: "Erro", message: (e as Error).message }); }
    finally { setSaving(false); }
  }

  return (
    <Modal open={!!tenant} onClose={onClose} width={520}>
      <div className="modal-head">
        <div className="modal-title">Credenciais da API Meta</div>
        <div className="modal-sub">{tenant?.nome} — WhatsApp Business (Cloud API)</div>
      </div>
      <div className="modal-body col" style={{ gap: 12 }}>
        {tenant?.id === 1 && (
          <div className="card card-pad" style={{ background: "var(--st-pending-soft)", fontSize: 12.5, color: "var(--text-secondary)" }}>
            Este é o tenant da plataforma — por padrão usa as credenciais do <b>.env</b>. Preencher aqui sobrescreve.
          </div>
        )}
        <div className="row" style={{ gap: 12 }}>
          <Field label="Phone Number ID"><Input value={m.phone_number_id} onChange={(e) => setM({ ...m, phone_number_id: e.target.value.trim() })} /></Field>
          <Field label="WABA ID"><Input value={m.waba_id} onChange={(e) => setM({ ...m, waba_id: e.target.value.trim() })} /></Field>
        </div>
        <Field label="Token de acesso (System User)" hint="Token permanente da Meta.">
          <Input type="password" value={m.token} onChange={(e) => setM({ ...m, token: e.target.value.trim() })} />
        </Field>
        <div className="row" style={{ gap: 12 }}>
          <Field label="App Secret" hint="Valida o HMAC do webhook.">
            <Input type="password" value={m.app_secret} onChange={(e) => setM({ ...m, app_secret: e.target.value.trim() })} />
          </Field>
          <Field label="Versão da API"><Input value={m.api_version} onChange={(e) => setM({ ...m, api_version: e.target.value.trim() })} /></Field>
        </div>
        <Field label="Webhook Verify Token" hint="Opcional, se o órgão usar app Meta próprio.">
          <Input value={m.webhook_verify_token} onChange={(e) => setM({ ...m, webhook_verify_token: e.target.value.trim() })} />
        </Field>
      </div>
      <div className="modal-foot">
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="primary" icon="check" aria-disabled={saving || !m.phone_number_id || !m.token} onClick={salvar}>{saving ? "Salvando…" : "Salvar credenciais"}</Button>
      </div>
    </Modal>
  );
}
