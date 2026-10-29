# Sistema de oficina da fase 1

Status: escopo da API implementado; validação e pendências de entrega em [Requisitos e evidências](entrega-fase-1.md).

## Objetivo e escopo

Desenvolver uma API acadêmica FIAP SOAT para gerenciar uma oficina mecânica. A entrega técnica é o backend com documentação interativa Swagger. Funcionários e clientes realizam suas operações por HTTP. O código deve ser legível, ter comentários apenas quando explicarem decisões e usar uma arquitetura proporcional ao MVP.

A stack adotada é Go 1.26.0 ou superior, Gin 1.12.0, PostgreSQL 18, `pgx`, OpenAPI e Docker Compose. Os detalhes de negócio abaixo são decisões do projeto para tornar o escopo implementável; não são regras explicitamente definidas pela FIAP.

Fonte acadêmica: `Fase 1 - Tech Challenge.pdf`, 12 páginas, fornecido pelo usuário. A aplicação deve atender às funcionalidades e entregáveis das páginas 3 a 11.

## Pessoas e permissões

- Funcionário: autentica com usuário e senha; gerencia cadastros, estoque, diagnóstico, orçamento e execução das OS. Um único perfil administrativo basta para o MVP.
- Cliente: recebe um token de acesso a uma OS; consulta o andamento e aprova ou rejeita o orçamento vigente pela API, enviando `Authorization: Bearer <token>`. Não acessa cadastros administrativos nem OS de outras pessoas.

Não há cadastro público de funcionários. O primeiro usuário administrativo é criado por comando de seed com credenciais fornecidas por variáveis de ambiente; senhas reais não são versionadas.

## Arquitetura e stack

Um backend monolítico Go com Gin e um banco PostgreSQL. O próprio backend serve Swagger em `GET /docs` e a especificação estática em `GET /openapi.yaml`. Docker Compose configura API e banco para execução local.

O backend separa responsabilidades de HTTP, aplicação, domínio e persistência nos pacotes `auth`, `registry`, `workorders`, `domain`, `platform` e `server`, com migrações em `backend/migrations`. `registry` reúne clientes, veículos, catálogo e estoque; `workorders` coordena OS, orçamentos e indicadores. Esses módulos não representam automaticamente contextos delimitados de DDD; os limites estão explicados no modelo de domínio.

Os handlers validam a entrada e usam funções de serviço e persistência. `registry` e `workorders` coordenam as operações, com SQL explícito e a interface `platform.DB` para conexão ou transação. `domain` concentra as regras de orçamento, transição de estado e validação, independentes de Gin e PostgreSQL.

Persistência: SQL explícito com `pgx`, migração SQL versionada executada pelo próprio backend e transações nos casos de uso que alteram múltiplos registros. Cadastros e o agregado de OS são armazenados em JSONB. Colunas geradas permitem chaves estrangeiras, unicidade, índices e restrições de estado; movimentos de estoque são registros separados. Essa combinação reduz o mapeamento do agregado e preserva snapshots, com o custo de regravar o documento e consultar estruturas JSON para relatórios. Regras internas de itens e orçamentos também dependem das validações do domínio; o esquema não é totalmente normalizado.

Swagger UI usa os assets de `swagger-ui-dist` 5.33.1 via CDN. A conexão à internet é necessária para carregar esses assets; a API e o contrato são servidos localmente. Todas as regras de negócio são verificadas pelo backend.

Não fazem parte desta fase: pagamentos, emissão fiscal, múltiplas oficinas, integrações reais de WhatsApp/email, aplicativo móvel, permissões administrativas complexas ou serviços distribuídos. A disponibilização de orçamento via API atende à alternativa expressa na página 9 do enunciado.

## Modelo de dados e regras comuns

- Cliente: UUID, nome, tipo de documento, CPF/CNPJ normalizado e único, email e telefone opcionais, ativo.
- Veículo: UUID, cliente, placa normalizada e única, marca, modelo, ano, ativo.
- Serviço do catálogo: UUID, nome, descrição opcional, preço em centavos de BRL, ativo.
- Item de estoque: UUID, SKU único, nome, tipo `PART` ou `SUPPLY`, preço em centavos de BRL, saldo inteiro não negativo, ativo. O MVP trabalha com unidades inteiras; frações de litros ou quilos ficam fora do escopo.
- OS: UUID, número sequencial legível, cliente e veículo, descrição do problema, status, versão para concorrência, datas, serviços e peças solicitados.
- Item de serviço da OS: UUID estável, serviço do catálogo, estado de execução e horários. Cada serviço aparece no máximo uma vez por OS e corresponde a uma execução; não existe quantidade de serviços.
- Item de peça da OS: item de estoque e quantidade inteira positiva, sem duplicatas; preço congelado no orçamento.
- Orçamento: UUID, OS, versão sequencial por OS, estado, itens com nomes e valores congelados, total, datas e decisão do cliente. Histórico é preservado.
- Movimento de estoque: UUID, item, quantidade com sinal, motivo, data e OS quando aplicável.

