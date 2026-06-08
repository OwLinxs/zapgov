"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Logo } from "@/components/shell/Logo";
import { Button, Field, Input, Icon, Modal, useToast } from "@/components/ui";
import { useTheme } from "@/lib/theme";
import { login, changePassword } from "@/lib/api";

function MetaStatusPillStandalone() {
  return (
    <div
      className="row center"
      style={{ gap: 9, padding: "8px 11px", borderRadius: 8, background: "var(--st-read-soft)", border: "1px solid var(--border-subtle)" }}
    >
      <span style={{ width: 7, height: 7, borderRadius: 99, background: "var(--st-read)", boxShadow: "0 0 0 3px var(--st-read-soft)", flex: "none" }} />
      <div className="col" style={{ gap: 1, lineHeight: 1.25 }}>
        <div style={{ fontSize: 11.5, fontWeight: 700, color: "var(--st-read)" }}>API conectada</div>
        <div style={{ fontSize: 10, color: "var(--text-faint)" }}>Meta Cloud API</div>
      </div>
    </div>
  );
}

const STATS: [string, string][] = [
  ["98%", "taxa de entrega média"],
  ["12M+", "mensagens/mês"],
  ["LGPD", "em conformidade"],
];

export default function LoginPage() {
  const router = useRouter();
  const { theme, toggleTheme } = useTheme();
  const [step, setStep] = useState<"login" | "onboard">("login");
  const [sub, setSub] = useState("ribeiraopreto");
  const [email, setEmail] = useState("admin@zapgov.local");
  const [senha, setSenha] = useState("");
  const [err, setErr] = useState("");
  const [loading, setLoading] = useState(false);
  const [needChange, setNeedChange] = useState(false);

  const entrar = async () => {
    setErr("");
    if (!email.includes("@")) return setErr("E-mail inválido.");
    if (!senha) return setErr("Informe a senha.");
    setLoading(true);
    try {
      const { token, user } = await login(email, senha);
      localStorage.setItem("zg-token", token);
      localStorage.setItem("zg-user", JSON.stringify(user));
      if (user.must_change_password) {
        setNeedChange(true); // bloqueia até trocar a senha
        setLoading(false);
        return;
      }
      router.push("/dashboard");
    } catch (e) {
      setErr((e as Error).message);
      setLoading(false);
    }
  };

  return (
    <div className="login-wrap fade-in">
      {/* Lateral institucional */}
      <div className="login-aside">
        <div style={{ position: "absolute", inset: 0, opacity: 0.5, background: "radial-gradient(900px 500px at 80% -10%, var(--brand-soft), transparent 70%)" }} />
        <div style={{ position: "relative" }}>
          <Logo size={34} />
        </div>
        <div style={{ position: "relative", maxWidth: 420 }}>
          <div style={{ fontSize: 30, fontWeight: 800, letterSpacing: "-0.04em", lineHeight: 1.12 }}>
            Comunicação oficial via WhatsApp, com a seriedade que o setor público exige.
          </div>
          <p style={{ marginTop: 16, fontSize: 14.5, color: "var(--text-tertiary)", lineHeight: 1.6 }}>
            Disparos em massa, atendimento ativo e relatórios auditáveis sobre a API oficial da Meta.
            Multi-tenant, seguro e rastreável.
          </p>
          <div className="row" style={{ gap: 22, marginTop: 30 }}>
            {STATS.map((s, i) => (
              <div key={i} className="col" style={{ gap: 2 }}>
                <div className="mono" style={{ fontSize: 22, fontWeight: 700, color: "var(--brand-text)" }}>
                  {s[0]}
                </div>
                <div style={{ fontSize: 11.5, color: "var(--text-faint)" }}>{s[1]}</div>
              </div>
            ))}
          </div>
        </div>
        <div style={{ position: "relative" }}>
          <MetaStatusPillStandalone />
        </div>
      </div>

      {/* Formulário */}
      <div className="login-form-side">
        <div className="login-card">
          <button
            className="theme-toggle"
            style={{ width: "auto", position: "absolute", top: 24, right: 24 }}
            onClick={toggleTheme}
          >
            <Icon name={theme === "dark" ? "sun" : "moon"} size={15} />
          </button>

          {step === "login" ? (
            <>
              <h1 style={{ fontSize: 23, fontWeight: 700 }}>Acessar painel</h1>
              <p style={{ fontSize: 13.5, color: "var(--text-tertiary)", marginTop: 5, marginBottom: 26 }}>
                Entre com as credenciais do seu órgão.
              </p>

              <div className="col" style={{ gap: 16 }}>
                <Field label="Subdomínio do órgão" hint="Identificador único da sua organização no ZapGov.">
                  <div className="subdomain-input">
                    <span className="suffix" style={{ borderLeft: "none", borderRight: "1px solid var(--border)" }}>
                      https://
                    </span>
                    <input
                      value={sub}
                      onChange={(e) => setSub(e.target.value.replace(/[^a-z0-9-]/g, ""))}
                      placeholder="seu-orgao"
                    />
                    <span className="suffix">.zapgov.com.br</span>
                  </div>
                </Field>
                <Field label="E-mail institucional">
                  <Input icon="user" type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="nome@orgao.gov.br" />
                </Field>
                <Field label="Senha" error={err || undefined}>
                  <Input icon="key" type="password" value={senha} onChange={(e) => setSenha(e.target.value)} placeholder="••••••••" />
                </Field>
                <div className="row between center">
                  <label className="row center" style={{ gap: 7, fontSize: 12.5, color: "var(--text-secondary)", cursor: "pointer" }}>
                    <input type="checkbox" className="checkbox" defaultChecked /> Manter conectado
                  </label>
                  <a style={{ fontSize: 12.5, color: "var(--brand-text)", fontWeight: 600, cursor: "pointer" }}>
                    Esqueci a senha
                  </a>
                </div>
                <Button variant="primary" size="lg" className="btn-block" onClick={entrar} aria-disabled={loading}>
                  {loading ? "Entrando…" : "Entrar no painel"}
                  {!loading && <Icon name="arrowRight" size={16} />}
                </Button>
                <div className="divider" style={{ margin: "6px 0" }} />
                <div className="row center" style={{ gap: 8, justifyContent: "center", fontSize: 12.5, color: "var(--text-tertiary)" }}>
                  Primeiro acesso?
                  <a style={{ color: "var(--brand-text)", fontWeight: 600, cursor: "pointer" }} onClick={() => setStep("onboard")}>
                    Configurar WhatsApp Business
                  </a>
                </div>
              </div>
            </>
          ) : (
            <OnboardWizard onBack={() => setStep("login")} onDone={() => router.push("/dashboard")} />
          )}
        </div>
      </div>

      <TrocaSenhaModal
        open={needChange}
        senhaAtual={senha}
        onDone={() => {
          try {
            const raw = localStorage.getItem("zg-user");
            if (raw) {
              const u = JSON.parse(raw);
              u.must_change_password = false;
              localStorage.setItem("zg-user", JSON.stringify(u));
            }
          } catch {}
          router.push("/dashboard");
        }}
      />
    </div>
  );
}

