// Client da API ZapGov (backend Go). Base configurável por env.
// Nesta fatia cobre só o inbox (conversas/mensagens/envio); demais telas seguem em mock.

const BASE = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

// ---- Tipos espelhando o JSON do backend ----
export interface ApiConversation {
  id: number;
  contact_phone: string;
  contact_name: string;
  unread_count: number;
  last_message: string;
  last_msg_at: string;
  window_expires_at: string | null;
  setor: string;
  dados: Record<string, string> | null;
  status: string; // '' | aguardando | em_atendimento | resolvido
  atendente: string;
  opt_out: boolean;
  labels: ApiLabel[] | null;
}

export interface ApiLabel {
  id: number;
  nome: string;
  cor: string;
}

export const listLabels = async () => (await get<{ labels: ApiLabel[] }>("/labels")).labels ?? [];
export const createLabel = (nome: string, cor: string) => post<ApiLabel>("/labels", { nome, cor });
export const deleteLabel = (id: number) => del(`/labels/${id}`);
export const setConversationLabels = (convId: number, labelIds: number[]) =>
  post(`/conversations/${convId}/labels`, { label_ids: labelIds }, "PUT");

export interface ApiMessage {
  id: number;
  conversation_id: number;
  wa_message_id: string;
  direction: "in" | "out" | "note";
  body: string;
  status: string; // sent|delivered|read|failed|received|nota
  created_at: string;
  media_type: string; // ''=texto; image|audio|video|document|sticker
  media_id: string;
  media_mime: string;
  media_name: string;
  autor?: string; // quem escreveu (notas internas)
}

// Baixa a mídia (com auth) e devolve um object URL para exibir.
export async function fetchMediaBlob(mediaId: string): Promise<string> {
  const res = await fetch(BASE + "/media/" + mediaId, { headers: authHeaders() });
  if (!res.ok) throw new Error("mídia indisponível");
  return URL.createObjectURL(await res.blob());
}

export async function sendMedia(input: {
  to: string;
  kind: string;
  link: string;
  caption?: string;
  name?: string;
}): Promise<ApiMessage> {
  return post<ApiMessage>("/messages/media", input);
}

// Envia um arquivo (upload direto pra Meta, sem precisar de URL pública).
export async function uploadMedia(to: string, file: File, caption?: string): Promise<ApiMessage> {
  const fd = new FormData();
  fd.append("to", to);
  if (caption) fd.append("caption", caption);
  fd.append("file", file);
  const res = await fetch(BASE + "/messages/upload", { method: "POST", headers: authHeaders(), body: fd });
  if (res.status === 401) { handle401(401); throw new Error("Não autenticado"); }
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: "Falha ao enviar" }));
    throw new Error(err.error || "Falha ao enviar");
  }
  return res.json();
}

// ---- token / sessão ----
export function getToken(): string | null {
  return typeof window === "undefined" ? null : localStorage.getItem("zg-token");
}
export function clearSession() {
  if (typeof window === "undefined") return;
  localStorage.removeItem("zg-token");
  localStorage.removeItem("zg-user");
}
function authHeaders(extra?: Record<string, string>): Record<string, string> {
  const t = getToken();
  return { ...(extra || {}), ...(t ? { Authorization: "Bearer " + t } : {}) };
}
function handle401(status: number) {
  if (status === 401 && typeof window !== "undefined" && location.pathname !== "/login") {
    clearSession();
    location.href = "/login";
  }
}

async function get<T>(path: string): Promise<T> {
  const res = await fetch(BASE + path, { cache: "no-store", headers: authHeaders() });
  if (res.status === 401) {
    handle401(401);
    throw new Error("Não autenticado");
  }
  if (!res.ok) throw new Error(`GET ${path} → ${res.status}`);
  return res.json();
}

// ---- Autenticação ----
export type Papel = "admin" | "supervisor" | "atendente";
export interface AuthUser {
  id: number;
  nome: string;
  email: string;
  papel: Papel;
  must_change_password?: boolean;
}

export async function changePassword(senha_atual: string, nova_senha: string): Promise<void> {
  await post("/auth/password", { senha_atual, nova_senha });
}

export async function login(email: string, senha: string): Promise<{ token: string; user: AuthUser }> {
  const res = await fetch(BASE + "/auth/login", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ email, senha }),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: "Falha no login" }));
    throw new Error(err.error || "Falha no login");
  }
  return res.json();
}

