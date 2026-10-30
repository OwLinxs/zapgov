# ZapGov

Plataforma de atendimento ao cidadão pelo WhatsApp Business (Meta Cloud API),
com inbox, setores, chatbot, templates, campanhas e relatórios. O backend usa
Go, PostgreSQL e Redis; o painel usa Next.js. Cada órgão pode ter usuários,
dados e credenciais Meta próprios.

O uso do código por órgãos e secretarias do Município de Francisco Beltrão
segue a [licença do projeto](LICENSE.md). Este README mostra como iniciar uma
**instalação própria** para outro órgão. Para usar a instalação existente como
outro painel, veja [Adicionar um órgão na mesma instalação](#adicionar-um-órgão-na-mesma-instalação).

## Antes de começar

Prepare um servidor Linux com Docker e o plugin Compose, Node.js 20, acesso ao
repositório, dois endereços HTTPS e um proxy reverso. Os exemplos usam:

| Serviço | Endereço de exemplo | Destino no servidor interno |
| --- | --- | --- |
| Painel | `https://app.ORGAO.DOMINIO` | Porta 3000 |
| API e webhook | `https://api.ORGAO.DOMINIO` | Porta 8080 |

A Meta precisa alcançar `https://api.ORGAO.DOMINIO/webhook`, e o servidor deve
alcançar a API da Meta pela porta 443. Restrinja as portas 3000 e 8080 à rede
do proxy. Há um [exemplo de configuração nginx](deploy/nginx-dmz.conf) e um
[guia de rede e implantação](deploy/README.md).

**Não copie** `.env`, banco de dados, volumes Docker, senhas ou tokens da
instalação original. O novo órgão precisa de chaves, administrador e número
WhatsApp próprios.

## Instalação própria

### 1. Configurar o servidor

Coloque o código em `/opt/zapgov`. Na raiz do projeto, crie `.env`:

```dotenv
DB_PASSWORD=<senha forte e exclusiva>
APP_ORIGIN=https://app.ORGAO.DOMINIO
API_URL=https://api.ORGAO.DOMINIO
```

Em `backend/.env`, configure:

```dotenv
JWT_SECRET=<resultado de openssl rand -hex 32>
META_CREDENTIALS_KEY=<outro resultado de openssl rand -hex 32>
SEED_ADMIN_PASSWORD=<senha provisória de 12 a 72 bytes>
APP_ENV=production
LOG_LEVEL=info
```

Gere `JWT_SECRET` e `META_CREDENTIALS_KEY` separadamente. A segunda chave cifra
as credenciais da Meta gravadas no banco; guarde-a com os backups. Sem ela, um
backup do banco não recupera essas credenciais. Restrinja o acesso aos arquivos
com `chmod 600 .env backend/.env`. Eles não devem ser enviados ao Git.

As variáveis `WHATSAPP_*` e `WEBHOOK_VERIFY_TOKEN` não são necessárias para
uma instalação nova: o administrador as configura no painel após o primeiro
login. O arquivo [backend/.env.example](backend/.env.example) mostra as demais
opções disponíveis.

### 2. Iniciar API, banco e fila

Na raiz do projeto:

```bash
docker compose -f docker-compose.prod.yml config --quiet
docker compose -f docker-compose.prod.yml up -d --build
docker compose -f docker-compose.prod.yml ps
```

As migrações rodam antes da API e do worker. Na primeira inicialização, o
sistema cria o usuário `admin@zapgov.local` com `SEED_ADMIN_PASSWORD` e exige
troca de senha no primeiro acesso.

### 3. Compilar e iniciar o painel

`NEXT_PUBLIC_API_URL` fica gravada no build do Next.js. Use a URL pública da
API do novo órgão:

```bash
cd /opt/zapgov/web
npm ci
NEXT_PUBLIC_API_URL=https://api.ORGAO.DOMINIO npm run build
```

Instale [deploy/zapgov-web.service](deploy/zapgov-web.service) em
`/etc/systemd/system/zapgov-web.service`. Confira no arquivo o usuário, o
diretório `/opt/zapgov/web` e o caminho de `npm`, depois execute:

```bash
sudo systemctl daemon-reload
sudo systemctl enable --now zapgov-web
sudo systemctl status zapgov-web
```

Configure DNS, certificados TLS e proxy para os dois endereços. No proxy nginx,
deixe `/events` sem buffer para o inbox em tempo real. As instruções completas
estão no [guia de implantação](deploy/README.md).

### 4. Cadastrar o órgão e a Meta

1. Abra `https://app.ORGAO.DOMINIO/login`, entre com `admin@zapgov.local` e a
   senha provisória, e faça a troca obrigatória.
2. Em **Configurações → Organização**, informe nome, UF e fuso horário.
3. Em **Configurações → WhatsApp / Meta**, salve Phone Number ID, WABA ID,
   token de acesso permanente, App Secret, versão da API e Webhook Verify Token.
4. No aplicativo da Meta, configure a URL de callback
   `https://api.ORGAO.DOMINIO/webhook`, use o mesmo Verify Token e assine os
   eventos de mensagens.
5. Configure setores, usuários, horários de atendimento, respostas automáticas
   e templates conforme a operação do órgão.

O backend cifra token, App Secret e Verify Token no banco. Campos secretos
deixados vazios em edições posteriores mantêm os valores salvos. O status
**Configurado** indica que há credenciais cadastradas; confirme a conexão
recebendo e respondendo a uma mensagem de teste.

### 5. Validar e operar

Na raiz do projeto:

```bash
curl -fsS https://api.ORGAO.DOMINIO/health
docker compose -f docker-compose.prod.yml ps
docker compose -f docker-compose.prod.yml logs --tail=100 app worker migrate
sudo systemctl is-active zapgov-web
```

Confirme que uma mensagem enviada **ao** número do órgão aparece no inbox e
que uma resposta pelo painel chega ao número de teste. Para iniciar uma
conversa fora da janela de atendimento, use um template aprovado.

Configure backup diário do PostgreSQL, cópia externa e teste de restauração.
Guarde `META_CREDENTIALS_KEY` junto ao plano de recuperação. Os procedimentos
de [backup, atualização e proxy](deploy/README.md) e o
[guia detalhado de entrega a outro órgão](deploy/NOVO_ORGAO.md) ficam em `deploy/`.

## Adicionar um órgão na mesma instalação

Se o órgão usar a infraestrutura atual, o administrador da plataforma entra
em `/superadmin`, cria um órgão e seu administrador inicial, e habilita os
módulos desejados. No login, o novo administrador seleciona seu **Painel**,
troca a senha provisória e cadastra as próprias credenciais em
**Configurações → WhatsApp / Meta**. O número Meta deve ser distinto do número
associado aos outros painéis.

Nesse modo, os órgãos compartilham servidor e banco, com separação de dados
por `tenant_id` na aplicação. O backup abrange a instalação inteira. O
[passo a passo completo](deploy/NOVO_ORGAO.md#opção-b-adicionar-um-painel-na-instalação-existente)
explica essa opção.

## Desenvolvimento

Para rodar localmente, consulte [deploy/DEV.md](deploy/DEV.md). Para detalhes
do backend e da entrega de mensagens, consulte [backend/README.md](backend/README.md).
Os materiais de trabalho em `docs/` ficam apenas no workspace local e são
ignorados pelo Git; os guias destinados ao repositório ficam em `deploy/`.
