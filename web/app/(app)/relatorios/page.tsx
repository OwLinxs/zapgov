"use client";

import { useState } from "react";
import { Topbar } from "@/components/shell/Topbar";
import { Button, Field, Icon, useToast } from "@/components/ui";
import { downloadReport } from "@/lib/api";
import { IconName } from "@/lib/icons";

const REPORTS: { tipo: "atendentes" | "setores" | "diario" | "satisfacao"; titulo: string; sub: string; icon: IconName }[] = [
  { tipo: "atendentes", titulo: "Por atendente", sub: "Atendimentos, mensagens, tempo médio e satisfação de cada atendente.", icon: "users" },
  { tipo: "setores", titulo: "Por departamento", sub: "Volume, tempo médio e satisfação por setor.", icon: "building" },
  { tipo: "diario", titulo: "Diário", sub: "Mensagens recebidas, enviadas e falhas por dia (carga de trabalho).", icon: "calendar" },
  { tipo: "satisfacao", titulo: "Satisfação detalhada", sub: "Uma linha por pesquisa: contato, atendente, setor, nota e data.", icon: "smile" },
];

function hojeMenos(dias: number) {
  return new Date(Date.now() - dias * 24 * 3600 * 1000).toISOString().slice(0, 10);
}

export default function RelatoriosPage() {
  const toast = useToast();
  const [from, setFrom] = useState(hojeMenos(30));
  const [to, setTo] = useState(hojeMenos(0));
  const [baixando, setBaixando] = useState("");

  async function baixar(tipo: (typeof REPORTS)[number]["tipo"]) {
    setBaixando(tipo);
    try {
      await downloadReport(tipo, from, to);
    } catch (e) {
      toast({ type: "error", title: "Erro ao gerar", message: (e as Error).message });
    } finally {
      setBaixando("");
    }
  }

  return (
    <>
      <Topbar title="Relatórios" sub="Exportações em CSV (abre no Excel) por período" />
      <div className="content">
        <div className="content-pad col" style={{ gap: 16, maxWidth: 860 }}>
          <div className="card card-pad">
            <div className="row" style={{ gap: 14, alignItems: "flex-end" }}>
              <Field label="De">
                <input type="date" className="input" value={from} onChange={(e) => setFrom(e.target.value)} />
              </Field>
              <Field label="Até">
                <input type="date" className="input" value={to} onChange={(e) => setTo(e.target.value)} />
              </Field>
              <div className="muted" style={{ fontSize: 12, paddingBottom: 10 }}>
                O período vale para todos os relatórios abaixo.
              </div>
            </div>
          </div>

          <div style={{ display: "grid", gridTemplateColumns: "repeat(auto-fill, minmax(380px, 1fr))", gap: 14 }}>
            {REPORTS.map((r) => (
              <div key={r.tipo} className="card card-pad row center" style={{ gap: 14 }}>
                <span className="kpi-ico" style={{ width: 40, height: 40, background: "var(--brand-soft)", color: "var(--brand-text)" }}>
                  <Icon name={r.icon} size={19} />
                </span>
                <div className="col grow" style={{ gap: 2, minWidth: 0 }}>
                  <span className="strong" style={{ fontSize: 13.5 }}>{r.titulo}</span>
                  <span className="muted" style={{ fontSize: 12, lineHeight: 1.45 }}>{r.sub}</span>
                </div>
                <Button variant="secondary" size="sm" icon="download" aria-disabled={baixando === r.tipo} onClick={() => baixar(r.tipo)}>
                  {baixando === r.tipo ? "Gerando…" : "CSV"}
                </Button>
              </div>
            ))}
          </div>
        </div>
      </div>
    </>
  );
}
