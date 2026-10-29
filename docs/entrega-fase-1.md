# Requisitos e evidências da fase 1

Referência: enunciado `Fase 1 - Tech Challenge.pdf` e orientações complementares do professor. O escopo é uma API Go com Swagger, executada localmente por HTTP em loopback; não inclui frontend próprio nem implantação pública.

Identificação: **Grupo 1 — Lucas Velôso — RM 377934**, curso SOAT, fase 1. Discord: **Não utiliza Discord**.

| Requisito | Página | Evidência | Situação |
|---|---|---|---|
| Clientes e veículos, identificação por documento e placa | 3, 9–11 | API e testes de validação | Implementados e testados |
| Catálogo de serviços, peças e insumos | 3, 9–10 | CRUD e testes | Implementados e testados |
| OS, orçamento automático e aprovação/rejeição | 3, 9 | Fluxo HTTP e testes | Revisão, rejeição e aprovação verificadas |
| Seis status e regras de transição | 3, 9–10 | Domínio, API e testes | Fluxo até entrega verificado |
| Consulta da OS pelo cliente | 3, 10 | API e isolamento de acesso | Isolamento e rotação de token verificados |
| Estoque e transações | 3, 10–11 | Movimentos, concorrência e rollback | Verificados com PostgreSQL real |
| Tempo médio de execução | 4, 10 | Métricas por serviço e por OS | Intervalos com e sem amostras testados |
| JWT administrativo | 4, 11 | Autenticação e autorização | Implementado e testado |
| Monólito em camadas e justificativa do banco | 4, 10 | Código, arquitetura e decisões | JSONB, restrições relacionais e transações documentados |
| REST e OpenAPI/Swagger | 4–6, 10 | Contrato e documentação local | 40 operações servidas pelo backend |
| Dockerfile e Compose | 4 | Build e execução reproduzíveis | Docker local e CI aprovados; seed, saúde e persistência verificados |
| Testes e cobertura mínima | 4–5, 11 | Relatórios reproduzíveis | Linux/CI: 85,8% global e 99,0% no domínio |
| Análise de vulnerabilidades | 5, 7 | Relatório com interpretação dos achados | Relatório disponível; ressalva abaixo |
| README completo | 5–6 | Execução, testes, autenticação, arquitetura e links | Documentado, com links para o pacote de entrega |
| Event Storming, C4 até o nível 3 e linguagem ubíqua | 8–9 e orientação complementar | Diagramas, glossário e fontes | PDFs, PNGs, Markdown e fonte Excalidraw disponíveis |
| Repositório público; main protegida; PRs | 5–6 | GitHub e checks obrigatórios | Repositório público, proteção configurada e histórico no PR #1 |
| Branch e hash da entrega | 6 | Identificação da versão no PDF | Tag fase-1-v1; branch e commit completo no PDF |
| Vídeo de até 15 minutos | 6–7 e orientação complementar | Demonstração acessível e identificação | Vídeo final de 11min06,85s verificado; arquivo na release |
| PDF único e identificação do grupo | 4–5 e orientação complementar | Grupo, nomes, RMs, tecnologias e links | Arquivo entrega-fase-1-grupo-1.pdf na release; Discord informado como não utilizado |

## DDD e arquitetura

Conforme orientação do professor, a documentação reúne Event Storming, linguagem ubíqua e C4 nos níveis 1, 2 e 3. O nível 3 detalha um cenário: início do primeiro serviço com baixa de estoque. O modelo é uma construção conceitual do projeto, sem alegação de workshop com especialistas.

- [C4 em PDF](ddd/exports/c4.pdf) e [Event Storming em PDF](ddd/exports/event-storming.pdf).
- [Linguagem ubíqua e modelo de domínio](ddd/dominio.md).
- [Fonte editável no Excalidraw](ddd/exports/oficina-ddd.excalidraw) e [imagens e fontes textuais](ddd/c4.md).
- Quadro compartilhado do Excalidraw: [abrir no Excalidraw](https://excalidraw.com/#json=y-XhNVpZNgjkyZGcGTIug,NSCIC96zvDJ97e1Kz-Tjig).

## Verificação técnica

Os [testes de integração](evidencias/integracao.txt) cobrem autenticação, cadastros, fluxo completo de OS, snapshots, revisão de orçamento, isolamento e rotação de tokens, estoque insuficiente, concorrência e rollback. A [validação Docker](evidencias/docker.txt) executou build, seed, saúde, Swagger/OpenAPI e um fluxo de 32 requisições com 47 verificações aprovadas. Após recriar os contêineres sem excluir o volume, as oito verificações de persistência também passaram.

Os jobs `backend` e `compose-smoke` passaram na [execução 40883124842 do GitHub Actions](https://github.com/oveloso/tech_-challenge_pos_fiap/actions/runs/40883124842). A execução Linux inclui testes com detector de corridas. A cobertura global foi **85,8%**, incluindo os pontos de entrada e persistência com `-coverpkg=./...`; o domínio alcançou **99,0%**. A medição histórica no Windows, em 28/10/2026, permanece registrada em **85,6%** global. São execuções distintas, detalhadas no [registro de verificação](evidencias/verificacao.md).

O [govulncheck](evidencias/govulncheck.txt) registrou zero vulnerabilidades em código alcançável e zero em pacotes importados, com um achado em módulo requerido sem chamada identificada. Isso não equivale a ausência total de achados em dependências. Conforme orientação do professor, o relatório não precisa estar sem achados; deve apresentar o resultado e sua interpretação.

Swagger precisa de internet para carregar CSS e JavaScript da CDN. A especificação e as operações da API são locais. HTTPS não integra esta demonstração em loopback; uma futura exposição pública exigiria configuração de TLS.

## Repositório e publicação

O [repositório é público](https://github.com/oveloso/tech_-challenge_pos_fiap). A `main` exige PR e os checks `backend` e `compose-smoke`, aplica as regras aos administradores e bloqueia force push e exclusão. O grupo possui um integrante, por isso a quantidade de aprovações adicionais exigidas é zero. O [PR #1](https://github.com/oveloso/tech_-challenge_pos_fiap/pull/1) registra a implementação, as alterações e os checks de cada revisão.

A [release fase-1-v1](https://github.com/oveloso/tech_-challenge_pos_fiap/releases/tag/fase-1-v1) reúne os arquivos de apresentação:

- `entrega-fase-1-grupo-1.pdf`: identificação, tecnologias, branch/hash, declaração de apoio de IA e links de avaliação.
- `tech-challenge-fase-1-grupo-1.mp4`: vídeo final de **11min06,85s**, com 27 capítulos.
- `tech-challenge-fase-1-grupo-1.srt`: legendas em português, com 201 entradas.

O [vídeo final](../entrega/video/README.md) mostra operação real do Swagger e acompanha a OS nº 5 até a entrega, com orçamento de R$ 338 e saldos de peças 8→6 e insumo 5→4. A edição reduz esperas e oculta credenciais; a narração é sintética em português. O desenvolvimento teve apoio de IA da OpenAI/Codex (GPT-6), sob responsabilidade acadêmica do integrante.

O envio do PDF na plataforma acadêmica é uma etapa separada da publicação dos materiais. O repositório e os arquivos devem continuar acessíveis durante a correção e a revisão de notas.
