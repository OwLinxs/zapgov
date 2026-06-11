"use client";

import { useState } from "react";
import { useRouter } from "next/navigation";
import { Logo } from "@/components/shell/Logo";
import { Button, Field, Input, Icon, Modal, useToast } from "@/components/ui";
import { useTheme } from "@/lib/theme";
import { login, changePassword } from "@/lib/api";

export default function LoginPage() {
  const router = useRouter();
  const { theme, toggleTheme } = useTheme();
  const [email, setEmail] = useState("");
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
            Disparos, atendimento e relatórios auditáveis sobre a API oficial da Meta.
            Seguro, rastreável e em conformidade com a LGPD.
          </p>
        </div>
        <div style={{ position: "relative", fontSize: 11, color: "var(--text-faint)", lineHeight: 1.5 }}>
          Sistema desenvolvido para uso exclusivo do Município de Francisco Beltrão — PR.
          <br />Revenda e redistribuição proibidas. Todos os direitos reservados.
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

          <h1 style={{ fontSize: 23, fontWeight: 700 }}>Acessar painel</h1>
          <p style={{ fontSize: 13.5, color: "var(--text-tertiary)", marginTop: 5, marginBottom: 26 }}>
            Entre com suas credenciais.
          </p>

          <div className="col" style={{ gap: 16 }}>
            <Field label="E-mail">
              <Input icon="user" type="email" value={email} onChange={(e) => setEmail(e.target.value)} placeholder="nome@orgao.gov.br" />
            </Field>
            <Field label="Senha" error={err || undefined}>
              <Input
                icon="key"
                type="password"
                value={senha}
                onChange={(e) => setSenha(e.target.value)}
                placeholder="••••••••"
                onKeyDown={(e) => e.key === "Enter" && entrar()}
              />
            </Field>
            <label className="row center" style={{ gap: 7, fontSize: 12.5, color: "var(--text-secondary)", cursor: "pointer" }}>
              <input type="checkbox" className="checkbox" defaultChecked /> Manter conectado
            </label>
            <Button variant="primary" size="lg" className="btn-block" onClick={entrar} aria-disabled={loading}>
              {loading ? "Entrando…" : "Entrar no painel"}
              {!loading && <Icon name="arrowRight" size={16} />}
            </Button>
          </div>
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
