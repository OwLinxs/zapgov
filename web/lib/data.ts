// Dados mockados PT-BR (portados do handoff). Substituir por API quando o backend integrar.

export const TENANT = {
  nome: "Prefeitura de Ribeirão Preto",
  subdominio: "ribeiraopreto",
  plano: "Gov Pro",
  numero: "+55 16 3977-9000",
  phoneId: "109376428...4521",
  operador: {
    nome: "Mariana Costa",
    cargo: "Operadora Sênior",
    email: "mariana.costa@ribeiraopreto.sp.gov.br",
  },
  metaStatus: "ativo" as "ativo" | "erro" | "atencao",
};

export const SETORES = [
  "Saúde",
  "Tributos",
  "Educação",
  "Trânsito",
  "Assistência Social",
  "Meio Ambiente",
  "Geral",
];

export const SETOR_COR: Record<string, string> = {
  Saúde: "oklch(0.6 0.14 152)",
  Tributos: "oklch(0.64 0.13 78)",
  Educação: "oklch(0.6 0.13 256)",
  Trânsito: "oklch(0.6 0.15 25)",
  "Assistência Social": "oklch(0.6 0.14 300)",
  "Meio Ambiente": "oklch(0.6 0.12 160)",
  Geral: "oklch(0.6 0.02 264)",
};

export const KPIS = {
  enviadasHoje: 8472,
  enviadasDelta: 12.4,
  taxaEntrega: 97.8,
  entregaDelta: 0.6,
  taxaLeitura: 84.3,
  leituraDelta: -1.2,
  falhas: 187,
  falhasDelta: -8.1,
};

export interface VolumePoint {
  label: string;
  value: number;
  lidas: number;
  [k: string]: string | number;
}

export const VOLUME_HORA: VolumePoint[] = [
  { label: "08h", value: 420, lidas: 310 },
  { label: "09h", value: 980, lidas: 760 },
  { label: "10h", value: 1340, lidas: 1080 },
  { label: "11h", value: 1180, lidas: 990 },
  { label: "12h", value: 640, lidas: 480 },
  { label: "13h", value: 720, lidas: 560 },
  { label: "14h", value: 1520, lidas: 1290 },
  { label: "15h", value: 1090, lidas: 870 },
  { label: "16h", value: 582, lidas: 410 },
];

export const VOLUME_DIA: VolumePoint[] = [
  { label: "Sex", value: 6200, lidas: 5100 },
  { label: "Sáb", value: 2100, lidas: 1600 },
  { label: "Dom", value: 980, lidas: 700 },
  { label: "Seg", value: 7400, lidas: 6200 },
  { label: "Ter", value: 8900, lidas: 7400 },
  { label: "Qua", value: 8472, lidas: 6800 },
];

export interface Atividade {
  tipo: "disparo" | "resposta" | "falha";
  titulo: string;
  meta: string;
  setor: string;
  status?: string;
  tempo: string;
  operador?: string;
}

export const ATIVIDADE: Atividade[] = [
  { tipo: "disparo", titulo: 'Disparo "Vacinação Gripe 2026"', meta: "4.210 destinatários", setor: "Saúde", status: "entregue", tempo: "há 14 min", operador: "Mariana Costa" },
  { tipo: "resposta", titulo: "João Pereira da Silva respondeu", meta: '"Já posso comparecer hoje?"', setor: "Saúde", tempo: "há 22 min" },
  { tipo: "disparo", titulo: 'Disparo "2ª via IPTU vencendo"', meta: "1.890 destinatários", setor: "Tributos", status: "enviado", tempo: "há 38 min", operador: "Carlos Andrade" },
  { tipo: "resposta", titulo: "Ana Beatriz Souza respondeu", meta: '"Obrigada pelo aviso!"', setor: "Tributos", tempo: "há 41 min" },
  { tipo: "falha", titulo: '12 falhas no disparo "Matrícula Escolar"', meta: "Números inválidos", setor: "Educação", status: "falhou", tempo: "há 1 h" },
  { tipo: "disparo", titulo: 'Disparo "Coleta seletiva — novo horário"', meta: "780 destinatários", setor: "Meio Ambiente", status: "lido", tempo: "há 2 h", operador: "Mariana Costa" },
  { tipo: "resposta", titulo: "Roberto Carlos Lima respondeu", meta: '"Qual o telefone da secretaria?"', setor: "Geral", tempo: "há 2 h" },
];

