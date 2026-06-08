"use client";

import { useCallback, useEffect, useState } from "react";
import { Topbar } from "@/components/shell/Topbar";
import {
  Button,
  Dropdown,
  EmptyState,
  Field,
  Icon,
  Input,
  Modal,
  Skel,
  StatusBadge,
  Tabs,
  WaBubble,
  useToast,
} from "@/components/ui";
import { listTemplates, createTemplate, type ApiTemplate } from "@/lib/api";

const CAT_CODE: Record<string, string> = { Utilidade: "UTILITY", Marketing: "MARKETING", Autenticação: "AUTHENTICATION" };

function NovoTemplateModal({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: () => void }) {
  const toast = useToast();
  const [nome, setNome] = useState("");
  const [cat, setCat] = useState("Utilidade");
  const [idioma, setIdioma] = useState("pt_BR");
  const [corpo, setCorpo] = useState("Olá {{1}}, ");
  const [botoes, setBotoes] = useState("");
  const [exemplos, setExemplos] = useState<Record<string, string>>({});
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (open) {
      setNome("");
      setCat("Utilidade");
      setIdioma("pt_BR");
      setCorpo("Olá {{1}}, ");
      setBotoes("");
      setExemplos({});
    }
  }, [open]);

  const botoesList = botoes.split(",").map((b) => b.trim()).filter(Boolean);
  // Variáveis usadas no corpo ({{1}}, {{2}}…), em ordem.
  const varNums = Array.from(new Set((corpo.match(/\{\{\s*(\d+)\s*\}\}/g) || []).map((m) => m.replace(/\D/g, "")))).sort((a, b) => +a - +b);
  const exemplosOk = varNums.every((n) => exemplos[n]?.trim());

  async function salvar() {
    if (!nome.trim() || !corpo.trim()) return;
    if (!exemplosOk) {
      toast({ type: "error", title: "Faltam exemplos", message: "A Meta exige um exemplo para cada variável." });
      return;
    }
    setSaving(true);
    try {
      await createTemplate({
        nome,
        idioma,
        categoria: CAT_CODE[cat] || "UTILITY",
        corpo,
        botoes: botoesList,
        exemplos: varNums.map((n) => exemplos[n] || ""),
      });
      toast({ type: "success", title: "Enviado para aprovação", message: "A Meta responde em até 24h. Só Utilidade aprovada vira disparável." });
      onCreated();
      onClose();
    } catch (e) {
      toast({ type: "error", title: "Não foi possível criar", message: (e as Error).message });
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal open={open} onClose={onClose} width={720}>
      <div className="modal-head">
        <div className="modal-title">Novo template</div>
        <div className="modal-sub">Enviado à Meta para aprovação. Apenas templates aprovados como Utilidade podem ser disparados.</div>
      </div>
      <div className="modal-body" style={{ display: "grid", gridTemplateColumns: "1fr 280px", gap: 20 }}>
        <div className="col" style={{ gap: 14 }}>
          <div className="row" style={{ gap: 12 }}>
            <Field label="Nome" hint="vira minúsculo com _ (ex: aviso_consulta)">
              <Input value={nome} onChange={(e) => setNome(e.target.value)} placeholder="aviso consulta" />
            </Field>
            <Field label="Categoria">
              <select className="select" value={cat} onChange={(e) => setCat(e.target.value)}>
                {["Utilidade", "Marketing", "Autenticação"].map((c) => (
                  <option key={c}>{c}</option>
                ))}
              </select>
            </Field>
          </div>
          <Field label="Idioma">
            <select className="select" value={idioma} onChange={(e) => setIdioma(e.target.value)}>
              <option value="pt_BR">Português (Brasil)</option>
              <option value="en_US">Inglês</option>
            </select>
          </Field>
          <Field label="Corpo da mensagem" hint="Use {{1}}, {{2}}… para variáveis dinâmicas.">
            <textarea className="textarea" style={{ minHeight: 120 }} value={corpo} onChange={(e) => setCorpo(e.target.value)} />
          </Field>
          <div className="row">
            <Button variant="secondary" size="sm" onClick={() => setCorpo((c) => c + "{{" + ((c.match(/\{\{/g) || []).length + 1) + "}}")}>
              + Variável
            </Button>
          </div>

          {varNums.length > 0 && (
            <div className="col" style={{ gap: 10 }}>
              <span className="field-label">Exemplos das variáveis (obrigatório p/ aprovação)</span>
              {varNums.map((n) => (
                <div key={n} className="row center" style={{ gap: 8 }}>
                  <span className="wa-var" style={{ flex: "none", minWidth: 34, textAlign: "center" }}>{"{{" + n + "}}"}</span>
                  <Input value={exemplos[n] || ""} onChange={(e) => setExemplos((v) => ({ ...v, [n]: e.target.value }))} placeholder="ex: João" />
                </div>
              ))}
            </div>
          )}

          <Field label="Botões (opcional)" hint="Rótulos separados por vírgula. Até 10 respostas rápidas.">
            <Input value={botoes} onChange={(e) => setBotoes(e.target.value)} placeholder="Confirmar, Reagendar" />
          </Field>
        </div>
        <div className="col" style={{ gap: 8 }}>
          <div className="field-label">Pré-visualização</div>
          <div className="wa-canvas" style={{ flex: 1 }}>
            <WaBubble text={corpo || "Sua mensagem aparecerá aqui…"} dir="out" highlight status="lido" time="09:14" buttons={botoesList.length ? botoesList : undefined} />
          </div>
        </div>
      </div>
      <div className="modal-foot">
        <Button variant="ghost" onClick={onClose}>
          Cancelar
        </Button>
        <Button variant="primary" aria-disabled={saving || !nome || !corpo || !exemplosOk} onClick={salvar}>
          {saving ? "Enviando…" : "Enviar para aprovação"}
        </Button>
      </div>
    </Modal>
  );
}

export default function TemplatesPage() {
  const toast = useToast();
  const [tab, setTab] = useState("todos");
  const [novo, setNovo] = useState(false);
  const [tpls, setTpls] = useState<ApiTemplate[]>([]);
  const [loading, setLoading] = useState(true);
  const [erro, setErro] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setTpls(await listTemplates());
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

  const counts = {
    todos: tpls.length,
    aprovado: tpls.filter((t) => t.status === "aprovado").length,
    pendente: tpls.filter((t) => t.status === "pendente").length,
    rejeitado: tpls.filter((t) => t.status === "rejeitado").length,
  };
  const rows = tpls.filter((t) => tab === "todos" || t.status === tab);

  return (
    <>
      <Topbar
        title="Templates"
        sub="Sincronizados da sua conta WhatsApp Business (Meta)"
        actions={
          <>
            <Button variant="secondary" icon="refresh" onClick={() => { load(); toast({ type: "info", title: "Sincronizando…" }); }}>
              Sincronizar
            </Button>
            <Button variant="primary" icon="plus" onClick={() => setNovo(true)}>
              Novo template
            </Button>
          </>
        }
      />
      <div className="content">
        <div className="content-pad col" style={{ gap: 16 }}>
          <Tabs
            value={tab}
            onChange={setTab}
            tabs={[
              { id: "todos", label: "Todos", count: counts.todos },
              { id: "aprovado", label: "Aprovados", count: counts.aprovado },
              { id: "pendente", label: "Pendentes", count: counts.pendente },
              { id: "rejeitado", label: "Rejeitados", count: counts.rejeitado },
            ]}
          />

          {erro ? (
            <div className="card">
              <EmptyState icon="alertCircle" title="Não foi possível sincronizar" sub={erro} action={<Button size="sm" onClick={load}>Tentar de novo</Button>} />
            </div>
          ) : loading ? (
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(330px, 1fr))", gap: 14 }}>
              {[0, 1, 2].map((i) => (
                <div key={i} className="card card-pad col" style={{ gap: 12 }}>
                  <Skel w="60%" h={16} />
                  <Skel h={100} />
                  <Skel w="40%" h={12} />
                </div>
              ))}
            </div>
          ) : rows.length === 0 ? (
            <div className="card">
              <EmptyState icon="template" title="Nenhum template" sub="Crie um template no Gerenciador da Meta e sincronize aqui." />
            </div>
          ) : (
            <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(330px, 1fr))", gap: 14 }}>
              {rows.map((t) => (
                <div key={t.name + t.language} className="card" style={{ display: "flex", flexDirection: "column" }}>
                  <div className="card-pad col" style={{ gap: 12, flex: 1 }}>
                    <div className="row between" style={{ alignItems: "flex-start" }}>
                      <div className="col" style={{ gap: 4 }}>
                        <span className="strong" style={{ fontSize: 14 }}>
                          {t.name}
                        </span>
                        <div className="row center" style={{ gap: 6 }}>
                          <span className="badge badge-neutral">{t.category}</span>
                          <span className="mono faint" style={{ fontSize: 11 }}>
                            {t.language}
                          </span>
                        </div>
                      </div>
                      <StatusBadge status={t.status} />
                    </div>
                    <div className="wa-canvas" style={{ flex: 1, minHeight: 120 }}>
                      {t.header && (
                        <div className="wa-row">
                          <div className="wa-bubble in" style={{ fontWeight: 700, fontSize: 12 }}>
                            {t.header}
                          </div>
                        </div>
                      )}
                      <WaBubble text={t.body || "—"} dir="out" highlight status="lido" time="09:14" buttons={t.buttons?.length ? t.buttons : undefined} />
                    </div>
                    {t.status === "rejeitado" && t.motivo && (
                      <div className="row" style={{ gap: 8, padding: "8px 10px", borderRadius: 8, background: "var(--st-failed-soft)", fontSize: 11.5, color: "var(--st-failed)" }}>
                        <Icon name="alertCircle" size={14} style={{ flex: "none", marginTop: 1 }} />
                        <span>
                          <strong>Motivo: </strong>
                          {t.motivo}
                        </span>
                      </div>
                    )}
                  </div>
                  <div className="row between center" style={{ padding: "11px 16px", borderTop: "1px solid var(--border-subtle)" }}>
                    <div className="mono faint" style={{ fontSize: 11 }}>
                      {t.variables} {t.variables === 1 ? "variável" : "variáveis"}
                    </div>
                    <Dropdown
                      trigger={
                        <button className="btn btn-ghost btn-icon btn-sm">
                          <Icon name="more" size={15} />
                        </button>
                      }
                    >
                      <div className="dropdown-item">
                        <Icon name="send" size={15} /> Usar em disparo
                      </div>
                      <div className="dropdown-item">
                        <Icon name="copy" size={15} /> Copiar nome
                      </div>
                    </Dropdown>
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>
      <NovoTemplateModal open={novo} onClose={() => setNovo(false)} onCreated={load} />
    </>
  );
}
