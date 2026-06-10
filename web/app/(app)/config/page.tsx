"use client";

import { useCallback, useEffect, useState } from "react";
import { Topbar } from "@/components/shell/Topbar";
import { Avatar, Button, Dropdown, Field, Icon, Input, Modal, Skel, useToast } from "@/components/ui";
import { IconName } from "@/lib/icons";
import {
  getOrg, saveOrg, getMeta, getBilling, getApiKey, regenApiKey,
  getEncerramento, saveEncerramento,
  getAusencia, saveAusencia,
  getAssinatura, saveAssinatura,
  getRoteamento, saveRoteamento,
  getChatbotAtivo, saveChatbotAtivo,
  getLembrete, saveLembrete, listReminders, cancelReminder, listTemplates,
  listWebhooks, createWebhook, deleteWebhook,
  listUsers, createUser, updateUser, resetUserPassword,
  listDepartments,
  listQuickReplies, createQuickReply, deleteQuickReply,
  type SettingsOrg, type SettingsMeta, type SettingsBilling, type SettingsWebhook,
  type SettingsEncerramento, type SettingsAusencia, type SettingsRoteamento, type SettingsLembrete,
  type ApiUser, type Papel, type ApiDepartment, type ApiQuickReply, type ApiReminder, type ApiTemplate,
} from "@/lib/api";

const SECS: { id: string; label: string; icon: IconName }[] = [
  { id: "org", label: "Organização", icon: "building" },
  { id: "whatsapp", label: "WhatsApp / Meta", icon: "phone" },
  { id: "atendimento", label: "Atendimento", icon: "message" },
  { id: "respostas", label: "Respostas rápidas", icon: "zap" },
  { id: "usuarios", label: "Usuários e papéis", icon: "users" },
  { id: "webhooks", label: "Webhooks e API", icon: "webhook" },
  { id: "faturamento", label: "Faturamento", icon: "dollar" },
];

const PAPEL_LABEL: Record<Papel, string> = { admin: "Administrador", supervisor: "Supervisor", atendente: "Atendente" };
const PAPEL_CLS: Record<Papel, string> = { admin: "badge-brand", supervisor: "badge-delivered", atendente: "badge-neutral" };

function Head({ title, sub }: { title: string; sub?: string }) {
  return (
    <div style={{ marginBottom: 20 }}>
      <h2 style={{ fontSize: 17, fontWeight: 700 }}>{title}</h2>
      {sub && <div className="muted" style={{ fontSize: 13, marginTop: 3 }}>{sub}</div>}
    </div>
  );
}

export default function ConfigPage() {
  const [sec, setSec] = useState("org");
  return (
    <>
      <Topbar title="Configurações" sub="Ajustes da organização, usuários e integrações" />
      <div className="content">
        <div className="content-pad">
          <div className="cfg-layout">
            <nav className="cfg-nav">
              {SECS.map((s) => (
                <div key={s.id} className={"cfg-nav-item" + (sec === s.id ? " active" : "")} onClick={() => setSec(s.id)}>
                  <div className="row center" style={{ gap: 9 }}>
                    <Icon name={s.icon} size={15} /> {s.label}
                  </div>
                </div>
              ))}
            </nav>
            <div style={{ maxWidth: 680 }}>
              {sec === "org" && <OrgSection />}
              {sec === "whatsapp" && <MetaSection />}
              {sec === "atendimento" && <AtendimentoSection />}
              {sec === "respostas" && <RespostasSection />}
              {sec === "usuarios" && <UsuariosSection />}
              {sec === "webhooks" && <WebhooksSection />}
              {sec === "faturamento" && <FaturamentoSection />}
            </div>
          </div>
        </div>
      </div>
    </>
  );
}

function OrgSection() {
  const toast = useToast();
  const [org, setOrg] = useState<SettingsOrg | null>(null);
  const [saving, setSaving] = useState(false);

  useEffect(() => { getOrg().then(setOrg).catch(() => {}); }, []);

  if (!org) return <Skel h={200} />;

  async function salvar() {
    setSaving(true);
    try {
      await saveOrg({ nome: org!.nome, uf: org!.uf, fuso: org!.fuso });
      toast({ type: "success", title: "Alterações salvas" });
    } catch (e) {
      toast({ type: "error", title: "Erro", message: (e as Error).message });
    } finally {
      setSaving(false);
    }
  }

  return (
    <div className="fade-in">
      <Head title="Organização" sub="Dados do órgão e identidade no ZapGov." />
      <div className="card card-pad col" style={{ gap: 16 }}>
        <div className="row" style={{ gap: 14 }}>
          <Field label="Nome da organização">
            <Input value={org.nome} onChange={(e) => setOrg({ ...org, nome: e.target.value })} />
          </Field>
          <Field label="UF">
            <select className="select" value={org.uf} onChange={(e) => setOrg({ ...org, uf: e.target.value })}>
              {["SP", "MG", "RJ", "RS", "PR", "SC", "BA", "PE", "CE", "GO", "MS"].map((u) => <option key={u}>{u}</option>)}
            </select>
          </Field>
        </div>
        <Field label="Subdomínio" hint="Definido na criação da conta.">
          <div className="subdomain-input">
            <span className="suffix" style={{ borderLeft: "none", borderRight: "1px solid var(--border)" }}>https://</span>
            <input value={org.subdominio || "—"} readOnly />
            <span className="suffix">.zapgov.com.br</span>
          </div>
        </Field>
        <Field label="Fuso horário">
          <select className="select" value={org.fuso} onChange={(e) => setOrg({ ...org, fuso: e.target.value })}>
            <option value="America/Sao_Paulo">(GMT-03:00) Brasília</option>
            <option value="America/Manaus">(GMT-04:00) Manaus</option>
          </select>
        </Field>
      </div>
      <div className="row" style={{ justifyContent: "flex-end", gap: 9, marginTop: 14 }}>
        <Button variant="primary" aria-disabled={saving} onClick={salvar}>{saving ? "Salvando…" : "Salvar alterações"}</Button>
      </div>
    </div>
  );
}

const MESES = ["Janeiro", "Fevereiro", "Março", "Abril", "Maio", "Junho", "Julho", "Agosto", "Setembro", "Outubro", "Novembro", "Dezembro"];
const WD = ["D", "S", "T", "Q", "Q", "S", "S"];
const pad2 = (n: number) => String(n).padStart(2, "0");

