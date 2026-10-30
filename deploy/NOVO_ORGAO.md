# Implantar o ZapGov para outro órgão do Município

Este guia é para a equipe de TI que receberá o código. Ele cobre uma **instalação
própria**, com banco, Redis, domínio e número WhatsApp do novo órgão. Se o órgão
for usar a instalação ZapGov já existente, siga a [opção de painel adicional](#opção-b-adicionar-um-painel-na-instalação-existente) no fim.

O uso por órgãos e secretarias do Município de Francisco Beltrão está previsto
na [licença](../LICENSE.md). Não inclua no pacote entregue arquivos `.env`, dumps
de banco, volumes Docker, tokens ou senhas da instalação atual.

## 1. Dados que a equipe precisa reunir

| Item | Exemplo ou finalidade |
| --- | --- |
| Domínio do painel | `https://app.ORGAO.DOMINIO` |
| Domínio da API | `https://api.ORGAO.DOMINIO` |
| Servidor interno | Linux com Docker, Compose plugin e Node.js 20 |
| Proxy público | HTTPS nos dois domínios; encaminha para as portas internas 3000 e 8080 |
| Meta Cloud API | Phone Number ID, WABA ID, token permanente de System User e App Secret próprios |
| Administrador inicial | Responsável pelo primeiro acesso e pela configuração do painel |

O número WhatsApp do novo órgão deve ter suas próprias credenciais Meta. A Meta
precisa conseguir acessar `https://api.ORGAO.DOMINIO/webhook`; o servidor precisa
alcançar `graph.facebook.com:443` e os domínios usados para mídia. Restrinja
as portas 3000 e 8080 à rede do proxy. Veja o desenho de rede em
[deploy/README.md](README.md).

## 2. Instalar o código e preparar os segredos

Coloque esta versão do repositório em `/opt/zapgov` no servidor interno. O usuário
que executa o painel deve ter leitura do código e permissão de escrita em
`/opt/zapgov/web/.next`. O serviço fornecido usa o usuário `zapgov`; crie-o ou
ajuste o campo `User=` de [zapgov-web.service](zapgov-web.service).

Na **raiz** do projeto, crie `.env` com valores novos para esta instalação:

```dotenv
DB_PASSWORD=<senha forte exclusiva desta instalação>
APP_ORIGIN=https://app.ORGAO.DOMINIO
API_URL=https://api.ORGAO.DOMINIO
```

Em `backend/.env`, configure pelo menos:

```dotenv
JWT_SECRET=<resultado de openssl rand -hex 32>
META_CREDENTIALS_KEY=<outro resultado de openssl rand -hex 32>
SEED_ADMIN_PASSWORD=<senha provisória exclusiva, de 12 a 72 bytes>
APP_ENV=production
LOG_LEVEL=info
```

Gere `JWT_SECRET` e `META_CREDENTIALS_KEY` **separadamente** com
`openssl rand -hex 32`. Não reutilize as chaves da instalação original.
`META_CREDENTIALS_KEY` cifra as credenciais Meta no banco: guarde-a com o plano
de recuperação, pois o dump do banco sozinho não permite recuperá-las. Não
publique esses arquivos no Git; ambos são ignorados pelo repositório.
Restrinja a leitura deles aos administradores da instalação (`chmod 600 .env
backend/.env`, a partir da raiz do projeto).

As variáveis `WHATSAPP_*` e `WEBHOOK_VERIFY_TOKEN` podem ficar ausentes: o
administrador cadastra as informações da Meta em **Configurações → WhatsApp /
Meta** depois do primeiro acesso. Para este fluxo, deixe também as variáveis
`GATEWAY_*` e `SIGE_*` sem valor, a menos que haja uma integração SIGE planejada.

## 3. Subir API, banco, fila e painel

Na raiz do projeto:

```bash
docker compose -f docker-compose.prod.yml config --quiet
docker compose -f docker-compose.prod.yml up -d --build
docker compose -f docker-compose.prod.yml ps
docker compose -f docker-compose.prod.yml logs --tail=100 app worker migrate
```

O Compose executa as migrações antes de iniciar a API e o worker. Na primeira
subida, a API cria `admin@zapgov.local` com `SEED_ADMIN_PASSWORD`; esse usuário
deve trocar a senha no primeiro login. A senha provisória não é recriada quando
já existem usuários no banco.

Compile o painel com a **URL pública** da API. Ela fica gravada no build, então
trocar `API_URL` depois exige um novo build:

```bash
cd /opt/zapgov/web
npm ci
NEXT_PUBLIC_API_URL=https://api.ORGAO.DOMINIO npm run build
```

Execute esses comandos com o usuário que manterá os arquivos do painel.
Instale [zapgov-web.service](zapgov-web.service) em `/etc/systemd/system/`, confira
`WorkingDirectory`, `User` e o caminho de `npm` em `ExecStart`, e então:

```bash
sudo cp /opt/zapgov/deploy/zapgov-web.service /etc/systemd/system/zapgov-web.service
sudoedit /etc/systemd/system/zapgov-web.service
sudo systemctl daemon-reload
sudo systemctl enable --now zapgov-web
sudo systemctl status zapgov-web
```

No **servidor de proxy**, configure DNS, TLS e encaminhamento. O arquivo
[nginx-dmz.conf](nginx-dmz.conf) é uma referência: copie-o para a configuração
do nginx desse servidor, substitua `INTERNAL_IP`, os dois domínios e os caminhos
do certificado, valide com `sudo nginx -t` e recarregue o nginx. A rota
`/events` precisa permanecer sem buffer para o atendimento em tempo real.

## 4. Configurar o órgão e a Meta no painel

1. Abra `https://app.ORGAO.DOMINIO/login` e entre com
   `admin@zapgov.local` e a senha provisória. Faça a troca obrigatória.
2. Em **Configurações → Organização**, ajuste nome, UF e fuso horário.
3. Em **Configurações → WhatsApp / Meta**, preencha Phone Number ID, WABA ID,
   token de acesso, App Secret, versão da API e Webhook Verify Token. Escolha
   um Verify Token próprio e guarde-o para a próxima etapa.
4. Salve. O indicador **Configurado** confirma que há credenciais salvas; ele
   não faz uma chamada de teste à Meta. Em edições futuras, campos secretos
   vazios preservam os valores já cadastrados.
5. Na configuração do webhook do aplicativo Meta, informe a URL
   `https://api.ORGAO.DOMINIO/webhook`, use o mesmo Verify Token e assine os
   eventos de mensagens. O backend usa o App Secret para validar a assinatura
   dos eventos recebidos.
6. Configure departamentos, usuários, horários, mensagens automáticas,
   chatbot e permissões de templates conforme a operação do órgão.

O token, o App Secret e o Verify Token são cifrados no banco. A API e o worker
passam a usar o cadastro do painel sem reinício.

## 5. Conferir antes de liberar o atendimento

Volte à raiz do projeto no servidor:

```bash
cd /opt/zapgov
curl -fsS https://api.ORGAO.DOMINIO/health
docker compose -f docker-compose.prod.yml ps
sudo systemctl is-active zapgov-web
```

Depois, confira no painel e em um número de teste:

1. Login e troca de senha; acesso ao painel do órgão correto.
2. Verificação do webhook na Meta. Se falhar, confira URL pública, TLS, Verify
   Token e logs da API.
3. Mensagem enviada **para** o WhatsApp do órgão: deve aparecer no inbox.
4. Resposta pelo inbox dentro da janela de atendimento: deve chegar ao cidadão.
5. Sincronização de templates, se forem usados. Para iniciar conversa fora da
   janela de atendimento, use um template aprovado.
6. Logs da API e do worker sem erros de credenciais ou fila.

Não use dados reais de cidadãos no teste inicial. Para diagnóstico:

```bash
docker compose -f docker-compose.prod.yml logs --tail=100 app worker
journalctl -u zapgov-web -n 100 --no-pager
```

## 6. Operar e recuperar

Configure backup diário do PostgreSQL, retenção e cópia em outro disco ou host.
Teste a restauração em ambiente separado. Guarde junto a chave
`META_CREDENTIALS_KEY` em local seguro; sem ela, as credenciais Meta cifradas no
backup não abrem. Veja os comandos de backup e restauração em
[deploy/README.md](README.md#operação).

Para atualizar, siga [UPGRADE.md](UPGRADE.md) e use o
[script de atualização](update.sh) a partir da raiz do projeto. Ele faz um
backup antes de migrar e verifica API e painel depois do deploy. Não ative o
timer de atualização automática sem definir o processo de aprovação de
mudanças do órgão.

## Opção B: adicionar um painel na instalação existente

Se o órgão **não** precisa de servidor, banco e domínio próprios, use o
multi-tenant já presente na instalação da prefeitura:

1. Entre com o administrador da plataforma (admin do tenant 1) e acesse
   `/superadmin` no painel atual.
2. Crie o órgão com nome, identificador de painel, e-mail e senha provisória
   do administrador. O identificador aparece no seletor **Painel** da tela de
   login quando houver mais de um órgão.
3. Habilite para esse órgão os módulos desejados em **Módulos**. Novos órgãos
   começam sem módulos opcionais ligados.
4. O novo administrador entra selecionando seu painel, troca a senha e segue
   a [configuração do órgão e da Meta](#4-configurar-o-órgão-e-a-meta-no-painel).

Nesse modelo, todos usam a mesma infraestrutura, mas conversas, usuários e
configurações são separados por `tenant_id` na aplicação. O número da Meta do
novo órgão deve ser diferente do número já associado a outro painel. O backup
é da instalação inteira, não de um órgão isolado.