export const NOMES = [
  "João Pereira da Silva", "Ana Beatriz Souza", "Roberto Carlos Lima", "Fernanda Oliveira Santos",
  "Marcos Vinícius Costa", "Juliana Almeida Ferreira", "Pedro Henrique Rocha", "Camila Rodrigues Dias",
  "Lucas Gabriel Martins", "Patrícia Gomes Barbosa", "Rafael Augusto Nunes", "Beatriz Carvalho Melo",
  "Gustavo Henrique Pires", "Larissa Fernandes Cruz", "Eduardo Ramos Teixeira", "Mariana Lopes Cardoso",
  "Thiago Moreira Azevedo", "Vanessa Ribeiro Castro", "Felipe Santana Moura", "Aline Cristina Freitas",
];

export function gerarTelefone(i: number): string {
  return `+55 16 9${((8000 + (i * 137) % 2000)).toString().padStart(4, "0")}-${((1000 + (i * 311) % 9000)).toString().padStart(4, "0")}`;
}
export function gerarCPF(i: number): string {
  const n = (10000000000 + (i * 73218941) % 89999999999).toString().padStart(11, "0");
  return `${n.slice(0, 3)}.${n.slice(3, 6)}.${n.slice(6, 9)}-${n.slice(9, 11)}`;
}

export interface Conversa {
  id: number;
  nome: string;
  telefone: string;
  cpf: string;
  setor: string;
  ultima: string;
  horario: string;
  naoLida: boolean;
  naoLidaCount: number;
  online: boolean;
}

const ULTIMAS = [
  "Já posso comparecer hoje?", "Obrigada pelo aviso!", "Qual o telefone da secretaria?",
  "Não consegui acessar o link", "Bom dia, está confirmado?", "Preciso remarcar minha consulta",
  "Onde retiro o documento?", "Já efetuei o pagamento, obrigado", "Pode me enviar de novo?",
  "Recebi, muito obrigado!", "Qual o horário de atendimento?", "Ainda não recebi o boleto",
];
const HORARIOS = ["14:32", "14:18", "13:55", "13:40", "12:22", "11:48", "11:30", "10:15", "09:50", "09:22", "08:47", "08:10"];

export const CONVERSAS: Conversa[] = NOMES.slice(0, 12).map((nome, i) => ({
  id: i + 1,
  nome,
  telefone: gerarTelefone(i),
  cpf: gerarCPF(i),
  setor: SETORES[i % SETORES.length],
  ultima: ULTIMAS[i],
  horario: HORARIOS[i],
  naoLida: i < 4,
  naoLidaCount: i < 4 ? [3, 1, 2, 1][i] : 0,
  online: i < 2,
}));

export interface MensagemHist {
  dir: "in" | "out";
  text: string;
  time: string;
  status?: string;
}

export const HISTORICO_CONVERSA: Record<number, MensagemHist[]> = {
  1: [
    { dir: "out", text: "Olá João! A Secretaria de Saúde informa: campanha de Vacinação contra a Gripe 2026 aberta. Compareça ao posto mais próximo com documento e cartão SUS.", time: "09:14", status: "lido" },
    { dir: "in", text: "Bom dia! Preciso agendar ou é por ordem de chegada?", time: "14:20" },
    { dir: "out", text: "Por ordem de chegada, das 8h às 17h. Não é necessário agendamento.", time: "14:28", status: "lido" },
    { dir: "in", text: "Já posso comparecer hoje?", time: "14:32" },
  ],
};

export interface Disparo {
  id: string;
  data: string;
  template: string;
  setor: string;
  enviadas: number;
  entregues: number;
  lidas: number;
  falhas: number;
  operador: string;
  status: string;
  agendadoPara?: string;
}

