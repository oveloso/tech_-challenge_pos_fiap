# Verificações — 28 e 29/10/2026

## Registro histórico no Windows — 28/10/2026

Ambiente: Windows, Go 1.27.3 e PostgreSQL 18.6 nativo. A API atende somente em `127.0.0.1:8080` na execução local. Não foi necessário alterar o firewall. Os testes usam um banco separado, `oficina_test`.

| Verificação | Resultado |
|---|---|
| `gofmt -l ./backend` | Nenhum arquivo pendente de formatação |
| `go vet ./...` e `go build ./...`, em `backend` | Aprovados |
| Testes unitários e integração com PostgreSQL | Aprovados; [saída](testes.txt) |
| Cobertura global com `-coverpkg=./...` | **85,6%**; [resultado por função](cobertura.txt) |
| Cobertura de `internal/domain` | **99,0%** |
| Fluxo HTTP completo, concorrência e rollback | Aprovados; [fluxos](integracao.txt) e [entradas inválidas](integracao-entradas-hostis.txt) |
| `govulncheck` 1.9.0 | Zero vulnerabilidades alcançáveis; ressalva abaixo |
| `docker compose config --quiet` | Configuração válida |
| Contrato OpenAPI 3.0.3 | Validado contra o schema oficial; 40 operações, 26 caminhos e 380 referências resolvidas |
| Swagger, chamada `GET /api/v1/health` | HTTP 200 e `{"status":"ok"}` confirmados no navegador |
| Exportações DDD | Seis PNGs 1920 × 1080 e dois PDFs de três páginas; imagens e páginas renderizadas conferidas visualmente |

Para reproduzir no PowerShell, com Go no PATH, `.env` configurado e o banco de testes criado:

```powershell
./scripts/run.ps1 -Action test -DatabasePort 55432
Set-Location backend
go tool cover -func=coverage.out
go test -count=1 -coverprofile=domain-coverage.out ./internal/domain
go tool cover -func=domain-coverage.out
go vet ./...
go build ./...
go run golang.org/x/vuln/cmd/govulncheck@v1.9.0 -show verbose ./...
```

A porta `55432` corresponde ao PostgreSQL usado nesta verificação. Ajuste-a à sua instalação. O comando de testes executa `go test -count=1 -coverpkg=./... -coverprofile=coverage.out ./...`; sem `TEST_DATABASE_URL`, a integração é ignorada e a cobertura será menor.

O [relatório de segurança](govulncheck.txt) registra o aviso GO-2026-5932 referente a `golang.org/x/crypto/openpgp`, um pacote descontinuado que a aplicação não importa. O módulo `golang.org/x/crypto` é utilizado para bcrypt. Foram encontrados zero problemas nos símbolos alcançáveis e nos pacotes importados, e um aviso em módulo requerido. Conforme orientação do professor, não é necessário zerar os achados para apresentar o relatório. O resultado não substitui revisão de segurança nem garante ausência de vulnerabilidades.

## Docker e CI — 29/10/2026

A dificuldade inicial de inicialização do Docker Desktop foi superada. A [evidência Docker](docker.txt) registra o build e a execução real do Compose, com PostgreSQL 18.6, seed concluído, API/banco saudáveis e Swagger/OpenAPI disponíveis. O ambiente de verificação usou as portas 18080 e 15432, preservando a API e o banco nativos.

| Verificação | Resultado |
|---|---|
| Build, seed, saúde e Swagger/OpenAPI | Aprovados |
| [Fluxo HTTP em Docker](docker-fluxo.json) | 32 requisições e 47 verificações aprovadas |
| [Persistência após recriação dos contêineres](docker-persistencia.json) | 8 verificações aprovadas; volume preservado |
| [Testes Go no contêiner](docker-testes.txt) | Unitários e integração aprovados; global 85,8%, domínio 99,0% |
| [CI 40883124842](https://github.com/oveloso/tech_-challenge_pos_fiap/actions/runs/40883124842) | Jobs `backend` e `compose-smoke` aprovados |

O job `backend` executou testes com `-race` em Linux e cobertura global de 85,8%; o domínio permaneceu em 99,0%. O detector de corridas não foi executado no Windows nem no contêiner local com CGO desabilitado. A medição Windows de 85,6% acima é mantida como registro histórico de outra execução. Cada execução da CI identifica o commit analisado; os checks de cada revisão ficam vinculados ao PR.

## Documentação e vídeo

O Swagger é servido pela própria API em `/docs` e carrega os assets da versão 5.33.1 pela CDN. A especificação em `/openapi.yaml` e as operações da API são locais. A evidência de execução no Swagger está em [swagger.png](swagger.png).

As exportações locais cobrem [Event Storming](../ddd/exports/event-storming.pdf), [C4 níveis 1, 2 e 3](../ddd/exports/c4.pdf) e a [linguagem ubíqua](../ddd/dominio.md), conforme orientação do professor. O nível 3 detalha um cenário: início do primeiro serviço com baixa transacional de estoque. As fontes e imagens estão em [C4 da oficina](../ddd/c4.md).

A identificação é **Grupo 1 — Lucas Velôso — RM 377934**. Discord: **Não utiliza Discord**. O [vídeo final](../../entrega/video/README.md) tem **11min06,85s**, 27 capítulos, 201 entradas de legenda e narração sintética em português. As capturas mostram a operação do Swagger, criando a OS nº 5 com serviço, peças e insumo até a entrega; orçamento de R$ 338, peças 8→6 e insumo 5→4 foram confirmados. A edição final atualiza a validação Docker, a cobertura Linux/CI e os créditos de apoio de IA da OpenAI/Codex (GPT-6). Duração, codecs, capítulos e decodificação integral foram verificados; o [registro final](../../entrega/video/verificacao-final.json) inclui os hashes do MP4 e do SRT. As versões anteriores foram preservadas fora do histórico Git.

A [fonte editável dos diagramas](../ddd/exports/oficina-ddd.excalidraw) acompanha os PDFs e PNGs. Quadro compartilhado: [abrir no Excalidraw](https://excalidraw.com/#json=y-XhNVpZNgjkyZGcGTIug,NSCIC96zvDJ97e1Kz-Tjig).

O repositório público e a proteção da `main` estão configurados: PR obrigatório, checks `backend`/`compose-smoke`, aplicação a administradores, sem force push nem exclusão. Por se tratar de grupo individual, não há aprovação adicional obrigatória. O [PR #1](https://github.com/oveloso/tech_-challenge_pos_fiap/pull/1) registra a implementação e suas verificações. A [release fase-1-v1](https://github.com/oveloso/tech_-challenge_pos_fiap/releases/tag/fase-1-v1) identifica o código e reúne vídeo, legendas e PDF de entrega com o commit completo. O escopo verificado é a API local por HTTP, sem implantação pública.