// rótulo amigável de uma entrada de feriado ('MM-DD' recorrente ou 'AAAA-MM-DD').
function feriadoLabel(f: string): { txt: string; recorrente: boolean } {
  const p = f.split("-");
  if (p.length === 2) {
    const [m, d] = p;
    return { txt: `${d}/${m}`, recorrente: true };
  }
  if (p.length === 3) {
    const [y, m, d] = p;
    return { txt: `${d}/${m}/${y}`, recorrente: false };
  }
  return { txt: f, recorrente: false };
}

function FeriadosCalendar({ value, onChange, disabled }: { value: string[]; onChange: (v: string[]) => void; disabled?: boolean }) {
  const today = new Date();
  const [ym, setYm] = useState({ y: today.getFullYear(), m: today.getMonth() });
  const [recorrente, setRecorrente] = useState(true);
  const set = new Set(value);

  const isHol = (d: number) => set.has(`${ym.y}-${pad2(ym.m + 1)}-${pad2(d)}`) || set.has(`${pad2(ym.m + 1)}-${pad2(d)}`);
  const isToday = (d: number) => ym.y === today.getFullYear() && ym.m === today.getMonth() && d === today.getDate();

  function toggle(d: number) {
    if (disabled) return;
    const ymd = `${ym.y}-${pad2(ym.m + 1)}-${pad2(d)}`;
    const mmdd = `${pad2(ym.m + 1)}-${pad2(d)}`;
    const next = value.filter((x) => x !== ymd && x !== mmdd);
    if (next.length === value.length) next.push(recorrente ? mmdd : ymd); // não estava → adiciona
    onChange(next);
  }

  const firstWd = new Date(ym.y, ym.m, 1).getDay();
  const nDays = new Date(ym.y, ym.m + 1, 0).getDate();
  const cells: (number | null)[] = [];
  for (let i = 0; i < firstWd; i++) cells.push(null);
  for (let d = 1; d <= nDays; d++) cells.push(d);

  const prev = () => setYm((s) => (s.m === 0 ? { y: s.y - 1, m: 11 } : { y: s.y, m: s.m - 1 }));
  const next = () => setYm((s) => (s.m === 11 ? { y: s.y + 1, m: 0 } : { y: s.y, m: s.m + 1 }));

  const chips = [...value].sort();

  return (
    <div className="col" style={{ gap: 12, opacity: disabled ? 0.55 : 1, pointerEvents: disabled ? "none" : "auto" }}>
      <div className="card" style={{ padding: 12, maxWidth: 300 }}>
        <div className="row between center" style={{ marginBottom: 8 }}>
          <button type="button" className="btn btn-ghost btn-icon btn-sm" onClick={prev}><Icon name="chevronLeft" size={15} /></button>
          <div style={{ fontWeight: 600, fontSize: 13 }}>{MESES[ym.m]} {ym.y}</div>
          <button type="button" className="btn btn-ghost btn-icon btn-sm" onClick={next}><Icon name="chevronRight" size={15} /></button>
        </div>
        <div style={{ display: "grid", gridTemplateColumns: "repeat(7, 1fr)", gap: 2 }}>
          {WD.map((w, i) => (
            <div key={i} style={{ textAlign: "center", fontSize: 10.5, color: "var(--text-faint)", fontWeight: 600, padding: "2px 0" }}>{w}</div>
          ))}
          {cells.map((d, i) => {
            if (d === null) return <div key={i} />;
            const hol = isHol(d);
            return (
              <button
                key={i}
                type="button"
                onClick={() => toggle(d)}
                style={{
                  height: 30, borderRadius: 7, fontSize: 12.5, cursor: "pointer",
                  border: isToday(d) ? "1px solid var(--brand)" : "1px solid transparent",
                  background: hol ? "var(--brand)" : "transparent",
                  color: hol ? "#fff" : "var(--text-primary)",
                  fontWeight: hol ? 700 : 400,
                }}
              >
                {d}
              </button>
            );
          })}
        </div>
      </div>

      <label className="row center" style={{ gap: 8, cursor: "pointer" }}>
        <input type="checkbox" checked={recorrente} onChange={(e) => setRecorrente(e.target.checked)} style={{ width: 15, height: 15, accentColor: "var(--brand)" }} />
        <span style={{ fontSize: 12.5 }}>Repetir todo ano ao marcar uma data</span>
      </label>

      {chips.length > 0 && (
        <div className="row" style={{ flexWrap: "wrap", gap: 6 }}>
          {chips.map((f) => {
            const { txt, recorrente: rec } = feriadoLabel(f);
            return (
              <span key={f} className="tag" style={{ gap: 6 }}>
                <Icon name="calendar" size={12} />
                {txt}{rec && <span className="faint" style={{ fontSize: 10 }}>todo ano</span>}
                <span style={{ cursor: "pointer", marginLeft: 2 }} onClick={() => onChange(value.filter((x) => x !== f))}>
                  <Icon name="x" size={12} />
                </span>
              </span>
            );
          })}
        </div>
      )}
    </div>
  );
}