export const DISPAROS: Disparo[] = [
  { id: "DSP-20260", data: "03/06/2026 14:12", template: "Vacinação Gripe 2026", setor: "Saúde", enviadas: 4210, entregues: 4156, lidas: 3502, falhas: 54, operador: "Mariana Costa", status: "entregue" },
  { id: "DSP-20259", data: "03/06/2026 11:30", template: "2ª via IPTU vencendo", setor: "Tributos", enviadas: 1890, entregues: 1847, lidas: 1203, falhas: 43, operador: "Carlos Andrade", status: "entregue" },
  { id: "DSP-20258", data: "03/06/2026 09:05", template: "Matrícula Escolar 2026", setor: "Educação", enviadas: 980, entregues: 968, lidas: 740, falhas: 12, operador: "Mariana Costa", status: "entregue" },
  { id: "DSP-20257", data: "02/06/2026 16:40", template: "Coleta seletiva — novo horário", setor: "Meio Ambiente", enviadas: 780, entregues: 771, lidas: 690, falhas: 9, operador: "Mariana Costa", status: "lido" },
  { id: "DSP-20256", data: "02/06/2026 10:18", template: "Convocação — Assistência Social", setor: "Assistência Social", enviadas: 320, entregues: 312, lidas: 198, falhas: 8, operador: "Patrícia Gomes", status: "entregue" },
  { id: "DSP-20255", data: "01/06/2026 15:22", template: "Multa de trânsito — notificação", setor: "Trânsito", enviadas: 2140, entregues: 2089, lidas: 1340, falhas: 51, operador: "Carlos Andrade", status: "entregue" },
  { id: "DSP-20254", data: "01/06/2026 08:00", template: "Lembrete consulta agendada", setor: "Saúde", enviadas: 540, entregues: 538, lidas: 510, falhas: 2, operador: "Mariana Costa", status: "lido" },
  { id: "DSP-20253", data: "31/05/2026 14:50", template: "Campanha agasalho 2026", setor: "Assistência Social", enviadas: 1200, entregues: 0, lidas: 0, falhas: 0, operador: "Mariana Costa", status: "agendado", agendadoPara: "05/06/2026 09:00" },
];

export interface Template {
  id: string;
  nome: string;
  categoria: "Utilidade" | "Marketing";
  idioma: string;
  status: "aprovado" | "pendente" | "rejeitado";
  variaveis: number;
  usos: number;
  criado: string;
  motivo?: string;
  corpo: string;
  header: string | null;
  botoes: string[];
}

export const TEMPLATES: Template[] = [
  { id: "TPL-001", nome: "Vacinação Gripe 2026", categoria: "Utilidade", idioma: "pt_BR", status: "aprovado", variaveis: 1, usos: 4210, criado: "20/05/2026", corpo: "Olá {{1}}! A Secretaria de Saúde informa: campanha de Vacinação contra a Gripe 2026 aberta. Compareça ao posto mais próximo com documento e cartão SUS.", header: "Secretaria de Saúde", botoes: ["Ver postos próximos"] },
  { id: "TPL-002", nome: "2ª via IPTU vencendo", categoria: "Utilidade", idioma: "pt_BR", status: "aprovado", variaveis: 3, usos: 1890, criado: "18/05/2026", corpo: "Prezado(a) {{1}}, seu IPTU de inscrição {{2}} vence em {{3}}. Emita a 2ª via pelo portal ou no atendimento presencial.", header: "Secretaria de Tributos", botoes: ["Emitir 2ª via"] },
  { id: "TPL-003", nome: "Matrícula Escolar 2026", categoria: "Utilidade", idioma: "pt_BR", status: "aprovado", variaveis: 2, usos: 980, criado: "14/05/2026", corpo: "Olá {{1}}! As matrículas para {{2}} estão abertas até 15/06. Acesse o portal da Educação para garantir a vaga.", header: null, botoes: [] },
  { id: "TPL-004", nome: "Pesquisa de satisfação", categoria: "Marketing", idioma: "pt_BR", status: "pendente", variaveis: 1, usos: 0, criado: "02/06/2026", corpo: "Olá {{1}}, sua opinião importa! Avalie o atendimento que você recebeu nesta semana respondendo em 1 minuto.", header: null, botoes: ["Responder pesquisa"] },
  { id: "TPL-005", nome: "Promoção parcelamento dívida", categoria: "Marketing", idioma: "pt_BR", status: "rejeitado", variaveis: 2, usos: 0, criado: "28/05/2026", motivo: "Conteúdo promocional sem opt-in claro", corpo: "Aproveite! {{1}}, regularize sua dívida com até {{2}}% de desconto. Oferta por tempo limitado!", header: null, botoes: ["Quero o desconto"] },
  { id: "TPL-006", nome: "Lembrete consulta agendada", categoria: "Utilidade", idioma: "pt_BR", status: "aprovado", variaveis: 3, usos: 540, criado: "10/05/2026", corpo: "Olá {{1}}, lembrete da sua consulta em {{2}} às {{3}}. Em caso de falta, reagende pelo posto.", header: "Secretaria de Saúde", botoes: ["Confirmar", "Reagendar"] },
];

export type Papel = "admin" | "operador" | "visualizador";

export interface Usuario {
  nome: string;
  email: string;
  papel: Papel;
  ativo: boolean;
  ultimoAcesso: string;
}

