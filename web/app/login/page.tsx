"use client";

import { useEffect, useState } from "react";
import { useRouter } from "next/navigation";
import { Alert, Button, Field, Input, Icon, useToast } from "@/components/ui";
import { useTheme } from "@/lib/theme";
import { login, changePassword, listTenantOptions, type TenantOption } from "@/lib/api";

export default function LoginPage() {
  const router = useRouter();
  const { theme, toggleTheme } = useTheme();
  const [email, setEmail] = useState("");
  const [senha, setSenha] = useState("");
  const [verSenha, setVerSenha] = useState(false);
  const [err, setErr] = useState("");
  const [loading, setLoading] = useState(false);
  const [needChange, setNeedChange] = useState(false);
  const [nomeUser, setNomeUser] = useState("");
  const [paineis, setPaineis] = useState<TenantOption[]>([]);
  const [painel, setPainel] = useState(""); // subdomínio escolhido

  // Painéis disponíveis (Prefeitura, T.I., …) — evita precisar de subdomínio no DNS.
  useEffect(() => {
    listTenantOptions()
      .then((opts) => {
        setPaineis(opts);
        if (opts[0]) setPainel(opts[0].subdominio); // reflete a seleção do select
      })
      .catch(() => setPaineis([]));
  }, []);

  const entrar = async () => {
    setErr("");
    if (!email.includes("@")) return setErr("Informe um e-mail válido, como nome@orgao.gov.br.");
    if (!senha) return setErr("Informe a senha.");
    setLoading(true);
    try {
      const { token, user } = await login(email, senha, painel || undefined);
      localStorage.setItem("zg-token", token);
      localStorage.setItem("zg-user", JSON.stringify(user));
      if (user.must_change_password) {
        setNomeUser(user.nome.split(" ")[0]);
        setNeedChange(true); // bloqueia até trocar a senha
        setLoading(false);
        return;
      }
      // Atendente não tem dashboard → cai direto na caixa de entrada.
      router.push(user.papel === "atendente" ? "/inbox" : "/dashboard");
    } catch (e) {
      setErr((e as Error).message);
      setLoading(false);
    }
  };

  const aposTroca = () => {
    let papel = "";
    try {
      const raw = localStorage.getItem("zg-user");
      if (raw) {
        const u = JSON.parse(raw);
        u.must_change_password = false;
        papel = u.papel;
        localStorage.setItem("zg-user", JSON.stringify(u));
      }
    } catch {}
    router.push(papel === "atendente" ? "/inbox" : "/dashboard");
  };

  return (
    <div className="login-wrap fade-in">
      {/* Lateral institucional */}
      <section className="login-aside" aria-label="ZapGov">
        <div className="row center" style={{ gap: 10 }}>
          <span className="sidebar-org-mark">ZG</span>
          <b style={{ fontSize: 18, letterSpacing: "-0.01em" }}>ZapGov</b>
        </div>
        <div style={{ maxWidth: 460 }}>
          <div aria-hidden="true" className="col" style={{ gap: 10, marginBottom: 36 }}>
            <span style={{ alignSelf: "flex-start", width: 220, padding: "12px 14px", borderRadius: "4px 12px 12px 12px", background: "var(--nav-hover)" }}>
              <span style={{ display: "block", height: 8, width: 150, borderRadius: 4, background: "var(--nav-text-muted)" }} />
              <span style={{ display: "block", height: 8, width: 96, marginTop: 8, borderRadius: 4, background: "var(--nav-text-muted)" }} />
            </span>
            <span style={{ alignSelf: "flex-end", width: 260, padding: "12px 14px", borderRadius: "12px 4px 12px 12px", background: "var(--brand)" }}>
              <span style={{ display: "block", height: 8, width: 210, borderRadius: 4, background: "var(--on-brand)", opacity: 0.85 }} />
              <span style={{ display: "block", height: 8, width: 140, marginTop: 8, borderRadius: 4, background: "var(--on-brand)", opacity: 0.85 }} />
            </span>
            <span style={{ alignSelf: "flex-end", width: 120, padding: "10px 14px", borderRadius: "12px 4px 12px 12px", background: "var(--wa)" }}>
              <span style={{ display: "block", height: 8, width: 70, borderRadius: 4, background: "var(--on-wa)" }} />
            </span>
          </div>
          <h1 style={{ fontSize: 36, lineHeight: "42px", fontWeight: 700, letterSpacing: "-0.02em" }}>Atendimento oficial ao cidadão pelo WhatsApp.</h1>
          <p style={{ marginTop: 12, fontSize: 15, lineHeight: "23px", color: "var(--nav-text-muted)" }}>
            Conversas, campanhas e avisos da Prefeitura em um só lugar, com cada setor no seu painel.
          </p>
        </div>
        <p style={{ fontSize: 13, lineHeight: "18px", color: "var(--nav-text-muted)" }}>
          Problemas para entrar? Fale com o Suporte de T.I. da sua Prefeitura.
          <br />
          Uso exclusivo do Município de Francisco Beltrão — PR.
        </p>
      </section>

      {/* Formulário */}
      <section className="login-form-side" aria-label="Acesso" style={{ position: "relative" }}>
        <button
          className="btn btn-ghost btn-icon"
          style={{ position: "absolute", top: 24, right: 24 }}
          onClick={toggleTheme}
          aria-label={theme === "dark" ? "Usar tema claro" : "Usar tema escuro"}
        >
          <Icon name={theme === "dark" ? "sun" : "moon"} size={16} />
        </button>
        <div className="login-card">
          {needChange ? (
            <TrocaSenha nome={nomeUser} senhaAtual={senha} onDone={aposTroca} onVoltar={() => setNeedChange(false)} />
          ) : (
            <form
              className="col"
              style={{ gap: 16 }}
              onSubmit={(e) => {
                e.preventDefault();
                entrar();
              }}
            >
              <div>
                <h2 className="t-display">Entrar</h2>
                <p style={{ marginTop: 6, color: "var(--text-secondary)" }}>Use o e-mail institucional cadastrado pelo administrador do seu órgão.</p>
              </div>
              {err && (
                <Alert tone="err">
                  <b>Não foi possível entrar.</b> {err}
                </Alert>
              )}
              {paineis.length > 1 && (
                <Field label="Painel" hint="Cada secretaria ou órgão tem o seu painel.">
                  <select className="select" value={painel} onChange={(e) => setPainel(e.target.value)}>
                    {paineis.map((p) => (
                      <option key={p.subdominio} value={p.subdominio}>
                        {p.nome}
                      </option>
                    ))}
                  </select>
                </Field>
              )}
              <Field label="E-mail">
                <Input type="email" autoComplete="username" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="nome@orgao.gov.br" error={!!err} />
              </Field>
              <Field label="Senha">
                <div className="input-wrap">
                  <input
                    className={"input" + (err ? " has-error" : "")}
                    type={verSenha ? "text" : "password"}
                    autoComplete="current-password"
                    value={senha}
                    onChange={(e) => setSenha(e.target.value)}
                    style={{ paddingRight: 40 }}
                  />
                  <button
                    type="button"
                    className="btn btn-ghost btn-icon btn-sm"
                    aria-label={verSenha ? "Ocultar senha" : "Mostrar senha"}
                    onClick={() => setVerSenha((v) => !v)}
                    style={{ position: "absolute", right: 4 }}
                  >
                    <Icon name="eye" size={16} />
                  </button>
                </div>
              </Field>
              <Button type="submit" variant="primary" size="lg" className="btn-block" disabled={loading}>
                {loading ? <span className="zg-spinner" /> : null}
                {loading ? "Entrando…" : "Entrar"}
              </Button>
            </form>
          )}
        </div>
      </section>
    </div>
  );
}

