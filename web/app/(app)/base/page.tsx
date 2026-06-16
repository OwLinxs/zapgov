"use client";

import { useCallback, useEffect, useState } from "react";
import { Topbar } from "@/components/shell/Topbar";
import { Button, Drawer, EmptyState, Field, Icon, Input, Modal, useToast } from "@/components/ui";
import {
  listKb, listKbCategorias, getKbArticle, createKbArticle, updateKbArticle,
  arquivaKbArticle, votaKbArticle, type ApiKbArticle,
} from "@/lib/api";
import { useAuth } from "@/lib/auth";

function trecho(s: string, n = 140) {
  const t = (s || "").trim();
  return t.length > n ? t.slice(0, n) + "…" : t;
}
function fmtData(iso: string) {
  return new Date(iso).toLocaleDateString("pt-BR", { day: "2-digit", month: "2-digit", year: "numeric" });
}

export default function BasePage() {
  const toast = useToast();
  const { user } = useAuth();
  const podeEditar = user?.papel === "admin" || user?.papel === "supervisor";
  const [q, setQ] = useState("");
  const [categoria, setCategoria] = useState("");
  const [cats, setCats] = useState<string[]>([]);
  const [list, setList] = useState<ApiKbArticle[]>([]);
  const [loading, setLoading] = useState(true);
  const [verId, setVerId] = useState<number | null>(null);
  const [editar, setEditar] = useState<ApiKbArticle | "novo" | null>(null);

  const load = useCallback(async () => {
    setLoading(true);
    try {
      setList(await listKb({ q: q.trim() || undefined, categoria: categoria || undefined }));
    } catch (e) {
      toast({ type: "error", title: "Erro ao buscar", message: (e as Error).message });
    } finally {
      setLoading(false);
    }
  }, [q, categoria, toast]);

  useEffect(() => {
    const t = setTimeout(load, 300);
    return () => clearTimeout(t);
  }, [load]);
  useEffect(() => { listKbCategorias().then(setCats).catch(() => {}); }, []);
  // Deep-link /base?art=ID abre o artigo direto (vindo da sugestão no inbox).
  useEffect(() => {
    const art = new URLSearchParams(window.location.search).get("art");
    if (art) setVerId(Number(art));
  }, []);

  return (
    <>
      <Topbar
        title="Base de Conhecimento"
        sub="Soluções registradas pelos técnicos — pesquise antes de abrir um chamado"
        actions={podeEditar ? <Button variant="primary" icon="plus" onClick={() => setEditar("novo")}>Novo artigo</Button> : undefined}
      />
      <div className="content">
        <div className="content-pad col" style={{ gap: 16, maxWidth: 880 }}>
          {/* Busca */}
          <div className="card card-pad">
            <div className="row" style={{ gap: 12, flexWrap: "wrap" }}>
              <div className="grow" style={{ minWidth: 240 }}>
                <Input icon="search" placeholder="Buscar por sintoma, palavra-chave, solução…" value={q} onChange={(e) => setQ(e.target.value)} autoFocus />
              </div>
              <select className="select" style={{ width: "auto" }} value={categoria} onChange={(e) => setCategoria(e.target.value)}>
                <option value="">Todas categorias</option>
                {cats.map((c) => <option key={c} value={c}>{c}</option>)}
              </select>
            </div>
          </div>

          {/* Resultados */}
          {loading ? (
            <div className="muted" style={{ padding: 20 }}>Carregando…</div>
          ) : list.length === 0 ? (
            <EmptyState icon="search" title={q ? "Nada encontrado" : "Base vazia"} sub={q ? "Tente outras palavras." : "Registre a primeira solução."} />
          ) : (
            <div className="col" style={{ gap: 10 }}>
              {list.map((a) => (
                <div key={a.id} className="card card-pad" style={{ cursor: "pointer" }} onClick={() => setVerId(a.id)}>
                  <div className="row between center" style={{ gap: 10 }}>
                    <span className="strong" style={{ fontSize: 14 }}>{a.titulo}</span>
                    <span className="faint" style={{ fontSize: 11, whiteSpace: "nowrap" }}>
                      <Icon name="eye" size={12} /> {a.views}
                    </span>
                  </div>
                  {a.problema && <div className="muted" style={{ fontSize: 12.5, marginTop: 4, lineHeight: 1.5 }}>{trecho(a.problema)}</div>}
                  <div className="row center wrap" style={{ gap: 6, marginTop: 8 }}>
                    {a.categoria && <span className="badge badge-neutral" style={{ fontSize: 10 }}>{a.categoria}</span>}
                    {(a.tags ?? []).map((t) => <span key={t} className="mono faint" style={{ fontSize: 10 }}>#{t}</span>)}
                    {a.ticket_id && <span className="mono faint" style={{ fontSize: 10 }}>chamado #{a.ticket_id}</span>}
                  </div>
                </div>
              ))}
            </div>
          )}
        </div>
      </div>

      {verId != null && (
        <VerArtigo id={verId} podeEditar={podeEditar} onClose={() => setVerId(null)}
          onEditar={(a) => { setVerId(null); setEditar(a); }} onMudou={load} />
      )}
      {editar != null && (
        <EditorArtigo artigo={editar === "novo" ? null : editar} onClose={() => setEditar(null)}
          onSalvo={() => { setEditar(null); load(); listKbCategorias().then(setCats).catch(() => {}); }} />
      )}
    </>
  );
}

function VerArtigo({ id, podeEditar, onClose, onEditar, onMudou }: {
  id: number; podeEditar: boolean; onClose: () => void; onEditar: (a: ApiKbArticle) => void; onMudou: () => void;
}) {
  const toast = useToast();
  const [a, setA] = useState<ApiKbArticle | null>(null);
  const [votou, setVotou] = useState(false);

  useEffect(() => { getKbArticle(id).then(setA).catch(() => setA(null)); }, [id]);

  async function votar() {
    try { await votaKbArticle(id); setVotou(true); toast({ type: "success", title: "Obrigado!" }); }
    catch { /* ignore */ }
  }
  async function arquivar() {
    try { await arquivaKbArticle(id); toast({ type: "success", title: "Artigo arquivado" }); onMudou(); onClose(); }
    catch (e) { toast({ type: "error", title: "Erro", message: (e as Error).message }); }
  }

  if (!a) return <Drawer open onClose={onClose} width={560}><div className="muted" style={{ padding: 24 }}>Carregando…</div></Drawer>;

  return (
    <Drawer open onClose={onClose} width={560}>
      <div className="row between center" style={{ marginBottom: 12 }}>
        <span className="strong" style={{ fontSize: 16 }}>{a.titulo}</span>
        <button className="btn btn-ghost btn-icon btn-sm" onClick={onClose}><Icon name="x" size={16} /></button>
      </div>
      <div className="col" style={{ gap: 16, overflowY: "auto", maxHeight: "calc(100vh - 140px)" }}>
        <div className="row center wrap" style={{ gap: 6 }}>
          {a.categoria && <span className="badge badge-neutral">{a.categoria}</span>}
          {(a.tags ?? []).map((t) => <span key={t} className="mono faint" style={{ fontSize: 11 }}>#{t}</span>)}
        </div>

        {a.problema && (
          <div className="col" style={{ gap: 4 }}>
            <span className="field-label">Problema / sintoma</span>
            <div style={{ fontSize: 13.5, lineHeight: 1.6, whiteSpace: "pre-wrap" }}>{a.problema}</div>
          </div>
        )}
        <div className="col" style={{ gap: 4 }}>
          <span className="field-label">Solução</span>
          <div style={{ fontSize: 13.5, lineHeight: 1.6, whiteSpace: "pre-wrap" }}>{a.solucao || "—"}</div>
        </div>

        <div className="muted" style={{ fontSize: 11.5 }}>
          {a.autor && <>por {a.autor} · </>}atualizado {fmtData(a.atualizado_em)} · {a.views} visualizações
          {a.ticket_id && <> · chamado #{a.ticket_id}</>}
        </div>

        <div className="row center" style={{ gap: 8 }}>
          <Button variant="secondary" size="sm" icon="checkCircle" aria-disabled={votou} onClick={votar}>
            {votou ? "Marcado como útil" : `Isto ajudou (${a.uteis})`}
          </Button>
          {podeEditar && (
            <>
              <Button variant="ghost" size="sm" icon="edit" onClick={() => onEditar(a)}>Editar</Button>
              <Button variant="danger" size="sm" icon="trash" onClick={arquivar}>Arquivar</Button>
            </>
          )}
        </div>
      </div>
    </Drawer>
  );
}

function EditorArtigo({ artigo, onClose, onSalvo }: { artigo: ApiKbArticle | null; onClose: () => void; onSalvo: () => void }) {
  const toast = useToast();
  const [titulo, setTitulo] = useState(artigo?.titulo ?? "");
  const [problema, setProblema] = useState(artigo?.problema ?? "");
  const [solucao, setSolucao] = useState(artigo?.solucao ?? "");
  const [categoria, setCategoria] = useState(artigo?.categoria ?? "");
  const [tags, setTags] = useState((artigo?.tags ?? []).join(", "));
  const [saving, setSaving] = useState(false);

  async function salvar() {
    if (!titulo.trim()) return;
    setSaving(true);
    const tagsArr = tags.split(",").map((t) => t.trim()).filter(Boolean);
    try {
      if (artigo) await updateKbArticle(artigo.id, { titulo, problema, solucao, categoria, tags: tagsArr });
      else await createKbArticle({ titulo, problema, solucao, categoria, tags: tagsArr });
      toast({ type: "success", title: artigo ? "Artigo atualizado" : "Artigo criado" });
      onSalvo();
    } catch (e) {
      toast({ type: "error", title: "Erro", message: (e as Error).message });
    } finally {
      setSaving(false);
    }
  }

  return (
    <Modal open onClose={onClose} width={620}>
      <div className="modal-head">
        <div className="modal-title">{artigo ? "Editar artigo" : "Novo artigo"}</div>
        <div className="modal-sub">Registre o problema e a solução para consulta futura.</div>
      </div>
      <div className="modal-body col" style={{ gap: 14, maxHeight: "66vh", overflowY: "auto" }}>
        <Field label="Título *">
          <Input value={titulo} onChange={(e) => setTitulo(e.target.value)} placeholder="Ex.: Impressora não imprime em rede" autoFocus />
        </Field>
        <Field label="Problema / sintoma">
          <textarea className="input" rows={3} value={problema} onChange={(e) => setProblema(e.target.value)} placeholder="Como o problema se manifesta…" />
        </Field>
        <Field label="Solução">
          <textarea className="input" rows={6} value={solucao} onChange={(e) => setSolucao(e.target.value)} placeholder="Passo a passo da resolução…" />
        </Field>
        <div className="row" style={{ gap: 12 }}>
          <Field label="Categoria">
            <Input value={categoria} onChange={(e) => setCategoria(e.target.value)} placeholder="Ex.: Impressora" />
          </Field>
          <Field label="Tags (separadas por vírgula)">
            <Input value={tags} onChange={(e) => setTags(e.target.value)} placeholder="rede, drivers, windows" />
          </Field>
        </div>
      </div>
      <div className="modal-foot">
        <Button variant="ghost" onClick={onClose}>Cancelar</Button>
        <Button variant="primary" aria-disabled={saving || !titulo.trim()} onClick={salvar}>
          {saving ? "Salvando…" : "Salvar"}
        </Button>
      </div>
    </Modal>
  );
}