export async function listConversations(filtros?: { setor?: string; status?: string; q?: string; limit?: number }): Promise<ApiConversation[]> {
  const qs = new URLSearchParams();
  if (filtros?.setor && filtros.setor !== "Todos") qs.set("setor", filtros.setor);
  if (filtros?.status) qs.set("status", filtros.status);
  if (filtros?.q) qs.set("q", filtros.q);
  if (filtros?.limit) qs.set("limit", String(filtros.limit));
  const suffix = qs.toString() ? "?" + qs.toString() : "";
  const data = await get<{ conversations: ApiConversation[] }>("/conversations" + suffix);
  return data.conversations ?? [];
}

export interface ApiSession {
  id: number;
  setor: string;
  atendente: string;
  status: string; // aberta | resolvida
  dados: Record<string, string> | null;
  iniciada_em: string;
  encerrada_em: string | null;
  mensagens: number;
}

export async function listSessions(convId: number): Promise<ApiSession[]> {
  const data = await get<{ sessions: ApiSession[] }>(`/conversations/${convId}/sessions`);
  return data.sessions ?? [];
}

export async function listSessionMessages(sessionId: number): Promise<ApiMessage[]> {
  const data = await get<{ messages: ApiMessage[] }>(`/sessions/${sessionId}/messages`);
  return data.messages ?? [];
}

export async function claimConversation(id: number, atendente: string): Promise<void> {
  await post(`/conversations/${id}/claim`, { atendente }, "PUT");
}

export async function transferConversation(id: number, setor: string): Promise<void> {
  await post(`/conversations/${id}/transfer`, { setor }, "PUT");
}

export async function resolveConversation(id: number): Promise<void> {
  await post(`/conversations/${id}/resolve`, {}, "PUT");
}

export async function optOutConversation(id: number): Promise<void> {
  await post(`/conversations/${id}/optout`, {}, "PUT");
}

export async function optInConversation(id: number): Promise<void> {
  await post(`/conversations/${id}/optin`, {}, "PUT");
}

export async function resolveBulk(ids: number[]): Promise<number> {
  const r = await post<{ resolvidas: number }>("/conversations/resolve-bulk", { ids });
  return r.resolvidas;
}

export async function listConversationMessages(id: number): Promise<ApiMessage[]> {
  const data = await get<{ messages: ApiMessage[] }>(`/conversations/${id}/messages`);
  return data.messages ?? [];
}

export interface ApiTemplate {
  name: string;
  language: string;
  status: string; // aprovado | pendente | rejeitado
  category: string; // Utilidade | Marketing | Autenticação
  header: string;
  body: string;
  footer: string;
  buttons: string[] | null;
  variables: number;
  motivo?: string;
  setores: string[]; // departamentos liberados (vazio = só admin/supervisor)
}

export async function listTemplates(): Promise<ApiTemplate[]> {
  const data = await get<{ templates: ApiTemplate[] }>("/templates");
  return data.templates ?? [];
}

export async function createTemplate(input: {
  nome: string;
  idioma: string;
  categoria: string; // UTILITY | MARKETING | AUTHENTICATION
  corpo: string;
  header?: string;
  footer?: string;
  botoes?: string[];
  exemplos?: string[];
}): Promise<void> {
  await post("/templates", input);
}

export const setTemplateSetores = (name: string, setores: string[]) =>
  post(`/templates/${encodeURIComponent(name)}/setores`, { setores }, "PUT");

export interface ApiContact {
  id: number;
  nome: string;
  telefone: string;
  cpf: string;
  setor: string;
  tags: string[] | null;
  status: "ativo" | "bloqueado";
  mensagens: number;
  ultimo_contato: string | null;
  criado_em: string;
}

export async function listContacts(): Promise<ApiContact[]> {
  const data = await get<{ contacts: ApiContact[] }>("/contacts");
  return data.contacts ?? [];
}

export async function createContact(c: {
  nome: string;
  telefone: string;
  cpf?: string;
  setor?: string;
  tags?: string[];
}): Promise<ApiContact> {
  return post<ApiContact>("/contacts", c);
}

export interface ImportResult {
  inseridos: number;
  pulados: number;
  invalidos: number;
}