function AtendimentoSection() {
  const toast = useToast();
  const [enc, setEnc] = useState<SettingsEncerramento | null>(null);
  const [aus, setAus] = useState<SettingsAusencia | null>(null);
  const [feriados, setFeriados] = useState<string[]>([]);
  const [assinatura, setAssinatura] = useState(false);
  const [rot, setRot] = useState<SettingsRoteamento>({ por_setor: false, por_etiqueta: false });
  const [chatbot, setChatbot] = useState(true);
  const [lem, setLem] = useState<SettingsLembrete>({ ativo: false, template: "", idioma: "pt_BR", hora: "09:00", fuso: "" });
  const [lemList, setLemList] = useState<ApiReminder[]>([]);
  const [tpls, setTpls] = useState<ApiTemplate[]>([]);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    getEncerramento().then(setEnc).catch(() => {});
    getAusencia().then((a) => { setAus(a); setFeriados(a.feriados || []); }).catch(() => {});
    getAssinatura().then((a) => setAssinatura(a.ativo)).catch(() => {});
    getRoteamento().then(setRot).catch(() => {});
    getChatbotAtivo().then((c) => setChatbot(c.ativo)).catch(() => {});
    getLembrete().then(setLem).catch(() => {});
    listReminders().then(setLemList).catch(() => {});
    listTemplates().then((ts) => setTpls(ts.filter((t) => t.status === "aprovado"))).catch(() => {});
  }, []);

  async function cancelarLembrete(id: number) {
    try {
      await cancelReminder(id);
      setLemList((ls) => ls.filter((x) => x.id !== id));
    } catch (e) { toast({ type: "error", title: "Erro", message: (e as Error).message }); }
  }

  if (!enc || !aus) return <Skel h={320} />;

  async function salvar() {
    setSaving(true);
    try {
      await Promise.all([
        saveEncerramento(enc!),
        saveAusencia({
          ativo: aus!.ativo, fim_semana: aus!.fim_semana, feriado: aus!.feriado, feriados,
          horario_ativo: aus!.horario_ativo, horario_inicio: aus!.horario_inicio, horario_fim: aus!.horario_fim,
          horario_dias: aus!.horario_dias, horario_msg: aus!.horario_msg,
        }),
        saveAssinatura(assinatura),
        saveRoteamento(rot),
        saveChatbotAtivo(chatbot),
        saveLembrete({ ativo: lem.ativo, template: lem.template, idioma: lem.idioma, hora: lem.hora }),
      ]);
      toast({ type: "success", title: "Alterações salvas" });
    } catch (e) {
      toast({ type: "error", title: "Erro", message: (e as Error).message });
    } finally {
      setSaving(false);
    }
  }

  const preview = enc.msg.replace(/\{\{nome\}\}/g, "Maria");

  return (
    <div className="fade-in">
      <Head title="Atendimento" sub="Mensagens automáticas: encerramento e ausência." />

      {/* Encerramento */}
      <div className="card card-pad col" style={{ gap: 16 }}>
        <label className="row center" style={{ gap: 10, cursor: "pointer" }}>
          <input
            type="checkbox"
            checked={enc.ativo}
            onChange={(e) => setEnc({ ...enc, ativo: e.target.checked })}
            style={{ width: 17, height: 17, accentColor: "var(--brand)" }}
          />
          <div className="col" style={{ gap: 1 }}>
            <div style={{ fontWeight: 600, fontSize: 13.5 }}>Enviar mensagem de encerramento</div>
            <div className="muted" style={{ fontSize: 12 }}>
              Ao finalizar um atendimento, manda um aviso ao cidadão (só se a janela de 24h ainda estiver aberta).
            </div>
          </div>
        </label>

        <Field label="Mensagem" hint="Use {{nome}} para inserir o primeiro nome do contato.">
          <textarea
            className="input"
            rows={4}
            value={enc.msg}
            disabled={!enc.ativo}
            onChange={(e) => setEnc({ ...enc, msg: e.target.value })}
            style={{ resize: "vertical", lineHeight: 1.5, opacity: enc.ativo ? 1 : 0.55 }}
          />
        </Field>

        {enc.ativo && enc.msg.trim() && (
          <Field label="Prévia">
            <div
              style={{
                background: "var(--bubble-out, #d9fdd3)", color: "#111",
                padding: "9px 12px", borderRadius: 10, fontSize: 13.5,
                lineHeight: 1.5, whiteSpace: "pre-wrap", maxWidth: 360,
                boxShadow: "0 1px 1px rgba(0,0,0,.08)",
              }}
            >
              {preview}
            </div>
          </Field>
        )}
      </div>

      {/* Roteamento por departamento */}
      <div className="card card-pad col" style={{ gap: 14, marginTop: 16 }}>
        <div style={{ fontWeight: 700, fontSize: 14 }}>Roteamento de atendimento</div>
        <label className="row center" style={{ gap: 10, cursor: "pointer" }}>
          <input
            type="checkbox"
            checked={chatbot}
            onChange={(e) => setChatbot(e.target.checked)}
            style={{ width: 17, height: 17, accentColor: "var(--brand)" }}
          />
          <div className="col" style={{ gap: 1 }}>
            <div style={{ fontWeight: 600, fontSize: 13.5 }}>Chatbot ativo</div>
            <div className="muted" style={{ fontSize: 12 }}>
              Desligado: a mensagem do cidadão vai <b>direto pra fila</b> (atendente assume), sem menu nem auto-resposta do bot.
            </div>
          </div>
        </label>
        <div style={{ height: 1, background: "var(--border-subtle)" }} />
        <label className="row center" style={{ gap: 10, cursor: "pointer" }}>
          <input
            type="checkbox"
            checked={rot.por_setor}
            onChange={(e) => setRot({ ...rot, por_setor: e.target.checked })}
            style={{ width: 17, height: 17, accentColor: "var(--brand)" }}
          />
          <div className="col" style={{ gap: 1 }}>
            <div style={{ fontWeight: 600, fontSize: 13.5 }}>Separar conversas por departamento</div>
            <div className="muted" style={{ fontSize: 12 }}>
              O atendente vê só as conversas do próprio setor. Admin e supervisor veem todas.
              (Atendente sem setor definido continua vendo tudo.)
            </div>
          </div>
        </label>
        <label className="row center" style={{ gap: 10, cursor: "pointer" }}>
          <input
            type="checkbox"
            checked={rot.por_etiqueta}
            onChange={(e) => setRot({ ...rot, por_etiqueta: e.target.checked })}
            style={{ width: 17, height: 17, accentColor: "var(--brand)" }}
          />
          <div className="col" style={{ gap: 1 }}>
            <div style={{ fontWeight: 600, fontSize: 13.5 }}>Etiquetar com o nome do atendente</div>
            <div className="muted" style={{ fontSize: 12 }}>
              Ao assumir uma conversa, aplica automaticamente uma etiqueta com o nome de quem atendeu.
            </div>
          </div>
        </label>
      </div>

      {/* Assinatura do atendente */}
      <div className="card card-pad col" style={{ gap: 12, marginTop: 16 }}>
        <label className="row center" style={{ gap: 10, cursor: "pointer" }}>
          <input
            type="checkbox"
            checked={assinatura}
            onChange={(e) => setAssinatura(e.target.checked)}
            style={{ width: 17, height: 17, accentColor: "var(--brand)" }}
          />
          <div className="col" style={{ gap: 1 }}>
            <div style={{ fontWeight: 600, fontSize: 13.5 }}>Assinatura do atendente</div>
            <div className="muted" style={{ fontSize: 12 }}>
              Anexa o nome de quem respondeu ao fim de cada mensagem enviada manualmente.
            </div>
          </div>
        </label>
        {assinatura && (
          <div
            style={{
              background: "var(--bubble-out, #d9fdd3)", color: "#111",
              padding: "9px 12px", borderRadius: 10, fontSize: 13.5,
              lineHeight: 1.5, whiteSpace: "pre-wrap", maxWidth: 360,
              boxShadow: "0 1px 1px rgba(0,0,0,.08)",
            }}
          >
            <b>Maria</b>{"\nSua 2ª via foi enviada por e-mail."}
          </div>
        )}
      </div>

      {/* Ausência (fim de semana / feriado) */}
      <div className="card card-pad col" style={{ gap: 16, marginTop: 16 }}>
        <label className="row center" style={{ gap: 10, cursor: "pointer" }}>
          <input
            type="checkbox"
            checked={aus.ativo}
            onChange={(e) => setAus({ ...aus, ativo: e.target.checked })}
            style={{ width: 17, height: 17, accentColor: "var(--brand)" }}
          />
          <div className="col" style={{ gap: 1 }}>
            <div style={{ fontWeight: 600, fontSize: 13.5 }}>Auto-resposta de ausência</div>
            <div className="muted" style={{ fontSize: 12 }}>
              Responde automaticamente quando o cidadão escreve em fim de semana ou feriado
              (no máximo uma vez a cada 8h). Fuso: {aus.fuso || "America/Sao_Paulo"}.
            </div>
          </div>
        </label>

        <Field label="Mensagem de fim de semana" hint="Enviada aos sábados e domingos. Em branco = não envia.">
          <textarea
            className="input"
            rows={3}
            value={aus.fim_semana}
            disabled={!aus.ativo}
            onChange={(e) => setAus({ ...aus, fim_semana: e.target.value })}
            style={{ resize: "vertical", lineHeight: 1.5, opacity: aus.ativo ? 1 : 0.55 }}
          />
        </Field>

        <Field label="Mensagem de feriado" hint="Enviada nas datas listadas abaixo. Em branco = não envia.">
          <textarea
            className="input"
            rows={3}
            value={aus.feriado}
            disabled={!aus.ativo}
            onChange={(e) => setAus({ ...aus, feriado: e.target.value })}
            style={{ resize: "vertical", lineHeight: 1.5, opacity: aus.ativo ? 1 : 0.55 }}
          />
        </Field>

        <Field label="Feriados" hint="Clique nos dias para marcar. Verde = feriado. Datas marcadas aparecem abaixo.">
          <FeriadosCalendar value={feriados} onChange={setFeriados} disabled={!aus.ativo} />
        </Field>
      </div>

      {/* Horário comercial */}
      <div className="card card-pad col" style={{ gap: 16, marginTop: 16 }}>
        <label className="row center" style={{ gap: 10, cursor: "pointer" }}>
          <input
            type="checkbox"
            checked={aus.horario_ativo}
            onChange={(e) => setAus({ ...aus, horario_ativo: e.target.checked })}
            style={{ width: 17, height: 17, accentColor: "var(--brand)" }}
          />
          <div className="col" style={{ gap: 1 }}>
            <div style={{ fontWeight: 600, fontSize: 13.5 }}>Horário comercial</div>
            <div className="muted" style={{ fontSize: 12 }}>
              Fora do expediente (em dia útil), responde automaticamente. Fuso: {aus.fuso || "America/Sao_Paulo"}.
            </div>
          </div>
        </label>

        <div className="row" style={{ gap: 14 }}>
          <Field label="Abre às">
            <Input type="time" value={aus.horario_inicio} disabled={!aus.horario_ativo} onChange={(e) => setAus({ ...aus, horario_inicio: e.target.value })} />
          </Field>
          <Field label="Fecha às">
            <Input type="time" value={aus.horario_fim} disabled={!aus.horario_ativo} onChange={(e) => setAus({ ...aus, horario_fim: e.target.value })} />
          </Field>
        </div>

        <Field label="Dias de atendimento">
          <div className="row" style={{ gap: 6, flexWrap: "wrap" }}>
            {[["1", "Seg"], ["2", "Ter"], ["3", "Qua"], ["4", "Qui"], ["5", "Sex"], ["6", "Sáb"], ["0", "Dom"]].map(([num, lbl]) => {
              const dias = aus.horario_dias ? aus.horario_dias.split(",").filter(Boolean) : [];
              const on = dias.includes(num);
              return (
                <button
                  key={num}
                  type="button"
                  disabled={!aus.horario_ativo}
                  onClick={() => {
                    const next = on ? dias.filter((d) => d !== num) : [...dias, num];
                    setAus({ ...aus, horario_dias: next.join(",") });
                  }}
                  className="btn btn-sm"
                  style={{ background: on ? "var(--brand)" : "var(--bg-sunken)", color: on ? "#fff" : "var(--text-secondary)", opacity: aus.horario_ativo ? 1 : 0.55 }}
                >
                  {lbl}
                </button>
              );
            })}
          </div>
        </Field>

        <Field label="Mensagem fora do horário">
          <textarea
            className="input"
            rows={3}
            value={aus.horario_msg}
            disabled={!aus.horario_ativo}
            onChange={(e) => setAus({ ...aus, horario_msg: e.target.value })}
            style={{ resize: "vertical", lineHeight: 1.5, opacity: aus.horario_ativo ? 1 : 0.55 }}
          />
        </Field>
      </div>

      {/* Lembrete automático de consulta */}
      <div className="card card-pad col" style={{ gap: 16, marginTop: 16 }}>
        <label className="row center" style={{ gap: 10, cursor: "pointer" }}>
          <input
            type="checkbox"
            checked={lem.ativo}
            onChange={(e) => setLem({ ...lem, ativo: e.target.checked })}
            style={{ width: 17, height: 17, accentColor: "var(--brand)" }}
          />
          <div className="col" style={{ gap: 1 }}>
            <div style={{ fontWeight: 600, fontSize: 13.5 }}>Lembrete automático de consulta</div>
            <div className="muted" style={{ fontSize: 12 }}>
              Quando um template enviado tiver uma <b>data futura</b> (DD/MM/AAAA) numa variável,
              agenda um lembrete pro <b>dia anterior</b>, no horário abaixo. Fuso: {lem.fuso || "America/Sao_Paulo"}.
            </div>
          </div>
        </label>

        <div className="row" style={{ gap: 14 }}>
          <Field label="Template do lembrete" hint="Aprovado. Variáveis: {{1}} = nome, {{2}} = data.">
            <select
              className="select"
              value={lem.template}
              disabled={!lem.ativo}
              onChange={(e) => {
                const t = tpls.find((x) => x.name === e.target.value);
                setLem({ ...lem, template: e.target.value, idioma: t?.language || "pt_BR" });
              }}
            >
              <option value="">— escolher —</option>
              {tpls.map((t) => <option key={t.name + t.language} value={t.name}>{t.name}</option>)}
            </select>
          </Field>
          <Field label="Hora do envio">
            <Input type="time" value={lem.hora} disabled={!lem.ativo} onChange={(e) => setLem({ ...lem, hora: e.target.value })} />
          </Field>
        </div>

        {lemList.length > 0 && (
          <div className="col" style={{ gap: 6 }}>
            <span className="field-label">Lembretes agendados ({lemList.length})</span>
            <div style={{ maxHeight: 180, overflowY: "auto", border: "1px solid var(--border-subtle)", borderRadius: 9 }}>
              {lemList.map((r) => (
                <div key={r.id} className="row center between" style={{ padding: "8px 12px", borderBottom: "1px solid var(--border-subtle)", fontSize: 12.5 }}>
                  <div className="col" style={{ gap: 0 }}>
                    <span className="strong">{r.nome || r.phone}</span>
                    <span className="faint" style={{ fontSize: 11 }}>
                      consulta {new Date(r.data_evento).toLocaleDateString("pt-BR")} · lembrete {new Date(r.enviar_em).toLocaleString("pt-BR", { day: "2-digit", month: "2-digit", hour: "2-digit", minute: "2-digit" })}
                    </span>
                  </div>
                  <button className="btn btn-ghost btn-icon btn-sm" title="Cancelar lembrete" onClick={() => cancelarLembrete(r.id)}>
                    <Icon name="x" size={14} />
                  </button>
                </div>
              ))}
            </div>
          </div>
        )}
      </div>

      <div className="row" style={{ justifyContent: "flex-end", gap: 9, marginTop: 14 }}>
        <Button variant="primary" aria-disabled={saving} onClick={salvar}>{saving ? "Salvando…" : "Salvar alterações"}</Button>
      </div>
    </div>
  );
}

