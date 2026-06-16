"use client";

import { useCallback, useEffect, useState } from "react";
import ReactFlow, {
  Background,
  Controls,
  MiniMap,
  addEdge,
  useNodesState,
  useEdgesState,
  type Connection,
  type Edge,
  type Node,
} from "reactflow";
import "reactflow/dist/style.css";
import { Button, Icon, Field, Input, useToast } from "@/components/ui";
import { nodeTypes, NODE_META } from "./nodes";
import { getActiveFlow, saveFlowGraph, createFlow, listDepartments, type FlowGraph, type ApiDepartment, type Flow } from "@/lib/api";

let seq = 0;
const newId = (type: string) => `${type}_${Date.now()}_${seq++}`;

function edgeId(e: { source: string; sourceHandle?: string | null; target: string }) {
  return `${e.source}:${e.sourceHandle ?? ""}->${e.target}`;
}

const PALETTE: string[] = ["message", "menu", "input", "condition", "action", "handoff", "end"];

export default function FlowCanvas() {
  const toast = useToast();
  const [flowId, setFlowId] = useState<number | null>(null);
  const [nodes, setNodes, onNodesChange] = useNodesState([]);
  const [edges, setEdges, onEdgesChange] = useEdgesState([]);
  const [selId, setSelId] = useState<string | null>(null);
  const [deps, setDeps] = useState<ApiDepartment[]>([]);
  const [saving, setSaving] = useState(false);
  const [erro, setErro] = useState<string | null>(null);
  const [semFluxo, setSemFluxo] = useState(false); // tenant ainda não tem fluxo
  const [criando, setCriando] = useState(false);

  // Aplica um fluxo carregado ao canvas.
  const aplicarFluxo = useCallback((fl: Flow) => {
    setSemFluxo(false);
    setErro(null);
    setFlowId(fl.id);
    setNodes(fl.graph.nodes.map((n) => ({ id: n.id, type: n.type, position: n.position, data: n.data })));
    setEdges(fl.graph.edges.map((e) => ({ id: edgeId(e), source: e.source, sourceHandle: e.sourceHandle ?? undefined, target: e.target })));
  }, [setNodes, setEdges]);

  useEffect(() => {
    getActiveFlow()
      .then((fl) => {
        if (!fl) { setSemFluxo(true); return; }
        aplicarFluxo(fl);
      })
      .catch((e) => setErro((e as Error).message));
    listDepartments().then(setDeps).catch(() => {});
  }, [aplicarFluxo]);

  // Cria o primeiro fluxo do tenant (órgão recém-provisionado, ex.: T.I.).
  const criarFluxo = async () => {
    setCriando(true);
    try {
      aplicarFluxo(await createFlow("Fluxo de atendimento"));
    } catch (e) {
      toast({ type: "error", title: "Erro ao criar fluxo", message: (e as Error).message });
    } finally {
      setCriando(false);
    }
  };

  const onConnect = useCallback(
    (c: Connection) => setEdges((eds) => addEdge({ ...c, id: edgeId(c as any) }, eds)),
    [setEdges],
  );

  const addNode = (type: string) => {
    const id = newId(type);
    setNodes((nds) => [
      ...nds,
      {
        id,
        type,
        position: { x: 120 + Math.random() * 80, y: 120 + nds.length * 20 },
        data: structuredClone(NODE_META[type].defaultData),
      },
    ]);
    setSelId(id);
  };

  const patchData = (patch: Record<string, unknown>) => {
    if (!selId) return;
    setNodes((nds) => nds.map((n) => (n.id === selId ? { ...n, data: { ...n.data, ...patch } } : n)));
  };

  const removeSelected = () => {
    if (!selId) return;
    setNodes((nds) => nds.filter((n) => n.id !== selId));
    setEdges((eds) => eds.filter((e) => e.source !== selId && e.target !== selId));
    setSelId(null);
  };

  const salvar = async () => {
    if (flowId == null) return;
    setSaving(true);
    const graph: FlowGraph = {
      nodes: nodes.map((n) => ({ id: n.id, type: n.type as string, position: n.position, data: n.data })),
      edges: edges.map((e) => ({ source: e.source, sourceHandle: e.sourceHandle ?? "", target: e.target })),
    };
    try {
      await saveFlowGraph(flowId, graph);
      toast({ type: "success", title: "Fluxo salvo", message: "Já vale no atendimento." });
    } catch (e) {
      toast({ type: "error", title: "Erro ao salvar", message: (e as Error).message });
    } finally {
      setSaving(false);
    }
  };

  const selected = nodes.find((n) => n.id === selId) || null;

  if (semFluxo) {
    return (
      <div className="empty-state" style={{ height: "100%" }}>
        <div className="empty-ico">
          <Icon name="message" size={24} />
        </div>
        <div className="strong">Este painel ainda não tem um fluxo</div>
        <div className="muted" style={{ fontSize: 13, marginTop: 4 }}>Crie o fluxo do chatbot para começar a montar.</div>
        <Button variant="primary" icon="plus" style={{ marginTop: 14 }} aria-disabled={criando} onClick={criarFluxo}>
          {criando ? "Criando…" : "Criar fluxo"}
        </Button>
      </div>
    );
  }

  if (erro) {
    return (
      <div className="empty-state" style={{ height: "100%" }}>
        <div className="empty-ico">
          <Icon name="alertCircle" size={24} />
        </div>
        <div className="strong">{erro}</div>
      </div>
    );
  }

  return (
    <div style={{ display: "flex", height: "100%", minHeight: 0 }}>
      {/* Paleta */}
      <div style={{ width: 150, flex: "none", borderRight: "1px solid var(--border-subtle)", padding: 12, display: "flex", flexDirection: "column", gap: 6, background: "var(--bg-sunken)" }}>
        <div className="field-label" style={{ marginBottom: 4 }}>
          Blocos
        </div>
        {PALETTE.map((t) => (
          <button key={t} className="btn btn-secondary btn-sm" style={{ justifyContent: "flex-start" }} onClick={() => addNode(t)}>
            <Icon name={NODE_META[t].icon} size={14} />
            {NODE_META[t].label}
          </button>
        ))}
        <div className="grow" />
        <Button variant="primary" icon="check" aria-disabled={saving} onClick={salvar}>
          {saving ? "Salvando…" : "Salvar"}
        </Button>
      </div>

      {/* Canvas */}
      <div style={{ flex: 1, minWidth: 0, position: "relative" }}>
        <ReactFlow
          nodes={nodes}
          edges={edges}
          onNodesChange={onNodesChange}
          onEdgesChange={onEdgesChange}
          onConnect={onConnect}
          onNodeClick={(_, n) => setSelId(n.id)}
          onPaneClick={() => setSelId(null)}
          nodeTypes={nodeTypes}
          fitView
          proOptions={{ hideAttribution: true }}
        >
          <Background color="var(--border)" gap={18} />
          <Controls showInteractive={false} />
          <MiniMap pannable zoomable style={{ background: "var(--bg-surface)" }} />
        </ReactFlow>
      </div>

      {/* Painel de edição */}
      {selected && (
        <EditPanel
          key={selected.id}
          node={selected}
          deps={deps}
          onPatch={patchData}
          onRemove={removeSelected}
          onClose={() => setSelId(null)}
        />
      )}
    </div>
  );
}

