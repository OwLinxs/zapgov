"use client";

import { useCallback, useEffect, useRef, useState } from "react";
import { Topbar } from "@/components/shell/Topbar";
import {
  Avatar,
  Button,
  Drawer,
  EmptyState,
  Field,
  Icon,
  Input,
  Modal,
  Skel,
  useToast,
} from "@/components/ui";
import { SETORES, SETOR_COR } from "@/lib/data";
import { listContacts, createContact, importContacts, type ApiContact } from "@/lib/api";

function fmtData(iso: string | null): string {
  if (!iso) return "—";
  return new Date(iso).toLocaleDateString("pt-BR", { day: "2-digit", month: "2-digit", year: "numeric" });
}

function NovoContatoModal({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: () => void }) {
  const toast = useToast();
  const [f, setF] = useState({ nome: "", telefone: "", cpf: "", setor: "Geral", tags: "" });
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (open) setF({ nome: "", telefone: "", cpf: "", setor: "Geral", tags: "" });
  }, [open]);

  async function salvar() {
    if (!f.nome.trim() || !f.telefone.trim()) return;
    setSaving(true);
    try {
      await createContact({
        nome: f.nome,
        telefone: f.telefone,
        cpf: f.cpf,
        setor: f.setor,
        tags: f.tags ? f.tags.split(",").map((t) => t.trim()).filter(Boolean) : [],
      });
      toast({ type: "success", title: "Contato criado", message: f.nome });
      onCreated();
      onClose();
    } catch (e) {
      toast({ type: "error", title: "Não foi possível criar", message: (e as Error).message });
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal open={open} onClose={onClose} width={460}>
      <div className="modal-head">
        <div className="modal-title">Novo contato</div>
        <div className="modal-sub">Telefone no formato E.164 (ex: 5546991393338).</div>
      </div>
      <div className="modal-body col" style={{ gap: 14 }}>
        <Field label="Nome">
          <Input value={f.nome} onChange={(e) => setF({ ...f, nome: e.target.value })} placeholder="Nome completo" />
        </Field>
        <div className="row" style={{ gap: 12 }}>
          <Field label="Telefone">
            <Input icon="phone" value={f.telefone} onChange={(e) => setF({ ...f, telefone: e.target.value })} placeholder="5546991393338" />
          </Field>
          <Field label="CPF">
            <Input value={f.cpf} onChange={(e) => setF({ ...f, cpf: e.target.value })} placeholder="000.000.000-00" />
          </Field>
        </div>
        <div className="row" style={{ gap: 12 }}>
          <Field label="Setor">
            <select className="select" value={f.setor} onChange={(e) => setF({ ...f, setor: e.target.value })}>
              {SETORES.map((s) => (
                <option key={s}>{s}</option>
              ))}
            </select>
          </Field>
          <Field label="Tags (separadas por vírgula)">
            <Input value={f.tags} onChange={(e) => setF({ ...f, tags: e.target.value })} placeholder="VIP, Optin" />
          </Field>
        </div>
      </div>
      <div className="modal-foot">
        <Button variant="ghost" onClick={onClose}>
          Cancelar
        </Button>
        <Button variant="primary" icon="check" aria-disabled={saving || !f.nome || !f.telefone} onClick={salvar}>
          {saving ? "Salvando…" : "Criar contato"}
        </Button>
      </div>
    </Modal>
  );
}

