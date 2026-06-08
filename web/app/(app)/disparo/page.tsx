"use client";

import { Fragment, useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Topbar } from "@/components/shell/Topbar";
import {
  Avatar,
  Button,
  ConfirmModal,
  EmptyState,
  Field,
  Icon,
  Input,
  Skel,
  WaBubble,
  useToast,
} from "@/components/ui";
import { SETORES, SETOR_COR, TENANT } from "@/lib/data";
import { listTemplates, listContacts, createCampaign, type ApiTemplate, type ApiContact } from "@/lib/api";

const CUSTO_UNIT = 0.04;
const ALL_TAGS = ["Todas", "VIP", "Optin", "Idoso", "Empresa"];
const VAR_LABELS: Record<string, string> = {
  "1": "Nome do contato",
  "2": "Inscrição / referência",
  "3": "Data / valor",
};

function brl(v: number) {
  return "R$ " + v.toFixed(2).replace(".", ",");
}

export default function DisparoPage() {
  const router = useRouter();
  const toast = useToast();
  const [aprovados, setAprovados] = useState<ApiTemplate[]>([]);
  const [loadingTpls, setLoadingTpls] = useState(true);
  const [contatos, setContatos] = useState<ApiContact[]>([]);
  const [step, setStep] = useState(1);
  const [tpl, setTpl] = useState<ApiTemplate | null>(null);
  const [vars, setVars] = useState<Record<string, string>>({});
  const [selSetor, setSelSetor] = useState("Todos");
  const [selTag, setSelTag] = useState("Todas");
  const [q, setQ] = useState("");
  const [sel, setSel] = useState<Set<number>>(() => new Set());
  const [quando, setQuando] = useState("agora");
  const [data, setData] = useState("2026-06-05");
  const [hora, setHora] = useState("09:00");
  const [confirm, setConfirm] = useState(false);
  const [disparando, setDisparando] = useState(false);

  // Só templates APROVADOS podem ser disparados em massa (regra da Meta).
  useEffect(() => {
    listTemplates()
      .then((all) => {
        // Só dispara o que está aprovado E na categoria Utilidade (regra da Meta).
        const ap = all.filter((t) => t.status === "aprovado" && t.category === "Utilidade");
        setAprovados(ap);
        setTpl((cur) => cur ?? ap[0] ?? null);
      })
      .catch(() => setAprovados([]))
      .finally(() => setLoadingTpls(false));
    listContacts().then(setContatos).catch(() => setContatos([]));
  }, []);

  const filtrados = contatos.filter(
    (c) =>
      (selSetor === "Todos" || c.setor === selSetor) &&
      (selTag === "Todas" || (c.tags ?? []).includes(selTag)) &&
      (!q || c.nome.toLowerCase().includes(q.toLowerCase()) || c.cpf.includes(q) || c.telefone.includes(q)),
  );
  const toggle = (id: number) =>
    setSel((s) => {
      const n = new Set(s);
      n.has(id) ? n.delete(id) : n.add(id);
      return n;
    });
  const corpoPreenchido = tpl ? tpl.body.replace(/\{\{(\d+)\}\}/g, (m, n) => vars[n] || m) : "";
  const steps = [
    { n: 1, l: "Template" },
    { n: 2, l: "Destinatários" },
    { n: 3, l: "Revisar e enviar" },
  ];
  const podeAvancar = step === 1 ? !!tpl : step === 2 ? sel.size > 0 : true;
  const custoTotal = sel.size * CUSTO_UNIT;

  const confirmar = async () => {
    if (!tpl) return;
    setDisparando(true);
    const phones = contatos.filter((c) => sel.has(c.id)).map((c) => c.telefone);
    try {
      const camp = await createCampaign({
        template: tpl.name,
        language: tpl.language,
        params: Object.keys(vars)
          .sort()
          .map((k) => vars[k]),
        setor: selSetor === "Todos" ? "Geral" : selSetor,
        operador: TENANT.operador.nome,
        preview: corpoPreenchido,
        phones,
      });
      setConfirm(false);
      toast({
        type: "success",
        title: "Disparo enfileirado",
        message: `${camp.total} mensagens na fila — acompanhe o progresso no histórico`,
      });
      router.push("/historico");
    } catch (e) {
      toast({ type: "error", title: "Falha no disparo", message: (e as Error).message });
    } finally {
      setDisparando(false);
    }
  };

  return (
    <>
      <Topbar
        title="Novo Disparo"
        breadcrumb={["Disparos", "Novo"]}
        actions={
          <div className="stepper">
            {steps.map((s, i) => (
              <Fragment key={s.n}>
                {i > 0 && <div className={"step-line" + (step > i ? " done" : "")} />}
                <div className={"step" + (step === s.n ? " active" : step > s.n ? " done" : "")}>
                  <span className="step-dot">{step > s.n ? <Icon name="check" size={14} /> : s.n}</span>
                  <span className="step-label">{s.l}</span>
                </div>
              </Fragment>
            ))}
          </div>
        }
      />

      <div className="content">
        <div className="content-pad" style={{ maxWidth: 1080, margin: "0 auto", width: "100%" }}>
          {/* STEP 1 — Template */}
          {step === 1 && (
            <div className="grid-2 fade-in" style={{ gridTemplateColumns: "1.4fr 1fr", alignItems: "start" }}>
              <div className="col" style={{ gap: 16 }}>
                <div className="card">
                  <div className="card-head">
                    <div className="card-title">Selecione um template aprovado</div>
                    <span className="badge badge-read">{aprovados.length} disponíveis</span>
                  </div>
                  <div className="col" style={{ padding: 8, gap: 4 }}>
                    {loadingTpls ? (
                      [0, 1, 2].map((i) => (
                        <div key={i} style={{ padding: 12 }}>
                          <Skel w="50%" h={14} />
                        </div>
                      ))
                    ) : aprovados.length === 0 ? (
                      <EmptyState icon="template" title="Nenhum template aprovado" sub="Aprove um template na Meta para disparar." />
                    ) : (
                      aprovados.map((t) => {
                        const isSel = tpl?.name === t.name && tpl?.language === t.language;
                        return (
                          <div
                            key={t.name + t.language}
                            onClick={() => {
                              setTpl(t);
                              setVars({});
                            }}
                            style={{
                              display: "flex",
                              gap: 12,
                              padding: 12,
                              borderRadius: 9,
                              cursor: "pointer",
                              border: "1px solid " + (isSel ? "var(--brand)" : "transparent"),
                              background: isSel ? "var(--brand-soft)" : "transparent",
                            }}
                          >
                            <span className="kpi-ico" style={{ background: "var(--bg-surface-2)", color: "var(--text-tertiary)", width: 34, height: 34 }}>
                              <Icon name="template" size={16} />
                            </span>
                            <div className="col grow" style={{ gap: 2 }}>
                              <div className="row center" style={{ gap: 8 }}>
                                <span className="strong" style={{ fontSize: 13.5 }}>
                                  {t.name}
                                </span>
                                <span className="badge badge-neutral">{t.category}</span>
                              </div>
                              <div className="mono" style={{ fontSize: 11.5, color: "var(--text-faint)" }}>
                                {t.language + " · " + t.variables + " variáveis"}
                              </div>
                            </div>
                            {isSel && <Icon name="checkCircle" size={18} style={{ color: "var(--brand-text)" }} />}
                          </div>
                        );
                      })
                    )}
                  </div>
                </div>
                {tpl && tpl.variables > 0 && (
                  <div className="card">
                    <div className="card-head">
                      <div className="card-title">Preencher variáveis dinâmicas</div>
                    </div>
                    <div className="card-pad col" style={{ gap: 13 }}>
                      {Array.from({ length: tpl.variables }).map((_, i) => {
                        const n = String(i + 1);
                        return (
                          <Field
                            key={n}
                            label={
                              <span>
                                <span className="wa-var" style={{ marginRight: 6 }}>
                                  {"{{" + n + "}}"}
                                </span>
                                {VAR_LABELS[n] || "Variável " + n}
                              </span>
                            }
                          >
                            <div className="row center" style={{ gap: 8 }}>
                              <Input
                                placeholder={n === "1" ? "Ex: usar {{nome}} do contato" : "Valor fixo ou {{campo}}"}
                                value={vars[n] || ""}
                                onChange={(e) => setVars((v) => ({ ...v, [n]: e.target.value }))}
                              />
                              {n === "1" && (
                                <Button variant="secondary" size="sm" onClick={() => setVars((v) => ({ ...v, "1": "{{nome}}" }))}>
                                  Auto
                                </Button>
                              )}
                            </div>
                          </Field>
                        );
                      })}
                    </div>
                  </div>
                )}
              </div>
              <div className="card" style={{ position: "sticky", top: 0 }}>
                <div className="card-head">
                  <div className="card-title">Pré-visualização</div>
                  <span className="badge badge-wa">
                    <Icon name="phone" size={12} /> WhatsApp
                  </span>
                </div>
                <div className="card-pad">
                  {tpl ? (
                    <div className="wa-canvas">
                      {tpl.header && (
                        <div className="wa-row">
                          <div className="wa-bubble in" style={{ fontWeight: 700, paddingBottom: 4 }}>
                            <Icon name="building" size={13} style={{ marginRight: 5, verticalAlign: -2 }} />
                            {tpl.header}
                          </div>
                        </div>
                      )}
                      <WaBubble text={corpoPreenchido} dir="out" highlight status="lido" time="09:14" buttons={tpl.buttons?.length ? tpl.buttons : undefined} />
                    </div>
                  ) : (
                    <EmptyState icon="template" title="Selecione um template" />
                  )}
                  <div style={{ fontSize: 11.5, color: "var(--text-faint)", marginTop: 12, display: "flex", gap: 6 }}>
                    <Icon name="info" size={13} /> Variáveis destacadas serão substituídas por dados de cada contato no envio.
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* STEP 2 — Destinatários */}
          {step === 2 && (
            <div className="fade-in col" style={{ gap: 14 }}>
              <div className="card card-pad">
                <div className="toolbar">
                  <div className="toolbar-search">
                    <Input icon="search" placeholder="Buscar por nome, CPF ou telefone…" value={q} onChange={(e) => setQ(e.target.value)} />
                  </div>
                  <select className="select" style={{ width: "auto" }} value={selSetor} onChange={(e) => setSelSetor(e.target.value)}>
                    {["Todos", ...SETORES].map((s) => (
                      <option key={s}>{s}</option>
                    ))}
                  </select>
                  <select className="select" style={{ width: "auto" }} value={selTag} onChange={(e) => setSelTag(e.target.value)}>
                    {ALL_TAGS.map((t) => (
                      <option key={t}>{t}</option>
                    ))}
                  </select>
                  <div className="grow" />
                  <Button variant="secondary" icon="upload">
                    Importar CSV
                  </Button>
                  <Button variant="ghost" size="sm" onClick={() => setSel(new Set(filtrados.map((c) => c.id)))}>
                    Selecionar todos ({filtrados.length})
                  </Button>
                </div>
              </div>
              <div className="grid-2" style={{ gridTemplateColumns: "1.5fr 1fr", alignItems: "start" }}>
                <div className="card" style={{ overflow: "hidden" }}>
                  <div style={{ maxHeight: 440, overflowY: "auto" }}>
                    <table className="ds-table">
                      <thead>
                        <tr>
                          <th style={{ width: 40 }}></th>
                          <th>Contato</th>
                          <th>Setor</th>
                          <th>Tags</th>
                          <th>CPF</th>
                        </tr>
                      </thead>
                      <tbody>
                        {filtrados.map((c) => (
                          <tr key={c.id} className={sel.has(c.id) ? "selected" : ""} onClick={() => toggle(c.id)} style={{ cursor: "pointer" }}>
                            <td>
                              <input type="checkbox" className="checkbox" checked={sel.has(c.id)} readOnly />
                            </td>
                            <td className="strong">
                              <div className="row center" style={{ gap: 9 }}>
                                <Avatar name={c.nome} size={28} />
                                <div className="col" style={{ gap: 0 }}>
                                  <span>{c.nome}</span>
                                  <span className="mono faint" style={{ fontSize: 11 }}>
                                    {c.telefone}
                                  </span>
                                </div>
                              </div>
                            </td>
                            <td>
                              <span className="tag">
                                <span className="tag-dot" style={{ background: SETOR_COR[c.setor] }} /> {c.setor}
                              </span>
                            </td>
                            <td>
                              <div className="row" style={{ gap: 4 }}>
                                {(c.tags ?? []).slice(0, 2).map((t) => (
                                  <span key={t} className="badge badge-neutral">
                                    {t}
                                  </span>
                                ))}
                              </div>
                            </td>
                            <td className="mono" style={{ fontSize: 11.5 }}>
                              {c.cpf}
                            </td>
                          </tr>
                        ))}
                      </tbody>
                    </table>
                  </div>
                </div>
                <div className="card card-pad col" style={{ gap: 16, position: "sticky", top: 0 }}>
                  <div className="card-title">Resumo da seleção</div>
                  <div className="col center" style={{ gap: 4, padding: "8px 0" }}>
                    <span className="mono" style={{ fontSize: 40, fontWeight: 700, color: "var(--brand-text)" }}>
                      {sel.size}
                    </span>
                    <span className="muted" style={{ fontSize: 12.5 }}>
                      destinatários selecionados
                    </span>
                  </div>
                  <div className="divider" />
                  <div className="col" style={{ gap: 9 }}>
                    <div className="row between" style={{ fontSize: 13 }}>
                      <span className="muted">Custo por mensagem</span>
                      <span className="mono strong">{brl(CUSTO_UNIT)}</span>
                    </div>
                    <div className="row between" style={{ fontSize: 13 }}>
                      <span className="muted">Mensagens</span>
                      <span className="mono strong">{sel.size.toLocaleString("pt-BR")}</span>
                    </div>
                    <div className="divider" />
                    <div className="row between center">
                      <span className="strong">Custo estimado</span>
                      <span className="mono" style={{ fontSize: 20, fontWeight: 700, color: "var(--wa)" }}>
                        {brl(custoTotal)}
                      </span>
                    </div>
                  </div>
                  <div style={{ fontSize: 11, color: "var(--text-faint)", display: "flex", gap: 6, lineHeight: 1.5 }}>
                    <Icon name="info" size={13} style={{ flex: "none", marginTop: 1 }} /> Estimativa por conversa de utilidade (Meta). Valor final pode variar conforme categoria.
                  </div>
                </div>
              </div>
            </div>
          )}

          {/* STEP 3 — Revisar */}
          {step === 3 && (
            <div className="fade-in grid-2" style={{ gridTemplateColumns: "1fr 1fr", alignItems: "start" }}>
              <div className="col" style={{ gap: 14 }}>
                <div className="card">
                  <div className="card-head">
                    <div className="card-title">Quando enviar?</div>
                  </div>
                  <div className="card-pad col" style={{ gap: 12 }}>
                    <div className="row" style={{ gap: 10 }}>
                      {["agora", "agendar"].map((opt) => (
                        <button
                          key={opt}
                          onClick={() => setQuando(opt)}
                          style={{
                            flex: 1,
                            padding: 14,
                            borderRadius: 9,
                            cursor: "pointer",
                            textAlign: "left",
                            border: "1px solid " + (quando === opt ? "var(--brand)" : "var(--border)"),
                            background: quando === opt ? "var(--brand-soft)" : "var(--bg-sunken)",
                          }}
                        >
                          <div className="row center" style={{ gap: 9 }}>
                            <Icon name={opt === "agora" ? "zap" : "calendar"} size={17} style={{ color: quando === opt ? "var(--brand-text)" : "var(--text-tertiary)" }} />
                            <span className="strong" style={{ fontSize: 13.5 }}>
                              {opt === "agora" ? "Enviar agora" : "Agendar"}
                            </span>
                          </div>
                          <div className="muted" style={{ fontSize: 11.5, marginTop: 4 }}>
                            {opt === "agora" ? "Inicia imediatamente" : "Escolha data e hora"}
                          </div>
                        </button>
                      ))}
                    </div>
                    {quando === "agendar" && (
                      <div className="row" style={{ gap: 10 }}>
                        <Field label="Data">
                          <input type="date" className="input" value={data} onChange={(e) => setData(e.target.value)} />
                        </Field>
                        <Field label="Hora">
                          <input type="time" className="input" value={hora} onChange={(e) => setHora(e.target.value)} />
                        </Field>
                      </div>
                    )}
                  </div>
                </div>
                <div className="card card-pad col" style={{ gap: 11 }}>
                  <div className="card-title">Resumo final</div>
                  {(
                    [
                      ["Template", tpl ? tpl.name : "—"],
                      ["Destinatários", sel.size.toLocaleString("pt-BR") + " contatos"],
                      ["Custo estimado", brl(custoTotal)],
                      ["Operador", TENANT.operador.nome],
                      ["Envio", quando === "agora" ? "Imediato" : data.split("-").reverse().join("/") + " às " + hora],
                    ] as [string, string][]
                  ).map((r, i) => (
                    <div key={i} className="row between center" style={{ fontSize: 13, padding: "3px 0" }}>
                      <span className="muted">{r[0]}</span>
                      <span className="strong">{r[1]}</span>
                    </div>
                  ))}
                </div>
              </div>
              <div className="card">
                <div className="card-head">
                  <div className="card-title">Pré-visualização final</div>
                </div>
                <div className="card-pad">
                  <div className="wa-canvas">
                    {tpl && tpl.header && (
                      <div className="wa-row">
                        <div className="wa-bubble in" style={{ fontWeight: 700 }}>
                          {tpl.header}
                        </div>
                      </div>
                    )}
                    <WaBubble text={corpoPreenchido} dir="out" highlight status="entregue" time="09:14" buttons={tpl?.buttons?.length ? tpl.buttons : undefined} />
                  </div>
                </div>
              </div>
            </div>
          )}
        </div>
      </div>

      {/* Footer nav */}
      <div
        style={{
          display: "flex",
          justifyContent: "space-between",
          alignItems: "center",
          padding: "14px 26px",
          borderTop: "1px solid var(--border-subtle)",
          flex: "none",
          background: "var(--bg-base)",
        }}
      >
        <Button variant="ghost" onClick={() => (step > 1 ? setStep(step - 1) : router.push("/dashboard"))} icon="arrowLeft">
          {step > 1 ? "Voltar" : "Cancelar"}
        </Button>
        <div className="row center" style={{ gap: 12 }}>
          {step === 3 && (
            <span className="muted" style={{ fontSize: 13 }}>
              <span className="strong">{sel.size}</span> destinatários ·{" "}
              <span className="strong" style={{ color: "var(--wa)" }}>
                {brl(custoTotal)}
              </span>
            </span>
          )}
          {step < 3 ? (
            <Button variant="primary" iconRight="arrowRight" aria-disabled={!podeAvancar} onClick={() => setStep(step + 1)}>
              Continuar
            </Button>
          ) : (
            <Button variant="wa" icon={quando === "agora" ? "send" : "calendar"} onClick={() => setConfirm(true)}>
              {quando === "agora" ? "Disparar agora" : "Agendar disparo"}
            </Button>
          )}
        </div>
      </div>

      <ConfirmModal
        open={confirm}
        onClose={() => setConfirm(false)}
        onConfirm={confirmar}
        icon={quando === "agora" ? "send" : "calendar"}
        title={quando === "agora" ? "Confirmar disparo imediato?" : "Confirmar agendamento?"}
        message={
          "Você está prestes a enviar para " +
          sel.size +
          ' destinatários usando o template "' +
          (tpl ? tpl.name : "") +
          '". Custo estimado de ' +
          brl(custoTotal) +
          ". Esta ação não pode ser desfeita."
        }
        confirmLabel={quando === "agora" ? "Sim, disparar" : "Sim, agendar"}
      />
    </>
  );
}
