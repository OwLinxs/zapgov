"use client";

import { useCallback, useEffect, useState } from "react";
import { Topbar } from "@/components/shell/Topbar";
import { Avatar, Button, EmptyState, Icon, Input, Skel } from "@/components/ui";
import { ConversaHistorico } from "@/components/inbox/ConversaHistorico";
import { listAudit, type ApiAudit } from "@/lib/api";

const PAPEL_LABEL: Record<string, string> = {
  admin: "Administrador",
  supervisor: "Supervisor",
  atendente: "Atendente",
};

// Traduz método+rota em ação legível.
function acaoLabel(m: string, rota: string): string {
  if (m === "POST" && rota === "/campaigns") return "Criou disparo";
  if (m === "POST" && rota === "/contacts") return "Criou contato";
  if (m === "POST" && rota === "/users") return "Criou usuário";
  if (m === "PUT" && /\/users\/\d+/.test(rota)) return "Alterou usuário";
  if (m === "PUT" && /\/resolve$/.test(rota)) return "Finalizou atendimento";
  if (m === "PUT" && /\/claim$/.test(rota)) return "Pegou conversa";
  if (m === "PUT" && /\/transfer$/.test(rota)) return "Transferiu conversa";
  if (m === "PUT" && /\/optout$/.test(rota)) return "Descadastrou contato (opt-out)";
  if (m === "PUT" && /\/optin$/.test(rota)) return "Recadastrou contato";
  if (m === "POST" && rota === "/messages/send") return "Enviou mensagem";
  if (m === "POST" && rota === "/messages/template") return "Enviou template";
  if (m === "POST" && /\/comment$/.test(rota)) return "Comentou no chamado";
  if (m === "POST" && rota === "/tickets") return "Abriu chamado";
  if (m === "POST" && rota === "/departments") return "Criou departamento";
  if (m === "DELETE" && /\/departments\//.test(rota)) return "Removeu departamento";
  if (m === "PUT" && /\/flows\//.test(rota)) return "Editou fluxo do bot";
  if (m === "POST" && rota === "/auth/password") return "Trocou a própria senha";
  if (m === "POST" && /\/bot/.test(rota)) return "Configurou o bot";
  if (m === "PUT" && rota === "/settings/org") return "Editou organização";
  if (/\/settings\/webhooks/.test(rota)) return "Alterou webhooks";
  if (/\/settings\/apikey/.test(rota)) return "Rotacionou chave de API";
  return `${m} ${rota}`;
}

// Extrai o id da conversa de uma rota /conversations/:id/... (para "ver conversa").
function convIdDaRota(rota: string): number | null {
  const m = rota.match(/\/conversations\/(\d+)/);
  return m ? Number(m[1]) : null;
}

function hojeMenos(dias: number) {
  return new Date(Date.now() - dias * 864e5).toISOString().slice(0, 10);
}

export default function AuditoriaPage() {
  const [list, setList] = useState<ApiAudit[]>([]);
  const [loading, setLoading] = useState(true);
  const [erro, setErro] = useState<string | null>(null);
  const [from, setFrom] = useState(hojeMenos(30));
  const [to, setTo] = useState(hojeMenos(0));
  const [q, setQ] = useState("");
  const [verConv, setVerConv] = useState<number | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setList(await listAudit({ from, to, q: q.trim() || undefined }));
      setErro(null);
    } catch (e) {
      setErro((e as Error).message);
    } finally {
      setLoading(false);
    }
  }, [from, to, q]);

  // Recarrega ao mudar período; busca textual com debounce.
  useEffect(() => {
    const t = setTimeout(load, 300);
    return () => clearTimeout(t);
  }, [load]);

  return (
    <>
      <Topbar
        title="Auditoria"
        sub="Trilha de todas as ações que alteram dados"
        actions={
          <Button variant="secondary" icon="refresh" onClick={load}>
            Atualizar
          </Button>
        }
      />
      <div className="content">
        <div className="content-pad col" style={{ gap: 16 }}>
          {/* Filtros */}
          <div className="card card-pad">
            <div className="row" style={{ gap: 12, alignItems: "flex-end", flexWrap: "wrap" }}>
              <div className="col" style={{ gap: 4 }}>
                <span className="field-label">De</span>
                <input type="date" className="input" value={from} onChange={(e) => setFrom(e.target.value)} />
              </div>
              <div className="col" style={{ gap: 4 }}>
                <span className="field-label">Até</span>
                <input type="date" className="input" value={to} onChange={(e) => setTo(e.target.value)} />
              </div>
              <div className="grow" style={{ minWidth: 220 }}>
                <Input icon="search" placeholder="Buscar por usuário, contato/número, ação ou IP…" value={q} onChange={(e) => setQ(e.target.value)} />
              </div>
            </div>
          </div>

          <div className="card" style={{ overflow: "hidden" }}>
            {erro ? (
              <EmptyState icon="alertCircle" title="API indisponível" sub={erro} action={<Button size="sm" onClick={load}>Tentar de novo</Button>} />
            ) : loading ? (
              <div className="card-pad col" style={{ gap: 10 }}>
                {[0, 1, 2, 3].map((i) => (
                  <Skel key={i} h={24} />
                ))}
              </div>
            ) : list.length === 0 ? (
              <EmptyState icon="list" title="Sem registros" sub="Nenhuma ação no período/filtro escolhido." />
            ) : (
              <div style={{ overflowX: "auto" }}>
                <table className="ds-table">
                  <thead>
                    <tr>
                      <th>Data/Hora</th>
                      <th>Usuário</th>
                      <th>Papel</th>
                      <th>Ação</th>
                      <th>IP</th>
                      <th className="num">Status</th>
                      <th></th>
                    </tr>
                  </thead>
                  <tbody>
                    {list.map((a) => {
                      const convId = convIdDaRota(a.rota);
                      return (
                        <tr key={a.id}>
                          <td className="mono" style={{ fontSize: 11.5, whiteSpace: "nowrap" }}>
                            {new Date(a.criado_em).toLocaleString("pt-BR")}
                          </td>
                          <td className="strong">
                            <div className="row center" style={{ gap: 9 }}>
                              <Avatar name={a.user_nome || "?"} size={26} />
                              {a.user_nome || "—"}
                            </div>
                          </td>
                          <td>
                            <span className="muted" style={{ fontSize: 12.5 }}>
                              {PAPEL_LABEL[a.papel] || a.papel || "—"}
                            </span>
                          </td>
                          <td>{acaoLabel(a.metodo, a.rota)}</td>
                          <td className="mono faint" style={{ fontSize: 11.5 }}>{a.ip || "—"}</td>
                          <td className="num">
                            <span className={"badge " + (a.status < 300 ? "badge-read" : "badge-failed")}>{a.status}</span>
                          </td>
                          <td>
                            {convId && (
                              <Button variant="ghost" size="sm" icon="history" onClick={() => setVerConv(convId)}>
                                Ver conversa
                              </Button>
                            )}
                          </td>
                        </tr>
                      );
                    })}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      </div>

      {verConv != null && <ConversaHistorico convId={verConv} onClose={() => setVerConv(null)} />}
    </>
  );
}