export async function importContacts(file: File): Promise<ImportResult> {
  const fd = new FormData();
  fd.append("file", file);
  const res = await fetch(BASE + "/contacts/import", { method: "POST", headers: authHeaders(), body: fd });
  if (res.status === 401) { handle401(401); throw new Error("Não autenticado"); }
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: "Falha na importação" }));
    throw new Error(err.error || "Falha na importação");
  }
  return res.json();
}

export interface ApiCampaign {
  id: number;
  template: string;
  language: string;
  setor: string;
  total: number;
  enviadas: number;
  entregues: number;
  lidas: number;
  falhas: number;
  status: string;
  operador: string;
  scheduled_at: string | null;
  criado_em: string;
}

export async function listCampaigns(): Promise<ApiCampaign[]> {
  const data = await get<{ campaigns: ApiCampaign[] }>("/campaigns");
  return data.campaigns ?? [];
}

// Baixa o histórico de disparos como CSV (com auth) e dispara o download.
export async function exportCampaignsCSV(): Promise<void> {
  const res = await fetch(BASE + "/campaigns/export.csv", { headers: authHeaders() });
  if (res.status === 401) { handle401(401); throw new Error("Não autenticado"); }
  if (!res.ok) throw new Error("Falha ao exportar");
  const url = URL.createObjectURL(await res.blob());
  const a = document.createElement("a");
  a.href = url;
  a.download = "disparos.csv";
  a.click();
  URL.revokeObjectURL(url);
}

export async function createCampaign(input: {
  template: string;
  language: string;
  params?: string[];
  setor?: string;
  operador?: string;
  preview?: string;
  phones: string[];
}): Promise<ApiCampaign> {
  const res = await fetch(BASE + "/campaigns", {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify(input),
  });
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
    throw new Error(err.error || `HTTP ${res.status}`);
  }
  return res.json();
}

// ---- Departamentos ----
export interface ApiDepartment {
  id: number;
  nome: string;
  cor: string;
  ativo: boolean;
  criado_em: string;
}

export async function listDepartments(): Promise<ApiDepartment[]> {
  const data = await get<{ departments: ApiDepartment[] }>("/departments");
  return data.departments ?? [];
}

export async function createDepartment(nome: string, cor: string): Promise<ApiDepartment> {
  return post<ApiDepartment>("/departments", { nome, cor });
}

export async function deleteDepartment(id: number): Promise<void> {
  await del(`/departments/${id}`);
}

// ---- Chatbot ----
export interface BotConfig {
  enabled: boolean;
  welcome: string;
  fallback: string;
}

export interface BotOption {
  id: number;
  ord: number;
  chave: string;
  label: string;
  resposta: string;
  department_id: number | null;
  department_nome: string;
  ativo: boolean;
}

export async function getBot(): Promise<{ config: BotConfig; options: BotOption[] }> {
  return get<{ config: BotConfig; options: BotOption[] }>("/bot");
}

export async function saveBotConfig(cfg: BotConfig): Promise<void> {
  await post("/bot", cfg, "PUT");
}

export async function addBotOption(o: {
  ord: number;
  chave: string;
  label: string;
  resposta: string;
  department_id: number | null;
}): Promise<BotOption> {
  return post<BotOption>("/bot/options", o);
}

export async function deleteBotOption(id: number): Promise<void> {
  await del(`/bot/options/${id}`);
}

// ---- Flow builder ----
export interface FlowNode {
  id: string;
  type: string;
  position: { x: number; y: number };
  data: Record<string, unknown>;
}

export interface FlowEdge {
  source: string;
  sourceHandle?: string | null;
  target: string;
}

export interface FlowGraph {
  nodes: FlowNode[];
  edges: FlowEdge[];
}

export interface Flow {
  id: number;
  nome: string;
  graph: FlowGraph;
  ativo: boolean;
}

export async function getActiveFlow(): Promise<Flow | null> {
  const data = await get<{ flow: Flow | null }>("/flow");
  return data.flow;
}

export async function saveFlowGraph(id: number, graph: FlowGraph): Promise<void> {
  await post(`/flows/${id}`, { graph }, "PUT");
}