Valores monetários são inteiros em centavos; o total é calculado pelo servidor. Preços e totais não ultrapassam `9007199254740991` centavos, preservando interoperabilidade com consumidores JSON; somas e multiplicações detectam overflow antes de produzir resultados. Nenhum endpoint aceita um total calculado pelo cliente. Datas são armazenadas e transmitidas em UTC com formato RFC 3339.

CPF/CNPJ e placa têm validação de formato e, quando aplicável, dígitos verificadores. Documentos são strings. CPF usa 11 dígitos; CNPJ aceita o formato numérico legado e o alfanumérico vigente, com 12 posições alfanuméricas e dois dígitos verificadores numéricos. Normalizar pontuação e espaços e converter letras para maiúsculas, sem descartar letras do CNPJ. A Receita Federal confirma a coexistência dos formatos em sua [página do CNPJ alfanumérico](https://www.gov.br/receitafederal/pt-br/acesso-a-informacao/acoes-e-programas/programas-e-atividades/cnpj-alfanumerico); o [material técnico do DV](https://www.gov.br/receitafederal/pt-br/centrais-de-conteudo/publicacoes/documentos-tecnicos/cnpj) orienta a implementação. Placas aceitam os padrões brasileiros `LLLNNNN` e `LLLNLNN`, normalizados sem hífen; o segundo está descrito na [orientação do Detran RS](https://www.detran.rs.gov.br/veiculos/servicos/944).

Os cadastros administrativos oferecem criação, leitura, atualização e exclusão lógica. `DELETE` desativa o registro, preservando histórico. Registros inativos não podem ser incluídos em novas OS; OS já criadas continuam legíveis. Documento, placa e SKU continuam únicos após desativação. A edição permite reativação por `active: true`.

Na abertura, a OS mantém referências ao cliente e veículo e congela o nome do cliente e a identificação do veículo (placa, marca, modelo e ano) para leitura histórica. Atualizações posteriores nos cadastros não reescrevem esses snapshots. Transferir um veículo para outro cliente só é permitido quando todas as OS desse veículo estiverem `DELIVERED`; caso contrário, a atualização retorna 409. OS anteriores continuam associadas ao cliente original. Os tokens existentes continuam restritos às respectivas OS e não concedem acesso ao novo proprietário.

## Fluxo da ordem de serviço

Status exigidos, com valores da API:

| Status exibido | Valor da API |
|---|---|
| Recebida | `RECEIVED` |
| Em diagnóstico | `IN_DIAGNOSIS` |
| Aguardando aprovação | `AWAITING_APPROVAL` |
| Em execução | `IN_EXECUTION` |
| Finalizada | `COMPLETED` |
| Entregue | `DELIVERED` |

1. O funcionário localiza/cadastra cliente e veículo e cria a OS em `RECEIVED`. O veículo deve pertencer ao cliente informado. Serviços e peças podem ser informados na abertura ou adicionados no diagnóstico.
2. Inicia o diagnóstico, movendo a OS para `IN_DIAGNOSIS`.
3. Define os serviços e peças. A substituição da lista de itens só é permitida em `RECEIVED` ou `IN_DIAGNOSIS`. A criação de orçamento exige pelo menos um serviço e preços válidos.
4. Publica o orçamento. O servidor congela nomes, preços e quantidades, calcula o total, cria uma versão `PENDING` e move a OS para `AWAITING_APPROVAL`.
5. O cliente aprova ou rejeita a versão vigente. Aprovar muda o orçamento para `APPROVED`; a OS continua em `AWAITING_APPROVAL` até o início do primeiro serviço. A combinação dos dois estados indica orçamento aprovado, aguardando início. Rejeitar muda o orçamento para `REJECTED` e retorna a OS para `IN_DIAGNOSIS`.
6. O funcionário inicia o primeiro serviço. Na mesma transação, verifica aprovação vigente, baixa todas as peças da OS, registra os movimentos, marca o serviço `IN_PROGRESS` e a OS `IN_EXECUTION`. Falta de estoque impede todas essas alterações.
7. O funcionário inicia e conclui os serviços individualmente. Vários serviços podem estar em andamento. Um serviço só pode ser concluído se estiver `IN_PROGRESS`; concluído recebe `COMPLETED` e horário de término.
8. Quando todos os serviços estiverem concluídos, o funcionário finaliza a OS, que passa para `COMPLETED`. A entrega é uma ação separada que muda para `DELIVERED`.

As mudanças de status acontecem como consequência dessas ações. Não existe um endpoint de alteração livre do status.

### Revisão e concorrência do orçamento

Antes do início de qualquer serviço, o funcionário pode reabrir o diagnóstico de uma OS em `AWAITING_APPROVAL`. O orçamento pendente ou aprovado passa a `SUPERSEDED` e a OS volta para `IN_DIAGNOSIS`. O próximo orçamento tem nova versão e exige nova aprovação.

Uma decisão do cliente precisa informar o UUID do orçamento. Só o orçamento vigente e `PENDING` aceita uma decisão. Repetir a mesma decisão sobre a mesma versão é idempotente; tentar uma decisão diferente ou atuar numa versão superada retorna conflito. A repetição só é idempotente se o orçamento continuar sendo o vigente; uma versão antiga nunca altera o estado atual.

No MVP, serviços ou peças adicionais só podem ser incluídos antes de iniciar a execução, mediante revisão do diagnóstico e nova aprovação. Reparos adicionais descobertos durante a execução e cancelamento de OS ficam fora desta primeira versão; a rejeição do orçamento continua suportada.

### Estoque e integridade

Não há reserva de estoque na aprovação. A disponibilidade é verificada ao iniciar o serviço; saldo insuficiente retorna conflito na API. A baixa ocorre uma única vez, no início do primeiro serviço, para todos os itens aprovados.

Reposições e ajustes são movimentos explícitos com motivo obrigatório. O saldo nunca pode ficar negativo. A quantidade inicial é registrada como movimento de abertura. O saldo não é editável diretamente por `PATCH` do item.

Operações concorrentes devem usar bloqueio transacional da OS e dos itens de estoque, com ordem estável de bloqueio dos itens. Uma transação que falha não deixa aprovação, execução ou saldo parcialmente alterados. Repetir início de serviço já iniciado não causa nova baixa; o retorno é conflito sem efeitos adicionais.

## Acesso do cliente e segurança

APIs administrativas usam JWT com validade de uma hora. Senhas são armazenadas com bcrypt sobre o pré-hash SHA-256 codificado em Base64. Segredos e conexão com banco vêm de variáveis de ambiente.

Cada OS pode ter um token opaco de acesso do cliente, aleatório e com 256 bits de entropia. O servidor armazena apenas o hash SHA-256 do token. O funcionário pode emitir ou substituir o token; apenas o mais recente é válido. Prazo padrão de 30 dias, configurável.

O token é retornado somente no ato da emissão e enviado pelo consumidor no cabeçalho `Authorization: Bearer`. O funcionário entrega essa credencial ao cliente por um canal externo ao sistema. A consulta e a decisão do orçamento usam as rotas `/api/v1/customer`, sem token na URL. Swagger mantém a autorização somente na memória da página. Não há refresh token nesta fase.

O cliente recebe só os dados necessários à sua OS: identificação resumida do veículo, serviços, peças, valores, status e histórico de andamento. CPF/CNPJ, dados de outros clientes e dados administrativos não são retornados. Consultar por CPF ou placa não concede acesso à OS.

O login limita tentativas a 10 por minuto por IP e as rotas de cliente a 60 por minuto por IP, em memória do processo. Há validação de tamanho e tipo de entrada, consultas SQL parametrizadas e CORS com origens explícitas. Senhas, tokens e documentos pessoais não devem aparecer nos logs. Erros não expõem stack traces nem detalhes de SQL. O limitador local é proporcional à implantação de uma única instância do MVP.

## API e contrato

Prefixo `/api/v1`. Rotas administrativas em `/admin`, rotas do cliente em `/customer`, autenticação em `/auth`. O contrato de 40 operações fica em `docs/api/openapi.yaml`. Swagger UI em `/docs` exibe a mesma especificação versionada, servida em `/openapi.yaml`.

Respostas de erro seguem `application/problem+json`, com `type`, `title`, `status`, `detail`, `code` e erros de campos opcionais. Usar 400 para entrada inválida, 401 para credencial inválida/expirada, 404 para recurso inexistente ou OS fora do acesso concedido, 409 para conflito de estado, documento duplicado, saldo insuficiente ou decisão incompatível, e 429 para excesso de tentativas.

Listagens usam `page` (a partir de 1), `pageSize` (padrão 20, máximo 100) e metadados `total`, `page`, `pageSize`. Filtros essenciais: documento do cliente, placa/cliente do veículo, ativos, SKU/nome de item, número/status/cliente da OS. A consulta por documento é exata e somente administrativa.

Criação de OS recebe `customerId`, `vehicleId`, `problemDescription`, `serviceIds` e `parts` com `inventoryItemId` e `quantity`. A substituição de itens recebe as listas completas. Versões de concorrência são retornadas; alterações de OS e ações administrativas recebem `expectedVersion`. Uma versão desatualizada retorna 409 antes de efeitos. Decisões do cliente usam o ID do orçamento como controle de versão.

## Uso e demonstração da API

Swagger permite executar autenticação, cadastros, movimentações, ações da OS e relatórios. A credencial administrativa e o token do cliente têm esquemas de autorização distintos. O acompanhamento é uma consulta explícita à API, que retorna status, histórico e orçamento vigente. Decisões de aprovação ou rejeição informam o UUID do orçamento; erros e conflitos seguem o contrato HTTP.

## Tempo de execução

Cada item de serviço registra `startedAt` e `completedAt`. O tempo de execução do serviço é a diferença entre esses horários. O relatório agrupa pelo serviço do catálogo e retorna quantidade concluída e média em segundos; inclui apenas serviços concluídos no intervalo solicitado.

O relatório também oferece a média por OS finalizada: `completedAt - executionStartedAt`, incluindo somente OS com finalização no intervalo. Tempo de diagnóstico e espera por aprovação não entram nesse indicador. OS entregues continuam contando pela data da finalização. Sem amostras, a média é `null` e a contagem é zero.

## Qualidade e critérios de aceite

- Fluxo demonstrável: cadastrar cliente/veículo, abrir OS, diagnosticar, publicar orçamento, acessar como cliente, rejeitar e revisar, aprovar, iniciar/concluir serviços, finalizar e entregar.
- Autenticação administrativa e isolamento do acesso de cada cliente testados.
- Testes unitários para cálculo, congelamento de preços, transições, decisões e validações.
- Testes de integração com PostgreSQL real para persistência, rollback, estoque concorrente e principais endpoints.
- Cobertura mínima de 80% nos domínios críticos, com relatório global também publicado para evitar ambiguidade com a página 5 do enunciado. Definir como críticos ordens de serviço/orçamento, estoque e validações de domínio.
- Fluxo HTTP completo com API e PostgreSQL real, incluindo aprovação, rejeição, isolamento de acesso e ações condicionadas pelo estado.
- Formatação, lint/análise estática e análise de vulnerabilidades automatizadas, com evidências salvas para a entrega. Ferramentas concretas ficam com a implementação.
- Contrato OpenAPI válido e comportamento verificado contra ele nos fluxos principais.
- Subida local documentada por Docker Compose, migrações reproduzíveis e dados sintéticos de demonstração.

## Documentação e entrega acadêmica

Manter documentação junto do código: README de execução, linguagem ubíqua, Event Storming, modelo de domínio, diagramas de arquitetura e decisões arquiteturais curtas. C4 e ADRs ajudam a demonstrar o conteúdo estudado; não substituem as evidências específicas de DDD.

O enunciado exige repositório público no GitHub durante a avaliação, `main` protegida com alterações via PR, branch e hash da entrega, documentação DDD pública com exportação em PDF ou imagens no repositório, resultado de análise de vulnerabilidades, vídeo público no YouTube de até 15 minutos e PDF único com identificação do grupo e links. Publicações externas, configuração do repositório remoto, dados dos integrantes e gravação serão tratados quando a versão estiver pronta.

## Organização das etapas

1. Base Go/PostgreSQL, migrações, autenticação, contrato e execução local.
2. Cadastros e movimentação de estoque.
3. OS, orçamento, acesso do cliente e testes das regras.
4. Execução dos serviços, transações de estoque e indicadores.
5. Swagger, verificação integrada, documentação e evidências.
6. Validação da execução em contêineres e preparação da entrega acadêmica.

O fluxo de uma OS completa é verificado por testes HTTP. O estado das etapas e os limites das evidências ficam em [Requisitos e evidências](entrega-fase-1.md).