function EditPanel({
  node,
  deps,
  onPatch,
  onRemove,
  onClose,
}: {
  node: Node;
  deps: ApiDepartment[];
  onPatch: (p: Record<string, unknown>) => void;
  onRemove: () => void;
  onClose: () => void;
}) {
  const d = node.data as any;
  const meta = NODE_META[node.type as string];

  return (
    <div style={{ width: 300, flex: "none", borderLeft: "1px solid var(--border-subtle)", padding: 16, display: "flex", flexDirection: "column", gap: 14, overflowY: "auto", background: "var(--bg-surface)" }}>
      <div className="row between center">
        <div className="row center" style={{ gap: 8 }}>
          <Icon name={meta.icon} size={15} style={{ color: meta.color }} />
          <span className="card-title">{meta.label}</span>
        </div>
        <button className="btn btn-ghost btn-icon btn-sm" onClick={onClose}>
          <Icon name="x" size={15} />
        </button>
      </div>

      {node.type === "message" && (
        <Field label="Texto">
          <textarea className="textarea" value={d.text || ""} onChange={(e) => onPatch({ text: e.target.value })} />
        </Field>
      )}

      {node.type === "input" && (
        <>
          <Field label="Pergunta">
            <textarea className="textarea" value={d.prompt || ""} onChange={(e) => onPatch({ prompt: e.target.value })} />
          </Field>
          <Field label="Salvar resposta na variável">
            <Input value={d.var || ""} onChange={(e) => onPatch({ var: e.target.value })} placeholder="nome" />
          </Field>
        </>
      )}

      {node.type === "menu" && (
        <>
          <Field label="Texto do menu">
            <textarea className="textarea" value={d.text || ""} onChange={(e) => onPatch({ text: e.target.value })} />
          </Field>
          <Field label="Salvar escolha na variável (opcional)" hint="Preenchido: vira pergunta de escolha única e grava a opção tocada (ex.: categoria). Vazio: ramifica por opção.">
            <Input value={d.var || ""} onChange={(e) => onPatch({ var: e.target.value.trim() })} placeholder="ex.: categoria" />
          </Field>
          <div className="field-label">Opções</div>
          {(d.options || []).map((o: any, i: number) => (
            <div key={i} className="row center" style={{ gap: 6 }}>
              <input
                className="input"
                style={{ width: 48 }}
                value={o.key}
                onChange={(e) => {
                  const opts = [...d.options];
                  opts[i] = { ...o, key: e.target.value };
                  onPatch({ options: opts });
                }}
              />
              <input
                className="input"
                value={o.label}
                onChange={(e) => {
                  const opts = [...d.options];
                  opts[i] = { ...o, label: e.target.value };
                  onPatch({ options: opts });
                }}
              />
              <button
                className="btn btn-ghost btn-icon btn-sm"
                onClick={() => onPatch({ options: d.options.filter((_: any, j: number) => j !== i) })}
              >
                <Icon name="trash" size={14} />
              </button>
            </div>
          ))}
          <Button
            variant="secondary"
            size="sm"
            icon="plus"
            onClick={() => onPatch({ options: [...(d.options || []), { key: String((d.options?.length || 0) + 1), label: "Nova opção" }] })}
          >
            Opção
          </Button>
        </>
      )}

      {node.type === "condition" && (
        <>
          <Field label="Variável">
            <Input value={d.var || ""} onChange={(e) => onPatch({ var: e.target.value })} />
          </Field>
          <Field label="Operador">
            <select className="select" value={d.op || "eq"} onChange={(e) => onPatch({ op: e.target.value })}>
              <option value="eq">igual a</option>
              <option value="contains">contém</option>
            </select>
          </Field>
          <Field label="Valor">
            <Input value={d.value || ""} onChange={(e) => onPatch({ value: e.target.value })} />
          </Field>
        </>
      )}

      {node.type === "action" && (
        <Field label="Tipo de ação">
          <select className="select" value={d.kind || "assign_department"} onChange={(e) => onPatch({ kind: e.target.value })}>
            <option value="assign_department">Atribuir setor</option>
            <option value="abrir_chamado">Abrir chamado (T.I.)</option>
          </select>
        </Field>
      )}

      {/* Atribuir setor (action) ou handoff: escolhe o departamento. */}
      {((node.type === "action" && (d.kind || "assign_department") === "assign_department") || node.type === "handoff") && (
        <Field label="Departamento">
          <select className="select" value={d.department || ""} onChange={(e) => onPatch({ department: e.target.value })}>
            <option value="">Nenhum</option>
            {deps.map((dep) => (
              <option key={dep.id} value={dep.nome}>
                {dep.nome}
              </option>
            ))}
          </select>
        </Field>
      )}

      {/* Abrir chamado: prioridade opcional + dica das variáveis esperadas. */}
      {node.type === "action" && d.kind === "abrir_chamado" && (
        <>
          <Field label="Prioridade">
            <select className="select" value={d.prioridade || ""} onChange={(e) => onPatch({ prioridade: e.target.value })}>
              <option value="">Padrão (média)</option>
              <option value="baixa">Baixa</option>
              <option value="media">Média</option>
              <option value="alta">Alta</option>
              <option value="critica">Crítica</option>
            </select>
          </Field>
          <p className="muted" style={{ fontSize: 12, lineHeight: 1.5 }}>
            Coloque blocos de <b>Pergunta</b> antes deste, salvando nas variáveis{" "}
            <code>titulo</code>, <code>descricao</code>, <code>categoria</code>,{" "}
            <code>secretaria</code> e <code>local</code>. O chamado é criado com elas
            e o nome/telefone de quem escreveu.
          </p>
        </>
      )}

      {(node.type === "start" || node.type === "end") && (
        <p className="muted" style={{ fontSize: 12.5 }}>
          {node.type === "start" ? "Ponto de entrada do fluxo." : "Encerra a conversa."}
        </p>
      )}

      {node.type !== "start" && (
        <button className="btn btn-danger-ghost btn-sm" onClick={onRemove} style={{ marginTop: 4 }}>
          <Icon name="trash" size={14} /> Remover bloco
        </button>
      )}
    </div>
  );
}