// ---- Configurações (single-tenant) ----
export interface SettingsOrg {
  nome: string;
  uf: string;
  subdominio: string;
  fuso: string;
  plano: string;
  limite_mensal: number;
}
export interface SettingsMeta {
  phone_number_id: string;
  waba_id: string;
  api_version: string;
  connected: boolean;
}
export interface SettingsBilling {
  plano: string;
  limite_mensal: number;
  enviadas: number;
  entregues: number;
  lidas: number;
}
export interface SettingsWebhook {
  id: number;
  url: string;
  eventos: string[];
  ativo: boolean;
  criado_em: string;
}

// ---- Métricas (dashboard) ----
export interface ApiKpis {
  enviadasHoje: number;
  enviadasDelta: number;
  taxaEntrega: number;
  entregaDelta: number;
  taxaLeitura: number;
  leituraDelta: number;
  falhas: number;
  falhasDelta: number;
}
export interface ApiVolumePoint {
  label: string;
  value: number;
  lidas: number;
  [k: string]: string | number;
}
export interface ApiAtividade {
  tipo: "disparo" | "resposta" | "falha";
  titulo: string;
  meta: string;
  setor: string;
  status?: string;
  tempo: string;
  operador?: string;
}
export const getKpis = () => get<ApiKpis>("/metrics/kpis");
export const getVolume = async (range: "hora" | "dia") =>
  (await get<{ pontos: ApiVolumePoint[] }>(`/metrics/volume?range=${range}`)).pontos ?? [];
export const getActivity = async () =>
  (await get<{ atividade: ApiAtividade[] }>("/activity")).atividade ?? [];

export const getOrg = () => get<SettingsOrg>("/settings/org");
export const saveOrg = (o: { nome: string; uf: string; fuso: string }) => post("/settings/org", o, "PUT");
export interface SettingsEncerramento {
  ativo: boolean;
  msg: string;
}
export const getEncerramento = () => get<SettingsEncerramento>("/settings/encerramento");
export const saveEncerramento = (e: SettingsEncerramento) => post("/settings/encerramento", e, "PUT");

export interface SettingsAusencia {
  ativo: boolean;
  fim_semana: string;
  feriado: string;
  feriados: string[];
  fuso: string; // leitura (fuso da org)
  horario_ativo: boolean;
  horario_inicio: string; // "HH:MM"
  horario_fim: string;
  horario_dias: string; // "1,2,3,4,5" (0=Dom..6=Sáb)
  horario_msg: string;
}
export const getAusencia = () => get<SettingsAusencia>("/settings/ausencia");
export const saveAusencia = (a: Omit<SettingsAusencia, "fuso">) =>
  post("/settings/ausencia", a, "PUT");

export const getAssinatura = () => get<{ ativo: boolean }>("/settings/assinatura");
export const saveAssinatura = (ativo: boolean) => post("/settings/assinatura", { ativo }, "PUT");

export interface SettingsRoteamento {
  por_setor: boolean;
  por_etiqueta: boolean;
}
export const getRoteamento = () => get<SettingsRoteamento>("/settings/roteamento");
export const saveRoteamento = (r: SettingsRoteamento) => post("/settings/roteamento", r, "PUT");

export const getChatbotAtivo = () => get<{ ativo: boolean }>("/settings/chatbot");
export const saveChatbotAtivo = (ativo: boolean) => post("/settings/chatbot", { ativo }, "PUT");
export const getMeta = () => get<SettingsMeta>("/settings/meta");
export const getBilling = () => get<SettingsBilling>("/settings/billing");
export const getApiKey = () => get<{ api_key: string }>("/settings/apikey");
export const regenApiKey = () => post<{ api_key: string }>("/settings/apikey/regen", {});
export const listWebhooks = async () =>
  (await get<{ webhooks: SettingsWebhook[] }>("/settings/webhooks")).webhooks ?? [];
export const createWebhook = (url: string, eventos: string[]) =>
  post<SettingsWebhook>("/settings/webhooks", { url, eventos });
export const deleteWebhook = (id: number) => del(`/settings/webhooks/${id}`);

// ---- Usuários (admin) ----
export interface ApiUser {
  id: number;
  nome: string;
  email: string;
  papel: Papel;
  setor: string;
  ativo: boolean;
  criado_em: string;
}
export const listUsers = async () => (await get<{ users: ApiUser[] }>("/users")).users ?? [];
export const createUser = (u: { nome: string; email: string; senha: string; papel: Papel; setor?: string }) =>
  post<ApiUser>("/users", u);