function TrocaSenha({ nome, senhaAtual, onDone, onVoltar }: { nome: string; senhaAtual: string; onDone: () => void; onVoltar: () => void }) {
  const toast = useToast();
  const [nova, setNova] = useState("");
  const [conf, setConf] = useState("");
  const [saving, setSaving] = useState(false);
  const [erro, setErro] = useState("");

  // Regras exibidas ao vivo (o backend exige 8–72 caracteres).
  const regras = [
    { ok: nova.length >= 8, txt: "Pelo menos 8 caracteres" },
    { ok: /[a-z]/.test(nova) && /[A-Z]/.test(nova), txt: "Letras maiúsculas e minúsculas" },
    { ok: /[\d\W_]/.test(nova), txt: "Um número ou símbolo" },
    { ok: !!nova && nova !== senhaAtual, txt: "Diferente da senha provisória" },
  ];
  const forca = regras.filter((r) => r.ok).length;
  const forcaCor = forca >= 4 ? "var(--st-read)" : forca >= 2 ? "var(--st-pending)" : "var(--st-failed)";

  async function salvar() {
    setErro("");
    if (nova.length < 8) return setErro("A nova senha precisa de pelo menos 8 caracteres.");
    if (nova === senhaAtual) return setErro("Escolha uma senha diferente da provisória.");
    if (nova !== conf) return setErro("As duas senhas não são iguais. Digite de novo.");
    setSaving(true);
    try {
      await changePassword(senhaAtual, nova);
      toast({ type: "success", title: "Senha criada" });
      onDone();
    } catch (e) {
      setErro((e as Error).message);
      setSaving(false);
    }
  }

  return (
    <form
      className="col"
      style={{ gap: 16 }}
      onSubmit={(e) => {
        e.preventDefault();
        salvar();
      }}
    >
      <div>
        <span className="badge badge-brand">Primeiro acesso</span>
        <h2 className="t-display" style={{ marginTop: 10 }}>
          Crie sua senha
        </h2>
        <p style={{ marginTop: 6, color: "var(--text-secondary)" }}>
          {nome ? `Olá, ${nome}! ` : ""}A senha provisória que você recebeu vale só para este primeiro acesso.
        </p>
      </div>
      {erro && <Alert tone="err">{erro}</Alert>}
      <Field label="Nova senha">
        <Input type="password" autoComplete="new-password" value={nova} onChange={(e) => setNova(e.target.value)} autoFocus />
        {nova && (
          <>
            <div aria-hidden="true" style={{ display: "grid", gridTemplateColumns: "repeat(4, minmax(0, 1fr))", gap: 4, marginTop: 4 }}>
              {[0, 1, 2, 3].map((i) => (
                <span key={i} style={{ height: 4, borderRadius: 2, background: i < forca ? forcaCor : "var(--bg-active)" }} />
              ))}
            </div>
            <span className="field-hint" style={{ color: forcaCor, fontWeight: 600 }}>
              {forca >= 4 ? "Senha forte" : forca >= 2 ? "Senha média" : "Senha fraca"}
            </span>
          </>
        )}
      </Field>
      <ul className="col" style={{ listStyle: "none", gap: 6, fontSize: 13 }}>
        {regras.map((r) => (
          <li key={r.txt} className="row center" style={{ gap: 8, color: r.ok ? "var(--st-read)" : "var(--text-tertiary)" }}>
            <Icon name={r.ok ? "checkCircle" : "x"} size={16} />
            <span style={{ color: r.ok ? "var(--text-primary)" : undefined }}>{r.txt}</span>
          </li>
        ))}
      </ul>
      <Field label="Repita a nova senha">
        <Input type="password" autoComplete="new-password" value={conf} onChange={(e) => setConf(e.target.value)} placeholder="Digite de novo" />
      </Field>
      <Button type="submit" variant="primary" size="lg" className="btn-block" disabled={saving || !nova || !conf}>
        {saving ? "Salvando…" : "Salvar e entrar"}
      </Button>
      <Button type="button" variant="ghost" size="sm" style={{ alignSelf: "center" }} onClick={onVoltar}>
        Voltar
      </Button>
    </form>
  );
}