function MetaSection() {
  const [meta, setMeta] = useState<SettingsMeta | null>(null);
  useEffect(() => { getMeta().then(setMeta).catch(() => {}); }, []);
  if (!meta) return <Skel h={160} />;
  return (
    <div className="fade-in">
      <Head title="WhatsApp / Meta" sub="Status da conexão com a Meta Cloud API." />
      <div className="card card-pad" style={{ marginBottom: 14 }}>
        <div className="row between center">
          <div className="row center" style={{ gap: 13 }}>
            <span className="kpi-ico" style={{ background: "var(--wa-soft)", color: "var(--wa)", width: 44, height: 44 }}>
              <Icon name="phone" size={20} />
            </span>
            <div className="col" style={{ gap: 2 }}>
              <div className="strong" style={{ fontSize: 14.5 }}>Phone ID: <span className="mono">{meta.phone_number_id || "—"}</span></div>
              <div className="mono" style={{ fontSize: 11.5, color: "var(--text-tertiary)" }}>WABA: {meta.waba_id || "—"} · {meta.api_version}</div>
            </div>
          </div>
          <span className={"badge " + (meta.connected ? "badge-read" : "badge-failed")}>
            <span className="dot" /> {meta.connected ? "Conectado" : "Sem token"}
          </span>
        </div>
      </div>
      <div className="card card-pad" style={{ fontSize: 12.5, color: "var(--text-tertiary)", display: "flex", gap: 8 }}>
        <Icon name="info" size={15} style={{ flex: "none", marginTop: 1 }} />
        Credenciais (token, App Secret) vivem no ambiente do servidor (`.env`). Edição por painel entra com o multi-tenant.
      </div>
    </div>
  );
}