export const updateUser = (id: number, u: { nome: string; email: string; papel: Papel; setor: string; ativo: boolean }) =>
  post(`/users/${id}`, u, "PUT");
export const resetUserPassword = (id: number, senha: string) =>
  post(`/users/${id}/reset-password`, { senha });

// ---- Super Admin (cross-tenant) ----
export interface ApiTenant {
  id: number;
  nome: string;
  subdominio: string;
  plano: string;
  limite_mensal: number;
  ativo: boolean;
  criado_em: string;
}
export interface TenantMeta {
  phone_number_id: string;
  waba_id: string;
  token: string;
  app_secret: string;
  api_version: string;
  webhook_verify_token: string;
}
export const listTenants = async () => (await get<{ tenants: ApiTenant[] }>("/admin/tenants")).tenants ?? [];
export const createTenant = (t: { nome: string; subdominio: string; plano?: string; admin_email?: string; admin_senha?: string }) =>
  post<ApiTenant>("/admin/tenants", t);
export const updateTenantPlano = (id: number, plano: string, limite_mensal: number) =>
  post(`/admin/tenants/${id}/plano`, { plano, limite_mensal }, "PUT");
export const setTenantMeta = (id: number, m: TenantMeta) =>
  post(`/admin/tenants/${id}/meta`, m, "PUT");
export const suspendTenant = (id: number) => post(`/admin/tenants/${id}/suspend`, {});
export const activateTenant = (id: number) => post(`/admin/tenants/${id}/activate`, {});

// ---- Auditoria ----
export interface ApiAudit {
  id: number;
  user_id: number | null;
  user_nome: string;
  papel: string;
  metodo: string;
  rota: string;
  status: number;
  criado_em: string;
}
export const listAudit = async () => (await get<{ audit: ApiAudit[] }>("/audit")).audit ?? [];

// ---- helpers de escrita ----
async function post<T>(path: string, body: unknown, method: "POST" | "PUT" = "POST"): Promise<T> {
  const res = await fetch(BASE + path, {
    method,
    headers: authHeaders({ "Content-Type": "application/json" }),
    body: JSON.stringify(body),
  });
  if (res.status === 401) {
    handle401(401);
    throw new Error("Não autenticado");
  }
  if (!res.ok) {
    const err = await res.json().catch(() => ({ error: `HTTP ${res.status}` }));
    throw new Error(err.error || `HTTP ${res.status}`);
  }
  return res.status === 204 ? (undefined as T) : res.json();
}

async function del(path: string): Promise<void> {
  const res = await fetch(BASE + path, { method: "DELETE", headers: authHeaders() });
  if (res.status === 401) handle401(401);
  if (!res.ok && res.status !== 204) throw new Error(`DELETE ${path} → ${res.status}`);
}

export async function sendMessage(to: string, body: string): Promise<ApiMessage> {
  return post<ApiMessage>("/messages/send", { to, body });
}

export async function addNote(convId: number, body: string): Promise<ApiMessage> {
  return post<ApiMessage>(`/conversations/${convId}/note`, { body });
}

// ---- Respostas rápidas (canned) ----
export interface ApiQuickReply {
  id: number;
  atalho: string;
  texto: string;
}
export const listQuickReplies = async () =>
  (await get<{ quick_replies: ApiQuickReply[] }>("/quick-replies")).quick_replies ?? [];
export const createQuickReply = (atalho: string, texto: string) =>
  post<ApiQuickReply>("/quick-replies", { atalho, texto });
export const deleteQuickReply = (id: number) => del(`/quick-replies/${id}`);

export async function sendTemplate(input: {
  to: string;
  template: string;
  language: string;
  params?: string[];
  preview?: string;
}): Promise<ApiMessage> {
  return post<ApiMessage>("/messages/template", input);
}

// ---- Mapeamento status backend → status visual (WaBubble/StatusBadge) ----
export function mapStatus(s: string): string {
  switch (s) {
    case "sent":
      return "enviado";
    case "delivered":
      return "entregue";
    case "read":
      return "lido";
    case "failed":
      return "falhou";
    case "pending":
      return "pendente";
    default:
      return "enviado";
  }
}

// HH:MM a partir de um ISO timestamp.
export function hhmm(iso: string): string {
  return new Date(iso).toLocaleTimeString("pt-BR", { hour: "2-digit", minute: "2-digit" });
}