export const USUARIOS: Usuario[] = [
  { nome: "Mariana Costa", email: "mariana.costa@ribeiraopreto.sp.gov.br", papel: "admin", ativo: true, ultimoAcesso: "agora" },
  { nome: "Carlos Andrade", email: "carlos.andrade@ribeiraopreto.sp.gov.br", papel: "operador", ativo: true, ultimoAcesso: "há 2 h" },
  { nome: "Patrícia Gomes", email: "patricia.gomes@ribeiraopreto.sp.gov.br", papel: "operador", ativo: true, ultimoAcesso: "ontem" },
  { nome: "Ricardo Tavares", email: "ricardo.tavares@ribeiraopreto.sp.gov.br", papel: "visualizador", ativo: true, ultimoAcesso: "há 3 dias" },
  { nome: "Helena Marques", email: "helena.marques@ribeiraopreto.sp.gov.br", papel: "operador", ativo: false, ultimoAcesso: "há 1 mês" },
];

export const PAPEIS: Record<Papel, { label: string; cls: string }> = {
  admin: { label: "Administrador", cls: "badge-brand" },
  operador: { label: "Operador", cls: "badge-delivered" },
  visualizador: { label: "Visualizador", cls: "badge-neutral" },
};

export interface Webhook {
  url: string;
  eventos: string[];
  status: string;
}

export const WEBHOOKS: Webhook[] = [
  { url: "https://api.ribeiraopreto.sp.gov.br/hooks/whatsapp", eventos: ["messages", "message_status"], status: "ativo" },
  { url: "https://integrador.gov.br/zapgov/inbound", eventos: ["messages"], status: "ativo" },
];

export interface TenantRow {
  nome: string;
  subdominio: string;
  plano: "Gov Pro" | "Business" | "Enterprise";
  status: "ativo" | "trial" | "suspenso";
  usoMes: number;
  limite: number;
  mrr: number;
  desde: string;
  uf: string;
}

export const TENANTS: TenantRow[] = [
  { nome: "Prefeitura de Ribeirão Preto", subdominio: "ribeiraopreto", plano: "Gov Pro", status: "ativo", usoMes: 184200, limite: 250000, mrr: 4990, desde: "01/2025", uf: "SP" },
  { nome: "Prefeitura de Uberlândia", subdominio: "uberlandia", plano: "Gov Pro", status: "ativo", usoMes: 142800, limite: 250000, mrr: 4990, desde: "03/2025", uf: "MG" },
  { nome: "Saneamento Águas do Vale", subdominio: "aguasdovale", plano: "Business", status: "ativo", usoMes: 89400, limite: 120000, mrr: 2490, desde: "06/2025", uf: "SC" },
  { nome: "Prefeitura de Sorocaba", subdominio: "sorocaba", plano: "Gov Pro", status: "trial", usoMes: 12300, limite: 50000, mrr: 0, desde: "05/2026", uf: "SP" },
  { nome: "Câmara Municipal de Londrina", subdominio: "camara-londrina", plano: "Business", status: "ativo", usoMes: 34200, limite: 120000, mrr: 2490, desde: "11/2025", uf: "PR" },
  { nome: "Energisa Atendimento", subdominio: "energisa", plano: "Enterprise", status: "ativo", usoMes: 421000, limite: 600000, mrr: 12900, desde: "02/2025", uf: "MS" },
  { nome: "Prefeitura de Caxias do Sul", subdominio: "caxias", plano: "Business", status: "suspenso", usoMes: 0, limite: 120000, mrr: 2490, desde: "08/2025", uf: "RS" },
  { nome: "IPTU Fácil — Consórcio ABC", subdominio: "consorcioabc", plano: "Gov Pro", status: "trial", usoMes: 4800, limite: 50000, mrr: 0, desde: "28/05/2026", uf: "SP" },
];

export interface Contato {
  id: number;
  nome: string;
  telefone: string;
  cpf: string;
  setor: string;
  tags: string[];
  status: "ativo" | "bloqueado";
  ultimoContato: string;
  mensagens: number;
}

const TAG_SETS = [["VIP", "Idoso"], ["Optin"], ["Empresa"], ["Optin", "VIP"], []];
const ULTIMO_CONTATO = ["hoje", "ontem", "há 3 dias", "há 1 semana", "há 2 semanas"];

export const CONTATOS: Contato[] = NOMES.map((nome, i) => ({
  id: i + 1,
  nome,
  telefone: gerarTelefone(i),
  cpf: gerarCPF(i),
  setor: SETORES[i % SETORES.length],
  tags: TAG_SETS[i % 5],
  status: i % 7 === 0 ? "bloqueado" : "ativo",
  ultimoContato: ULTIMO_CONTATO[i % 5],
  mensagens: 5 + (i * 7) % 40,
}));
