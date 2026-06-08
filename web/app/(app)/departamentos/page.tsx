"use client";

import { useCallback, useEffect, useState } from "react";
import { Topbar } from "@/components/shell/Topbar";
import {
  Button,
  ConfirmModal,
  EmptyState,
  Field,
  Icon,
  Input,
  Modal,
  Skel,
  useToast,
} from "@/components/ui";
import { listDepartments, createDepartment, deleteDepartment, type ApiDepartment } from "@/lib/api";

const CORES = [
  "oklch(0.6 0.14 152)",
  "oklch(0.64 0.13 78)",
  "oklch(0.6 0.13 256)",
  "oklch(0.6 0.15 25)",
  "oklch(0.6 0.14 300)",
  "oklch(0.6 0.12 160)",
  "oklch(0.58 0.15 340)",
  "oklch(0.6 0.02 264)",
];

function NovoModal({ open, onClose, onCreated }: { open: boolean; onClose: () => void; onCreated: () => void }) {
  const toast = useToast();
  const [nome, setNome] = useState("");
  const [cor, setCor] = useState(CORES[0]);
  const [saving, setSaving] = useState(false);

  useEffect(() => {
    if (open) {
      setNome("");
      setCor(CORES[0]);
    }
  }, [open]);

  async function salvar() {
    if (!nome.trim()) return;
    setSaving(true);
    try {
      await createDepartment(nome, cor);
      toast({ type: "success", title: "Departamento criado", message: nome });
      onCreated();
      onClose();
    } catch (e) {
      toast({ type: "error", title: "Erro", message: (e as Error).message });
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal open={open} onClose={onClose} width={440}>
      <div className="modal-head">
        <div className="modal-title">Novo departamento</div>
        <div className="modal-sub">Usado para organizar contatos, conversas e disparos.</div>
      </div>
      <div className="modal-body col" style={{ gap: 16 }}>
        <Field label="Nome">
          <Input value={nome} onChange={(e) => setNome(e.target.value)} placeholder="ex: Iluminação Pública" autoFocus />
        </Field>
        <Field label="Cor">
          <div className="row wrap" style={{ gap: 8 }}>
            {CORES.map((c) => (
              <button
                key={c}
                onClick={() => setCor(c)}
                style={{
                  width: 30,
                  height: 30,
                  borderRadius: 8,
                  background: c,
                  border: cor === c ? "2px solid var(--text-primary)" : "2px solid transparent",
                  cursor: "pointer",
                }}
              />
            ))}
          </div>
        </Field>
      </div>
      <div className="modal-foot">
        <Button variant="ghost" onClick={onClose}>
          Cancelar
        </Button>
        <Button variant="primary" icon="check" aria-disabled={saving || !nome} onClick={salvar}>
          {saving ? "Salvando…" : "Criar"}
        </Button>
      </div>
    </Modal>
  );
}

export default function DepartamentosPage() {
  const toast = useToast();
  const [list, setList] = useState<ApiDepartment[]>([]);
  const [loading, setLoading] = useState(true);
  const [erro, setErro] = useState<string | null>(null);
  const [novo, setNovo] = useState(false);
  const [delAlvo, setDelAlvo] = useState<ApiDepartment | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setList(await listDepartments());
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

  async function remover() {
    if (!delAlvo) return;
    try {
      await deleteDepartment(delAlvo.id);
      toast({ type: "success", title: "Departamento removido" });
      setDelAlvo(null);
      load();
    } catch (e) {
      toast({ type: "error", title: "Erro", message: (e as Error).message });
    }
  }

  return (
    <>
      <Topbar
        title="Departamentos"
        sub="Setores que organizam o atendimento do seu órgão"
        actions={
          <Button variant="primary" icon="plus" onClick={() => setNovo(true)}>
            Novo departamento
          </Button>
        }
      />
      <div className="content">
        <div className="content-pad col" style={{ gap: 16, maxWidth: 720 }}>
          {erro ? (
            <div className="card">
              <EmptyState icon="alertCircle" title="API indisponível" sub={erro} action={<Button size="sm" onClick={load}>Tentar de novo</Button>} />
            </div>
          ) : loading ? (
            <div className="card card-pad col" style={{ gap: 12 }}>
              {[0, 1, 2].map((i) => (
                <Skel key={i} h={24} />
              ))}
            </div>
          ) : list.length === 0 ? (
            <div className="card">
              <EmptyState icon="building" title="Nenhum departamento" sub="Crie o primeiro setor." />
            </div>
          ) : (
            <div className="card">
              {list.map((d, i) => (
                <div
                  key={d.id}
                  className="row between center"
                  style={{ padding: "13px 18px", borderBottom: i < list.length - 1 ? "1px solid var(--border-subtle)" : "none" }}
                >
                  <div className="row center" style={{ gap: 12 }}>
                    <span style={{ width: 12, height: 12, borderRadius: 4, background: d.cor, flex: "none" }} />
                    <span className="strong" style={{ fontSize: 14 }}>
                      {d.nome}
                    </span>
                  </div>
                  <button className="btn btn-ghost btn-icon btn-sm" title="Remover" onClick={() => setDelAlvo(d)}>
                    <Icon name="trash" size={15} />
                  </button>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      <NovoModal open={novo} onClose={() => setNovo(false)} onCreated={load} />
      <ConfirmModal
        open={!!delAlvo}
        onClose={() => setDelAlvo(null)}
        onConfirm={remover}
        danger
        icon="trash"
        title={"Remover " + (delAlvo?.nome ?? "") + "?"}
        message="Contatos e conversas nesse setor não são apagados, mas perdem a referência."
        confirmLabel="Remover"
      />
    </>
  );
}
