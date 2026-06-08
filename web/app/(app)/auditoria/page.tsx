"use client";

import { useCallback, useEffect, useState } from "react";
import { Topbar } from "@/components/shell/Topbar";
import { Avatar, Button, EmptyState, Icon, Skel } from "@/components/ui";
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
  if (m === "PUT" && /\/optout$/.test(rota)) return "Descadastrou contato (opt-out)";
  if (m === "PUT" && /\/optin$/.test(rota)) return "Recadastrou contato";
  if (m === "POST" && rota === "/messages/send") return "Enviou mensagem";
  if (m === "POST" && rota === "/messages/template") return "Enviou template";
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

export default function AuditoriaPage() {
  const [list, setList] = useState<ApiAudit[]>([]);
  const [loading, setLoading] = useState(true);
  const [erro, setErro] = useState<string | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setList(await listAudit());
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
        <div className="content-pad">
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
              <EmptyState icon="list" title="Sem registros" sub="As ações aparecem aqui conforme acontecem." />
            ) : (
              <div style={{ overflowX: "auto" }}>
                <table className="ds-table">
                  <thead>
                    <tr>
                      <th>Data/Hora</th>
                      <th>Usuário</th>
                      <th>Papel</th>
                      <th>Ação</th>
                      <th className="num">Status</th>
                    </tr>
                  </thead>
                  <tbody>
                    {list.map((a) => (
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
                        <td className="num">
                          <span className={"badge " + (a.status < 300 ? "badge-read" : "badge-failed")}>{a.status}</span>
                        </td>
                      </tr>
                    ))}
                  </tbody>
                </table>
              </div>
            )}
          </div>
        </div>
      </div>
    </>
  );
}