function TrocaSenhaModal({ open, senhaAtual, onDone }: { open: boolean; senhaAtual: string; onDone: () => void }) {
  const toast = useToast();
  const [nova, setNova] = useState("");
  const [conf, setConf] = useState("");
  const [saving, setSaving] = useState(false);
  const [erro, setErro] = useState("");

  async function salvar() {
    setErro("");
    if (nova.length < 8) return setErro("A nova senha precisa de ao menos 8 caracteres.");
    if (nova !== conf) return setErro("As senhas não conferem.");
    setSaving(true);
    try {
      await changePassword(senhaAtual, nova);
      toast({ type: "success", title: "Senha alterada" });
      onDone();
    } catch (e) {
      setErro((e as Error).message);
      setSaving(false);
    }
  }

  return (
    <Modal open={open} width={420}>
      <div className="modal-head">
        <div className="modal-title">Defina uma nova senha</div>
        <div className="modal-sub">Primeiro acesso — troque a senha padrão para continuar.</div>
      </div>
      <div className="modal-body col" style={{ gap: 14 }}>
        <Field label="Nova senha">
          <Input type="password" value={nova} onChange={(e) => setNova(e.target.value)} placeholder="mín. 8 caracteres" autoFocus />
        </Field>
        <Field label="Confirmar nova senha" error={erro || undefined}>
          <Input type="password" value={conf} onChange={(e) => setConf(e.target.value)} />
        </Field>
      </div>
      <div className="modal-foot">
        <Button variant="primary" className="btn-block" aria-disabled={saving || !nova || !conf} onClick={salvar}>
          {saving ? "Salvando…" : "Salvar e entrar"}
        </Button>
      </div>
    </Modal>
  );
}

