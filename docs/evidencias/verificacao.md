# Verificações locais — 28/10/2026

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

O daemon do Docker Desktop não iniciou neste computador. As imagens e a execução via Compose ainda precisam ser verificadas em um ambiente com Docker funcionando. A CI está configurada para testes com `-race` no Linux, mas não foi executada remotamente. O detector de corridas não foi executado no Windows.

O Swagger é servido pela própria API em `/docs` e carrega os assets da versão 5.33.1 pela CDN. A especificação em `/openapi.yaml` e as operações da API são locais. A evidência de execução no Swagger está em [swagger.png](swagger.png).

As exportações locais cobrem [Event Storming](../ddd/exports/event-storming.pdf), [C4 níveis 1, 2 e 3](../ddd/exports/c4.pdf) e a [linguagem ubíqua](../ddd/dominio.md), conforme orientação do professor. O nível 3 detalha um cenário: início do primeiro serviço com baixa transacional de estoque. As fontes e imagens estão em [C4 da oficina](../ddd/c4.md).

A identificação da entrega é Grupo 1, Lucas Velôso, RM 377934. O [vídeo local, versão 3](../../entrega/video/README.md), foi concluído com 11min00s, 27 capítulos e narração neural em português. As capturas mostram o preenchimento e a execução de chamadas no Swagger, criando a OS nº 5 com serviço, peças e insumo e acompanhando-a até a entrega. Orçamento de R$ 338, saldo de peças 8→6 e insumo 5→4 foram confirmados nas respostas visíveis. A edição reaproveita os diagramas e as evidências de testes e segurança da versão 2. Duração, codecs, capítulos, legendas e decodificação integral foram verificados. As versões anteriores foram preservadas.

O repositório público foi inicializado em 29/10/2026. Publicação por PR, CI, proteção da main e PDF final com os links estão em andamento. O vídeo pode ser disponibilizado por YouTube, Vimeo, Drive ou arquivo acessível. As verificações locais não representam conclusão da entrega acadêmica.
