# Requisitos e evidências da fase 1

Referência: enunciado `Fase 1 - Tech Challenge.pdf` e orientações complementares do professor. A entrega técnica abrange a API Go e Swagger. Esta lista distingue implementação, verificações locais e pendências acadêmicas; resultados devem corresponder à versão efetivamente entregue.

Identificação: **Grupo 1 - Lucas Velôso - RM 377934**, curso SOAT, fase 1.

| Requisito | Página | Evidência esperada | Situação |
|---|---|---|---|
| Clientes e veículos, identificação por documento e placa | 3, 9–11 | API e testes de validação | Implementados; testes de integração aprovados |
| Catálogo de serviços, peças e insumos | 3, 9–10 | CRUD e testes | Implementados; testes de integração aprovados |
| OS, orçamento automático e aprovação/rejeição | 3, 9 | Fluxo HTTP e testes | Implementados; revisão, rejeição e aprovação verificadas |
| Seis status e regras de transição | 3, 9–10 | Domínio, API e testes | Implementados; fluxo até entrega verificado |
| Consulta da OS pelo cliente | 3, 10 | API e teste de isolamento de acesso | Implementada; isolamento e rotação de token verificados |
| Controle de estoque e transações | 3, 10–11 | Movimentos e testes de concorrência/rollback | Implementados; concorrência e rollback verificados com PostgreSQL 18.6 |
| Tempo médio de execução | 4, 10 | Relatório por serviço e por OS | Implementado; intervalos com e sem amostras testados |
| JWT administrativo | 4, 11 | Autenticação e testes de autorização | Implementado e testado |
| Monólito em camadas e justificativa do banco | 4, 10 | Código, arquitetura e decisões | Implementado; uso de JSONB, restrições relacionais e transações documentado |
| REST e OpenAPI/Swagger | 4–6, 10 | Contrato e documentação local | 40 operações; contrato estático e Swagger servidos pelo backend |
| Dockerfile e Compose | 4 | Ambiente completo reproduzível | Arquivos preparados; configuração validada; execução dos contêineres pendente |
| Testes unitários e integração; cobertura mínima | 4–5, 11 | Relatórios reproduzíveis da versão entregue | Testes aprovados; domínio com 99,0%; cobertura global de 85,6% |
| Análise de vulnerabilidades | 5, 7 | Relatório com data, versão analisada e tratamento de achados | Verificação local registrada; detalhamento abaixo |
| README completo | 5–6 | Objetivo, execução, testes, autenticação, arquitetura e links | Instruções locais documentadas; links públicos pendentes |
| Event Storming, C4 até o nível 3 e linguagem ubíqua | 8–9 e orientação complementar | Diagramas, glossário e exportações acessíveis | Markdown, seis PNGs e dois PDFs concluídos localmente; publicação pendente |
| Repositório público; main protegida; PRs | 5–6 | Configuração e histórico no GitHub | Repositório inicializado; publicação por PR, proteção e CI em andamento |
| Branch e hash da entrega | 6 | Versão identificada no PDF e coerente com o vídeo | Pendente da versão final |
| Vídeo de até 15 minutos | 6–7 e orientação complementar | Demonstração e identificação; YouTube, Vimeo, Drive ou arquivo acessível | MP4 local versão 3 de 11min00s, com operação do Swagger, concluído e verificado; disponibilização pendente |
| PDF único de entrega e identificação do grupo | 4–5 e orientação complementar | Grupo, nomes, RMs, tecnologias e links | Identificação recebida; PDF e links finais pendentes |

Conforme orientação do professor, a documentação DDD reúne Event Storming, linguagem ubíqua e C4 nos níveis 1, 2 e 3. O C4 pode detalhar um cenário: o nível 3 deste projeto cobre o início do primeiro serviço e a baixa de estoque. O modelo é uma construção conceitual do projeto, sem alegação de workshop com especialistas.

Exportações locais: [C4 em PDF](ddd/exports/c4.pdf), [Event Storming em PDF](ddd/exports/event-storming.pdf) e [imagens e fontes](ddd/c4.md). A [linguagem ubíqua](ddd/dominio.md) está junto do modelo de domínio. Esses arquivos ainda precisam ser disponibilizados em links acessíveis para avaliação.

## Verificações locais e limites

O [resultado de integração](evidencias/integracao.txt) registra autenticação, cadastros, fluxo completo de OS, snapshots, revisão de orçamento, isolamento e rotação de tokens, estoque insuficiente e concorrência. Os testes foram executados contra PostgreSQL 18.6, fora de contêineres. O daemon Docker local não iniciou; validar o Compose não comprova que as imagens foram construídas ou que o ambiente subiu.

O [resultado do govulncheck](evidencias/govulncheck.txt) registra zero vulnerabilidades em código alcançável e zero em pacotes importados, mas um achado em módulo requerido sem chamada identificada. Esse resultado não equivale a ausência total de achados em dependências. Conforme orientação do professor, o relatório não precisa estar sem achados; deve apresentar o resultado real, as versões analisadas e a interpretação dos avisos.

A CI prevista executa verificações Go, testes, cobertura e segurança. A execução remota no GitHub e as configurações de proteção do repositório permanecem pendentes. Swagger precisa de internet para carregar os assets da CDN; a especificação e as operações da API são servidas localmente.

## Cobertura

O enunciado especifica 80% nos domínios críticos nas páginas 4 e 11 e usa uma formulação geral na página 5. A medição do pacote `internal/domain` é 99,0%. A cobertura global é **85,6%**, incluindo os pontos de entrada e os pacotes de persistência, HTTP e estoque, medida com `-coverpkg=./...` e integração com PostgreSQL real. Os testes verificam regras de negócio, autorização, concorrência, entradas inválidas e rollback. Os comandos, versões e limites estão no [registro de verificações](evidencias/verificacao.md), e os resultados por função no [relatório de cobertura](evidencias/cobertura.txt).

## Preparação do PDF final

Incluir Grupo 1, Lucas Velôso, RM 377934, curso SOAT, fase 1, stack, branch e hash final. Acrescentar links para o repositório, diagramas, vídeo e relatório de vulnerabilidades; incluir o glossário separadamente se ele não estiver junto da documentação DDD.

O [vídeo local, versão 3](../entrega/video/README.md), tem 11min00s, narração neural em português, identificação do integrante, diagramas e capturas da operação real no Swagger. A demonstração cria os cadastros e acompanha a OS nº 5, com serviço, peças e insumo, até a entrega. A edição corta esperas e oculta credenciais. O MP4, as legendas, os 27 capítulos e as evidências estão em `entrega/video/`; as versões anteriores permanecem preservadas.

O vídeo pode ser entregue por YouTube, Vimeo, Drive ou arquivo acessível. Conferir o acesso dos avaliadores e garantir que vídeo, diagramas e relatórios correspondam à mesma versão. A publicação no repositório e a validação em contêineres estão em andamento; o PDF final depende dos links publicados e da identificação da versão.