function OnboardWizard({ onBack, onDone }: { onBack: () => void; onDone: () => void }) {
  const [s, setS] = useState(1);
  const [phoneId, setPhoneId] = useState("");
  const [token, setToken] = useState("");
  const [waba, setWaba] = useState("");
  const [testing, setTesting] = useState(false);
  const [ok, setOk] = useState(false);

  const testar = () => {
    setTesting(true);
    setTimeout(() => {
      setTesting(false);
      setOk(true);
    }, 1100);
  };

  return (
    <div className="fade-in">
      <button
        className="btn btn-ghost btn-sm"
        style={{ marginBottom: 18, marginLeft: -8 }}
        onClick={s === 1 ? onBack : () => setS(s - 1)}
      >
        <Icon name="arrowLeft" size={15} /> Voltar
      </button>
      <h1 style={{ fontSize: 22, fontWeight: 700 }}>Configurar WhatsApp Business</h1>
      <p style={{ fontSize: 13.5, color: "var(--text-tertiary)", marginTop: 5, marginBottom: 22 }}>
        Conecte a conta da Meta Cloud API ao ZapGov. Passo {s} de 2.
      </p>

      <div className="progress" style={{ marginBottom: 24 }}>
        <div className="progress-bar" style={{ width: (s === 1 ? 50 : 100) + "%" }} />
      </div>

      {s === 1 ? (
        <div className="col" style={{ gap: 16 }}>
          <Field label="WhatsApp Business Account ID (WABA)" hint="Encontrado no Gerenciador de Negócios da Meta.">
            <Input value={waba} onChange={(e) => setWaba(e.target.value)} placeholder="102938475610293" />
          </Field>
          <Field label="Phone Number ID">
            <Input value={phoneId} onChange={(e) => setPhoneId(e.target.value)} placeholder="109376428454521" />
          </Field>
          <Button variant="primary" size="lg" className="btn-block" onClick={() => setS(2)}>
            Continuar <Icon name="arrowRight" size={16} />
          </Button>
        </div>
      ) : (
        <div className="col" style={{ gap: 16 }}>
          <Field label="Token de acesso permanente" hint="Gerado em System Users → Generate Token.">
            <Input
              type="password"
              value={token}
              onChange={(e) => {
                setToken(e.target.value);
                setOk(false);
              }}
              placeholder="EAAG..."
            />
          </Field>
          <div
            className="card card-pad"
            style={{
              background: ok ? "var(--st-read-soft)" : "var(--bg-sunken)",
              borderColor: ok ? "var(--st-read)" : "var(--border)",
            }}
          >
            <div className="row center between">
              <div className="row center" style={{ gap: 10 }}>
                <Icon name={ok ? "checkCircle" : "link"} size={18} style={{ color: ok ? "var(--st-read)" : "var(--text-tertiary)" }} />
                <div className="col" style={{ gap: 1 }}>
                  <div style={{ fontSize: 13, fontWeight: 700 }}>{ok ? "Conexão verificada" : "Testar conexão"}</div>
                  <div style={{ fontSize: 11.5, color: "var(--text-tertiary)" }}>
                    {ok ? "Webhook e número validados" : "Valida o token e o número"}
                  </div>
                </div>
              </div>
              {!ok && (
                <Button size="sm" onClick={testar} aria-disabled={testing || !token}>
                  {testing ? "Testando…" : "Testar"}
                </Button>
              )}
            </div>
          </div>
          <Button variant="wa" size="lg" className="btn-block" onClick={onDone} aria-disabled={!ok}>
            <Icon name="check" size={16} /> Concluir configuração
          </Button>
        </div>
      )}
    </div>
  );
}
