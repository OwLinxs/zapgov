# Bot → SIGE-TI

O bloco **Chamar API** mantém as perguntas e variáveis no ZapGov e envia um POST para `/integracao/ordens-servico`. Nesta versão, o conector disponível é SIGE-TI; destino, método e autenticação seguem esse contrato. Não é um executor de URLs arbitrárias.

## Configurar no bloco

No editor, adicione **Chamar API** e preencha **URL base** (por exemplo `https://sige-ti.exemplo.gov.br`) e **Chave de API**. O caminho `/integracao/ordens-servico` é acrescentado automaticamente. Não é necessário informar tenant ID: o vínculo é o órgão dono do fluxo.

A chave é criptografada no banco e não é devolvida ao navegador ao carregar o fluxo. Após salvar, o campo fica vazio com a indicação “Chave configurada”. Deixe vazio para manter ou digite uma nova chave para substituir. Ao mudar a URL, informe a chave novamente. Excluir o bloco remove sua credencial do grafo atual.

O servidor usa a `META_CREDENTIALS_KEY` já utilizada para proteger credenciais (64 caracteres hexadecimais). Preserve essa chave e seu backup. As antigas variáveis `SIGE_TI_BASE_URL`, `SIGE_TI_API_KEY` e `SIGE_TI_TENANT_ID` não são mais usadas: configure cada bloco pelo editor. Não há migração de banco adicional.

A edição continua restrita a administradores. HTTP é aceito para rede interna confiável; prefira HTTPS. Redirecionamentos não são seguidos. Cada fluxo/órgão pode configurar seu destino e sua chave.

## Montar o fluxo

1. Pergunta sobre o resumo → variável `equipamento_descricao`.
2. Pergunta sobre o problema → variável `defeito_relatado`.
3. Opcionalmente, perguntar o nome → variável `nome`.
4. Adicionar **Chamar API**, com o corpo:

```json
{
  "equipamento_descricao": "{{equipamento_descricao}}",
  "defeito_relatado": "{{defeito_relatado}}",
  "equipamento_identificacao": "",
  "solicitante_nome": "{{_nome}}",
  "solicitante_contato": "{{_telefone}}",
  "prioridade": "normal"
}
```

`_nome` é o nome do perfil do WhatsApp; use `{{nome}}` para o nome perguntado. Se não houver nome de perfil, ele fica vazio. `_telefone` contém o telefone normalizado da conversa. Valores devem ser textos JSON. Variáveis ausentes causam erro de dados antes do envio; remova campos opcionais não coletados. Aspas e quebras de linha das respostas são escapadas corretamente. Prioridade aceita `baixa`, `normal` ou `alta`; menus de captura salvam o rótulo, portanto use esses valores nos rótulos ou configure uma prioridade fixa.

O servidor inclui `origem=whatsapp` e `referencia_externa=wa-<UUID>` automaticamente. A referência pertence à execução do fluxo e é persistida antes da chamada. Repetições no mesmo fluxo reutilizam a referência. Um novo atendimento após encerrar/limpar o fluxo gera outra referência. Não há repetição automática: para oferecer uma nova tentativa, conecte o caminho de erro a um menu que retorne ao mesmo bloco. Após timeout, confira ou repita no mesmo fluxo; não inicie outro atendimento para tentar confirmar a mesma criação.

5. Manter o mapeamento de retorno `numero → numero`, `id → id`, `status → status` (nomes de variáveis editáveis).
6. Ligar **Sucesso** a uma mensagem:

```text
✅ Chamado aberto: {{numero}}. A equipe de T.I. já vai te atender por aqui.
```

7. Ligar **Erro** a uma mensagem/condição/menu ou ao bloco Atendente. Sem saída de erro conectada, o bot encaminha ao atendimento humano com uma mensagem de falha de confirmação.
8. Após a mensagem de sucesso, conectar ao próximo bloco ou Fim e salvar o fluxo.

HTTP 200 e 201 são sucesso somente com `id` positivo, `numero` e `status` preenchidos no JSON. Os dados coletados e mapeados são preservados em `conversations.dados`, inclusive se o próximo bloco for Fim. Isso não cria um módulo interno de chamados.

## Erros disponíveis no fluxo

- `api_http_status`: código HTTP; `0` quando não recebeu resposta.
- `api_erro`: `chave_invalida` (401), `dados_invalidos` (422 ou validação local), `integracao_indisponivel` (503), `resultado_incerto` (falha de transporte/timeout), `resposta_invalida`, `erro_http`, `integracao_nao_configurada` ou `configuracao_invalida`.

A chamada tem limite de 15 segundos e resposta de 1 MiB. O corpo bruto de erro não é enviado ao cidadão nem salvo nas variáveis; `campos` do 422 não é mapeado nesta versão. Variáveis de retorno anteriores são limpas antes da tentativa, evitando anunciar protocolo antigo como sucesso.

## Limites

Não recebe atualizações posteriores de status/técnico: o contrato fornecido só cria/atualiza a OS e retorna os dados dessa chamada. A resposta fornecida também não traz nome de técnico. Para exibir alterações posteriores, será necessário um endpoint/evento separado acordado com o SIGE-TI.

Validação local com servidor HTTP simulado; a configuração real e o teste ponta a ponta com WhatsApp e SIGE-TI ainda precisam ser feitos no ambiente de implantação.