function RespostasSection() {
  const toast = useToast();
  const [list, setList] = useState<ApiQuickReply[]>([]);
  const [loading, setLoading] = useState(true);
  const [atalho, setAtalho] = useState("");
  const [texto, setTexto] = useState("");
  const [saving, setSaving] = useState(false);

  const load = useCallback(async () => {
    setLoading(true);
    try { setList(await listQuickReplies()); } catch (e) { toast({ type: "error", title: "Erro", message: (e as Error).message }); }
    finally { setLoading(false); }
  }, [toast]);
  useEffect(() => { load(); }, [load]);

  async function adicionar() {
    if (!texto.trim()) return;
    setSaving(true);
    try {
      await createQuickReply(atalho.trim(), texto.trim());
      setAtalho(""); setTexto("");
      load();
    } catch (e) { toast({ type: "error", title: "Erro", message: (e as Error).message }); }
    finally { setSaving(false); }
  }
  async function remover(id: number) {
    try { await deleteQuickReply(id); load(); } catch (e) { toast({ type: "error", title: "Erro", message: (e as Error).message }); }
  }

  return (
    <div className="fade-in">
      <Head title="Respostas rápidas" sub="Textos prontos que o atendente insere com um clique no chat." />
      <div className="card card-pad col" style={{ gap: 12, marginBottom: 16 }}>
        <div className="row" style={{ gap: 12 }}>
          <Field label="Atalho (opcional)" hint="Ex.: /2via">
            <Input value={atalho} onChange={(e) => setAtalho(e.target.value)} placeholder="/saudacao" />
          </Field>
        </div>
        <Field label="Texto">
          <textarea className="input" rows={3} value={texto} onChange={(e) => setTexto(e.target.value)} placeholder="Olá! Como posso ajudar?" style={{ resize: "vertical", lineHeight: 1.5 }} />
        </Field>
        <div className="row" style={{ justifyContent: "flex-end" }}>
          <Button variant="primary" icon="plus" aria-disabled={saving || !texto.trim()} onClick={adicionar}>{saving ? "Salvando…" : "Adicionar"}</Button>
        </div>
      </div>

      <div className="card" style={{ overflow: "hidden" }}>
        {loading ? (
          <div className="card-pad col" style={{ gap: 10 }}>{[0, 1].map((i) => <Skel key={i} h={24} />)}</div>
        ) : list.length === 0 ? (
          <div className="card-pad muted" style={{ fontSize: 13, textAlign: "center" }}>Nenhuma resposta rápida ainda.</div>
        ) : (
          <table className="ds-table">
            <thead><tr><th style={{ width: 130 }}>Atalho</th><th>Texto</th><th style={{ width: 40 }}></th></tr></thead>
            <tbody>
              {list.map((q) => (
                <tr key={q.id}>
                  <td className="mono" style={{ fontSize: 12.5 }}>{q.atalho || "—"}</td>
                  <td style={{ fontSize: 12.5, color: "var(--text-secondary)" }}>{q.texto}</td>
                  <td>
                    <button className="btn btn-ghost btn-icon btn-sm" onClick={() => remover(q.id)} title="Remover">
                      <Icon name="trash" size={14} />
                    </button>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </div>
  );
}

function UsuariosSection() {
  const toast = useToast();
  const [list, setList] = useState<ApiUser[]>([]);
  const [deps, setDeps] = useState<ApiDepartment[]>([]);
  const [loading, setLoading] = useState(true);
  const [novo, setNovo] = useState(false);
  const [editar, setEditar] = useState<ApiUser | null>(null);
  const [resetada, setResetada] = useState<{ nome: string; senha: string } | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      const [us, ds] = await Promise.all([listUsers(), listDepartments().catch(() => [])]);
      setList(us);
      setDeps(ds);
    } catch (e) { toast({ type: "error", title: "Erro", message: (e as Error).message }); }
    finally { setLoading(false); }
  }, [toast]);
  useEffect(() => { load(); }, [load]);

  async function mudarPapel(u: ApiUser, papel: Papel) {
    try { await updateUser(u.id, { nome: u.nome, email: u.email, papel, setor: u.setor, ativo: u.ativo }); load(); } catch (e) { toast({ type: "error", title: "Erro", message: (e as Error).message }); }
  }
  async function toggleAtivo(u: ApiUser) {
    try { await updateUser(u.id, { nome: u.nome, email: u.email, papel: u.papel, setor: u.setor, ativo: !u.ativo }); load(); } catch (e) { toast({ type: "error", title: "Erro", message: (e as Error).message }); }
  }
  async function resetarSenha(u: ApiUser) {
    // Gera uma senha provisória legível (8 caracteres).
    const chars = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789abcdefghijkmnpqrstuvwxyz";
    const senha = Array.from({ length: 8 }, () => chars[Math.floor(Math.random() * chars.length)]).join("");
    try {
      await resetUserPassword(u.id, senha);
      setResetada({ nome: u.nome, senha });
    } catch (e) {
      toast({ type: "error", title: "Erro", message: (e as Error).message });
    }
  }

  return (
    <div className="fade-in">
      <div className="row between center" style={{ marginBottom: 20 }}>
        <Head title="Usuários e papéis" sub="Quem acessa o painel e com qual permissão." />
        <Button variant="primary" size="sm" icon="plus" onClick={() => setNovo(true)}>Novo usuário</Button>
      </div>
      <div className="card" style={{ overflow: "visible" }}>
        {loading ? (
          <div className="card-pad col" style={{ gap: 10 }}>{[0, 1, 2].map((i) => <Skel key={i} h={24} />)}</div>
        ) : (
          <table className="ds-table">
            <thead>
              <tr><th>Usuário</th><th>Papel</th><th>Setor</th><th>Status</th><th style={{ width: 40 }}></th></tr>
            </thead>
            <tbody>
              {list.map((u) => (
                <tr key={u.id}>
                  <td className="strong">
                    <div className="row center" style={{ gap: 10 }}>
                      <Avatar name={u.nome} size={30} />
                      <div className="col" style={{ gap: 0 }}>
                        <span>{u.nome}</span>
                        <span className="faint" style={{ fontSize: 11 }}>{u.email}</span>
                      </div>
                    </div>
                  </td>
                  <td><span className={"badge " + PAPEL_CLS[u.papel]}>{PAPEL_LABEL[u.papel]}</span></td>
                  <td><span className="faint" style={{ fontSize: 12.5 }}>{u.setor || "Todos"}</span></td>
                  <td><span className={"badge " + (u.ativo ? "badge-read" : "badge-neutral")}><span className="dot" /> {u.ativo ? "Ativo" : "Inativo"}</span></td>
                  <td>
                    <Dropdown trigger={<button className="btn btn-ghost btn-icon btn-sm"><Icon name="more" size={15} /></button>}>
                      <div className="dropdown-item" onClick={() => setEditar(u)}>
                        <Icon name="edit" size={15} /> Editar dados
                      </div>
                      <div className="dropdown-item" onClick={() => resetarSenha(u)}>
                        <Icon name="key" size={15} /> Resetar senha
                      </div>
                      <div className="dropdown-sep" />
                      <div className="dropdown-label">Alterar papel</div>
                      {(["admin", "supervisor", "atendente"] as Papel[]).map((p) => (
                        <div key={p} className="dropdown-item" onClick={() => mudarPapel(u, p)}>
                          <Icon name={u.papel === p ? "check" : "user"} size={15} /> {PAPEL_LABEL[p]}
                        </div>
                      ))}
                      <div className="dropdown-sep" />
                      <div className="dropdown-item danger" onClick={() => toggleAtivo(u)}>
                        <Icon name="ban" size={15} /> {u.ativo ? "Desativar" : "Reativar"}
                      </div>
                    </Dropdown>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
      <NovoUsuarioModal open={novo} deps={deps} onClose={() => setNovo(false)} onCreated={load} />
      <EditarUsuarioModal user={editar} deps={deps} onClose={() => setEditar(null)} onSaved={load} />

      <Modal open={!!resetada} onClose={() => setResetada(null)} width={420}>
        <div className="modal-head">
          <div className="modal-title">Senha redefinida</div>
          <div className="modal-sub">{resetada?.nome} — troca obrigatória no próximo login.</div>
        </div>
        <div className="modal-body col" style={{ gap: 12 }}>
          <div className="muted" style={{ fontSize: 12.5 }}>Repasse esta senha provisória ao usuário:</div>
          <div className="row center between" style={{ padding: "12px 14px", borderRadius: 9, background: "var(--bg-sunken)", border: "1px solid var(--border)" }}>
            <span className="mono" style={{ fontSize: 18, fontWeight: 700, letterSpacing: 1 }}>{resetada?.senha}</span>
            <Button
              variant="secondary"
              size="sm"
              icon="copy"
              onClick={() => { if (resetada) { navigator.clipboard?.writeText(resetada.senha); toast({ type: "success", title: "Copiado" }); } }}
            >
              Copiar
            </Button>
          </div>
        </div>
        <div className="modal-foot">
          <Button variant="primary" className="btn-block" onClick={() => setResetada(null)}>Entendi</Button>
        </div>
      </Modal>
    </div>
  );
}

function EditarUsuarioModal({ user, deps, onClose, onSaved }: { user: ApiUser | null; deps: ApiDepartment[]; onClose: () => void; onSaved: () => void }) {
  const toast = useToast();
  const [f, setF] = useState<{ nome: string; email: string; papel: Papel; setor: string; ativo: boolean }>({ nome: "", email: "", papel: "atendente", setor: "", ativo: true });
  const [novaSenha, setNovaSenha] = useState("");
  const [saving, setSaving] = useState(false);
  useEffect(() => { if (user) { setF({ nome: user.nome, email: user.email, papel: user.papel, setor: user.setor, ativo: user.ativo }); setNovaSenha(""); } }, [user]);

  async function salvar() {
    if (!user || !f.nome || !f.email) return;
    if (novaSenha && novaSenha.length < 6) {
      toast({ type: "error", title: "Senha curta", message: "A senha provisória precisa ter ao menos 6 caracteres" });
      return;
    }
    setSaving(true);
    try {
      await updateUser(user.id, f);
      if (novaSenha) await resetUserPassword(user.id, novaSenha);
      toast({ type: "success", title: "Usuário atualizado", message: novaSenha ? "Senha redefinida — troca obrigatória no próximo login" : f.email });
      onSaved();
      onClose();
    } catch (e) {
      toast({ type: "error", title: "Erro", message: (e as Error).message });
    } finally { setSaving(false); }
  }

  return (
    <Modal open={!!user} onClose={onClose} width={460}>
      <div className="modal-head"><div className="modal-title">Editar usuário</div><div className="modal-sub">Atualize nome, e-mail e permissão.</div></div>
      <div className="modal-body col" style={{ gap: 14 }}>
        <Field label="Nome"><Input value={f.nome} onChange={(e) => setF({ ...f, nome: e.target.value })} /></Field>
        <Field label="E-mail"><Input type="email" value={f.email} onChange={(e) => setF({ ...f, email: e.target.value })} /></Field>
        <div className="row" style={{ gap: 12 }}>
          <Field label="Papel">
            <select className="select" value={f.papel} onChange={(e) => setF({ ...f, papel: e.target.value as Papel })}>
              <option value="atendente">Atendente</option>
              <option value="supervisor">Supervisor</option>
              <option value="admin">Administrador</option>
            </select>
          </Field>
          <Field label="Status">
            <select className="select" value={f.ativo ? "1" : "0"} onChange={(e) => setF({ ...f, ativo: e.target.value === "1" })}>
              <option value="1">Ativo</option>
              <option value="0">Inativo</option>
            </select>
          </Field>
        </div>
        <Field label="Setor" hint="Departamento do atendente. Em branco = todos os setores.">
          <select className="select" value={f.setor} onChange={(e) => setF({ ...f, setor: e.target.value })}>
            <option value="">Todos os setores</option>
            {deps.map((d) => <option key={d.id} value={d.nome}>{d.nome}</option>)}
          </select>
        </Field>
        <Field label="Redefinir senha" hint="Deixe em branco para manter. Ao definir, o usuário troca no próximo login.">
          <Input type="password" placeholder="Nova senha provisória" value={novaSenha} onChange={(e) => setNovaSenha(e.target.value)} />
        </Field>
      </div>
      <div className="modal-foot">
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="primary" icon="check" aria-disabled={saving || !f.nome || !f.email} onClick={salvar}>{saving ? "Salvando…" : "Salvar"}</Button>
      </div>
    </Modal>
  );
}

function NovoUsuarioModal({ open, deps, onClose, onCreated }: { open: boolean; deps: ApiDepartment[]; onClose: () => void; onCreated: () => void }) {
  const toast = useToast();
  const [f, setF] = useState<{ nome: string; email: string; senha: string; papel: Papel; setor: string }>({ nome: "", email: "", senha: "", papel: "atendente", setor: "" });
  const [saving, setSaving] = useState(false);
  useEffect(() => { if (open) setF({ nome: "", email: "", senha: "", papel: "atendente", setor: "" }); }, [open]);

  async function salvar() {
    if (!f.nome || !f.email || !f.senha) return;
    setSaving(true);
    try {
      await createUser(f);
      toast({ type: "success", title: "Usuário criado", message: f.email });
      onCreated();
      onClose();
    } catch (e) {
      toast({ type: "error", title: "Erro", message: (e as Error).message });
    } finally { setSaving(false); }
  }

  return (
    <Modal open={open} onClose={onClose} width={460}>
      <div className="modal-head"><div className="modal-title">Novo usuário</div><div className="modal-sub">Acesso ao painel com o papel escolhido.</div></div>
      <div className="modal-body col" style={{ gap: 14 }}>
        <Field label="Nome"><Input value={f.nome} onChange={(e) => setF({ ...f, nome: e.target.value })} /></Field>
        <Field label="E-mail"><Input type="email" value={f.email} onChange={(e) => setF({ ...f, email: e.target.value })} /></Field>
        <div className="row" style={{ gap: 12 }}>
          <Field label="Senha"><Input type="password" value={f.senha} onChange={(e) => setF({ ...f, senha: e.target.value })} /></Field>
          <Field label="Papel">
            <select className="select" value={f.papel} onChange={(e) => setF({ ...f, papel: e.target.value as Papel })}>
              <option value="atendente">Atendente</option>
              <option value="supervisor">Supervisor</option>
              <option value="admin">Administrador</option>
            </select>
          </Field>
        </div>
        <Field label="Setor" hint="Departamento do atendente. Em branco = todos os setores.">
          <select className="select" value={f.setor} onChange={(e) => setF({ ...f, setor: e.target.value })}>
            <option value="">Todos os setores</option>
            {deps.map((d) => <option key={d.id} value={d.nome}>{d.nome}</option>)}
          </select>
        </Field>
      </div>
      <div className="modal-foot">
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="primary" icon="check" aria-disabled={saving || !f.nome || !f.email || !f.senha} onClick={salvar}>{saving ? "Criando…" : "Criar usuário"}</Button>
      </div>
    </Modal>
  );
}

function WebhooksSection() {
  const toast = useToast();
  const [apiKey, setApiKey] = useState("");
  const [hooks, setHooks] = useState<SettingsWebhook[]>([]);
  const [novaUrl, setNovaUrl] = useState("");

  const load = useCallback(async () => {
    try {
      const [k, w] = await Promise.all([getApiKey(), listWebhooks()]);
      setApiKey(k.api_key);
      setHooks(w);
    } catch (e) { toast({ type: "error", title: "Erro", message: (e as Error).message }); }
  }, [toast]);
  useEffect(() => { load(); }, [load]);

  async function regen() {
    try { const r = await regenApiKey(); setApiKey(r.api_key); toast({ type: "success", title: "Nova chave gerada" }); }
    catch (e) { toast({ type: "error", title: "Erro", message: (e as Error).message }); }
  }
  async function add() {
    if (!novaUrl.trim()) return;
    try { await createWebhook(novaUrl, ["messages", "message_status"]); setNovaUrl(""); load(); }
    catch (e) { toast({ type: "error", title: "Erro", message: (e as Error).message }); }
  }
  async function remove(id: number) {
    try { await deleteWebhook(id); load(); } catch (e) { toast({ type: "error", title: "Erro", message: (e as Error).message }); }
  }

  return (
    <div className="fade-in">
      <Head title="Webhooks e API" sub="Integre o ZapGov aos sistemas do seu órgão." />
      <div className="card card-pad col" style={{ gap: 12, marginBottom: 14 }}>
        <Field label="Chave de API">
          <div className="row" style={{ gap: 8 }}>
            <Input readOnly value={apiKey || "—"} icon="key" />
            <Button variant="secondary" icon="copy" onClick={() => { navigator.clipboard.writeText(apiKey); toast({ type: "info", title: "Chave copiada" }); }}>Copiar</Button>
            <Button variant="secondary" icon="refresh" onClick={regen}>Rotacionar</Button>
          </div>
        </Field>
      </div>
      <div className="card">
        <div className="card-head">
          <div className="card-title">Endpoints</div>
        </div>
        <div className="card-pad row" style={{ gap: 8 }}>
          <Input placeholder="https://seu-sistema.gov.br/webhook" value={novaUrl} onChange={(e) => setNovaUrl(e.target.value)} />
          <Button variant="secondary" icon="plus" onClick={add}>Adicionar</Button>
        </div>
        {hooks.length === 0 ? (
          <div className="muted" style={{ padding: "0 18px 16px", fontSize: 12.5 }}>Nenhum endpoint cadastrado.</div>
        ) : hooks.map((w, i) => (
          <div key={w.id} className="row between center" style={{ padding: "13px 18px", borderTop: "1px solid var(--border-subtle)" }}>
            <div className="col" style={{ gap: 4, minWidth: 0 }}>
              <div className="mono" style={{ fontSize: 12.5, whiteSpace: "nowrap", overflow: "hidden", textOverflow: "ellipsis" }}>{w.url}</div>
              <div className="row" style={{ gap: 5 }}>{w.eventos.map((e) => <span key={e} className="badge badge-neutral mono" style={{ fontSize: 10.5 }}>{e}</span>)}</div>
            </div>
            <button className="btn btn-ghost btn-icon btn-sm" onClick={() => remove(w.id)}><Icon name="trash" size={15} /></button>
          </div>
        ))}
      </div>
    </div>
  );
}

function FaturamentoSection() {
  const [b, setB] = useState<SettingsBilling | null>(null);
  useEffect(() => { getBilling().then(setB).catch(() => {}); }, []);
  if (!b) return <Skel h={160} />;
  const pct = b.limite_mensal ? Math.min(100, Math.round((b.enviadas / b.limite_mensal) * 100)) : 0;
  return (
    <div className="fade-in">
      <Head title="Faturamento" sub="Plano e consumo do mês." />
      <div className="card card-pad row between center" style={{ marginBottom: 14 }}>
        <div className="col" style={{ gap: 3 }}>
          <div className="row center" style={{ gap: 8 }}>
            <span className="strong" style={{ fontSize: 16 }}>Plano {b.plano}</span>
            <span className="badge badge-brand">Ativo</span>
          </div>
          <div className="muted" style={{ fontSize: 12.5 }}>até {b.limite_mensal.toLocaleString("pt-BR")} mensagens/mês</div>
        </div>
      </div>
      <div className="card card-pad col" style={{ gap: 12 }}>
        <div className="row between center">
          <span className="strong">Consumo do mês</span>
          <span className="mono muted" style={{ fontSize: 12.5 }}>{b.enviadas.toLocaleString("pt-BR")} / {b.limite_mensal.toLocaleString("pt-BR")}</span>
        </div>
        <div className="progress" style={{ height: 8 }}>
          <div className="progress-bar" style={{ width: pct + "%", background: pct > 85 ? "var(--st-failed)" : "var(--brand)" }} />
        </div>
        <div className="row" style={{ gap: 24, marginTop: 6 }}>
          {([["Enviadas", b.enviadas, "var(--brand-text)"], ["Entregues", b.entregues, "var(--st-delivered)"], ["Lidas", b.lidas, "var(--st-read)"]] as [string, number, string][]).map((s, i) => (
            <div key={i} className="stat-inline">
              <span className="v" style={{ color: s[2] }}>{s[1].toLocaleString("pt-BR")}</span>
              <span className="l">{s[0]}</span>
            </div>
          ))}
        </div>
      </div>
    </div>
  );
}