export default function ContatosPage() {
  const toast = useToast();
  const [q, setQ] = useState("");
  const [setor, setSetor] = useState("Todos");
  const [active, setActive] = useState<ApiContact | null>(null);
  const [novoOpen, setNovoOpen] = useState(false);
  const [list, setList] = useState<ApiContact[]>([]);
  const [loading, setLoading] = useState(true);
  const [importando, setImportando] = useState(false);
  const [erro, setErro] = useState<string | null>(null);
  const fileRef = useRef<HTMLInputElement>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setList(await listContacts());
      setErro(null);
    } catch (e) {
      setErro((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, []);

  useEffect(() => {
    load();
  }, [load]);

  async function onFile(e: React.ChangeEvent<HTMLInputElement>) {
    const file = e.target.files?.[0];
    e.target.value = ""; // permite reimportar o mesmo arquivo
    if (!file) return;
    setImportando(true);
    try {
      const r = await importContacts(file);
      toast({
        type: "success",
        title: "Importação concluída",
        message: `${r.inseridos} novos · ${r.pulados} já existiam · ${r.invalidos} inválidos`,
      });
      load();
    } catch (err) {
      toast({ type: "error", title: "Falha na importação", message: (err as Error).message });
    } finally {
      setImportando(false);
    }
  }

  const rows = list.filter(
    (c) =>
      (setor === "Todos" || c.setor === setor) &&
      (!q || c.nome.toLowerCase().includes(q.toLowerCase()) || c.cpf.includes(q) || c.telefone.includes(q)),
  );

  return (
    <div className="split">
      <div className="grow col" style={{ minWidth: 0, height: "100%" }}>
        <Topbar
          title="Contatos"
          sub={list.length.toLocaleString("pt-BR") + " cadastrados"}
          actions={
            <>
              <input ref={fileRef} type="file" accept=".csv,text/csv" hidden onChange={onFile} />
              <Button variant="secondary" icon="upload" onClick={() => fileRef.current?.click()} aria-disabled={importando}>
                {importando ? "Importando…" : "Importar CSV"}
              </Button>
              <Button variant="secondary" icon="refresh" onClick={load}>
                Atualizar
              </Button>
              <Button variant="primary" icon="plus" onClick={() => setNovoOpen(true)}>
                Novo contato
              </Button>
            </>
          }
        />
        <div className="content">
          <div className="content-pad col" style={{ gap: 14 }}>
            <div className="card card-pad">
              <div className="toolbar">
                <div className="toolbar-search">
                  <Input icon="search" placeholder="Buscar por nome, CPF ou telefone…" value={q} onChange={(e) => setQ(e.target.value)} />
                </div>
                <select className="select" style={{ width: "auto" }} value={setor} onChange={(e) => setSetor(e.target.value)}>
                  {["Todos", ...SETORES].map((s) => (
                    <option key={s}>{s}</option>
                  ))}
                </select>
                <div className="grow" />
                <span className="muted" style={{ fontSize: 12.5 }}>
                  {rows.length} contatos
                </span>
              </div>
            </div>

            <div className="card" style={{ overflow: "hidden" }}>
              {erro ? (
                <EmptyState icon="alertCircle" title="API indisponível" sub={erro} action={<Button size="sm" onClick={load}>Tentar de novo</Button>} />
              ) : loading ? (
                <div className="card-pad col" style={{ gap: 10 }}>
                  {[0, 1, 2, 3].map((i) => (
                    <Skel key={i} h={28} />
                  ))}
                </div>
              ) : rows.length === 0 ? (
                <EmptyState icon="contacts" title="Nenhum contato" sub="Crie um contato ou ajuste o filtro." />
              ) : (
                <table className="ds-table">
                  <thead>
                    <tr>
                      <th>Nome</th>
                      <th>Telefone</th>
                      <th>CPF</th>
                      <th>Setor</th>
                      <th>Tags</th>
                      <th>Status</th>
                      <th className="num">Mensagens</th>
                      <th>Último contato</th>
                    </tr>
                  </thead>
                  <tbody>
                    {rows.map((c) => (
                      <tr key={c.id} onClick={() => setActive(c)} style={{ cursor: "pointer" }}>
                        <td className="strong">
                          <div className="row center" style={{ gap: 10 }}>
                            <Avatar name={c.nome} size={30} />
                            {c.nome}
                          </div>
                        </td>
                        <td className="mono" style={{ fontSize: 12 }}>
                          {c.telefone}
                        </td>
                        <td className="mono" style={{ fontSize: 12 }}>
                          {c.cpf || "—"}
                        </td>
                        <td>
                          <span className="tag">
                            <span className="tag-dot" style={{ background: SETOR_COR[c.setor] || "var(--text-faint)" }} /> {c.setor}
                          </span>
                        </td>
                        <td>
                          <div className="row" style={{ gap: 4 }}>
                            {c.tags?.length ? (
                              c.tags.map((t) => (
                                <span key={t} className="badge badge-neutral">
                                  {t}
                                </span>
                              ))
                            ) : (
                              <span className="faint">—</span>
                            )}
                          </div>
                        </td>
                        <td>
                          <span className={"badge " + (c.status === "ativo" ? "badge-read" : c.status === "sem_whatsapp" ? "badge-pending" : "badge-failed")}>
                            <span className="dot" /> {c.status === "ativo" ? "Ativo" : c.status === "sem_whatsapp" ? "Sem WhatsApp" : "Bloqueado"}
                          </span>
                        </td>
                        <td className="num mono">{c.mensagens}</td>
                        <td className="muted" style={{ fontSize: 12 }}>
                          {fmtData(c.ultimo_contato)}
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              )}
            </div>
          </div>
        </div>
      </div>

      {/* Drawer de perfil */}
      <Drawer open={!!active} onClose={() => setActive(null)} width={420}>
        {active && (
          <>
            <div className="row between center" style={{ padding: "16px 20px", borderBottom: "1px solid var(--border-subtle)" }}>
              <div className="card-title">Perfil do contato</div>
              <button className="btn btn-ghost btn-icon btn-sm" onClick={() => setActive(null)}>
                <Icon name="x" size={16} />
              </button>
            </div>
            <div className="scroll-y grow" style={{ padding: 20 }}>
              <div className="col center" style={{ gap: 10, paddingBottom: 18, borderBottom: "1px solid var(--border-subtle)" }}>
                <Avatar name={active.nome} size={64} />
                <div className="col center" style={{ gap: 2 }}>
                  <div style={{ fontSize: 17, fontWeight: 700 }}>{active.nome}</div>
                  <span className="mono" style={{ fontSize: 12.5, color: "var(--text-tertiary)" }}>
                    {active.telefone}
                  </span>
                </div>
              </div>
              <div className="col" style={{ gap: 0, paddingTop: 16 }}>
                {(
                  [
                    ["CPF", active.cpf || "—", true],
                    ["Setor", active.setor, false],
                    ["Status", active.status === "ativo" ? "Ativo" : active.status === "sem_whatsapp" ? "Sem WhatsApp" : "Bloqueado", false],
                    ["Total de mensagens", String(active.mensagens), false],
                    ["Último contato", fmtData(active.ultimo_contato), false],
                    ["Cadastrado em", fmtData(active.criado_em), false],
                  ] as [string, string, boolean][]
                ).map((r, i) => (
                  <div key={i} className="row between center" style={{ padding: "11px 0", borderBottom: "1px solid var(--border-subtle)" }}>
                    <span className="muted" style={{ fontSize: 12.5 }}>
                      {r[0]}
                    </span>
                    <span className={(r[2] ? "mono " : "") + "strong"} style={{ fontSize: 13 }}>
                      {r[1]}
                    </span>
                  </div>
                ))}
                {active.tags?.length ? (
                  <div style={{ marginTop: 16 }}>
                    <div className="field-label" style={{ marginBottom: 8 }}>
                      Tags
                    </div>
                    <div className="row wrap" style={{ gap: 6 }}>
                      {active.tags.map((t) => (
                        <span key={t} className="badge badge-brand">
                          {t}
                        </span>
                      ))}
                    </div>
                  </div>
                ) : null}
              </div>
            </div>
          </>
        )}
      </Drawer>

      <NovoContatoModal open={novoOpen} onClose={() => setNovoOpen(false)} onCreated={load} />
    </div>
  );
}
