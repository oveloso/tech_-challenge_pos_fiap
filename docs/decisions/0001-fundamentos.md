# Decisões de arquitetura da fase 1

Data: 28 de outubro de 2026.

Status: decisões adotadas na implementação da API.

## Monólito organizado por funcionalidades e camadas

O enunciado exige um backend monolítico em camadas. A solução possui uma API Go e PostgreSQL compartilhado. `registry` reúne cadastros e estoque; `workorders` coordena OS e orçamentos; `domain` mantém regras independentes de HTTP e banco. `auth`, `platform` e `server` completam a aplicação. Isso permite uma transação única para iniciar serviço e baixar estoque. O custo é que os módulos compartilham implantação; seus limites precisam ser respeitados no código.

## Go com Gin

Go 1.26.0 ou superior é a base do backend, com Gin 1.12.0 na camada HTTP. A [documentação oficial de Go](https://go.dev/doc/tutorial/web-service-gin) demonstra uma API REST com Gin. O framework reduz trabalho de roteamento e binding; regras de negócio permanecem independentes do framework e as validações continuam sendo responsabilidade da aplicação.

## PostgreSQL, JSONB e SQL explícito

PostgreSQL 18 oferece restrições e [transações](https://www.postgresql.org/docs/18/tutorial-transactions.html) para manter os fluxos consistentes. A implementação usa `pgx` e migração SQL versionada. Consultas e bloqueios ficam explícitos; iniciar um serviço pode bloquear a OS e seus itens de estoque na mesma transação.

Cadastros e o agregado de OS são documentos JSONB. Colunas geradas extraem valores necessários a chaves estrangeiras, índices, `UNIQUE` e `CHECK`. Movimentos de estoque permanecem em registros separados, com referência ao item e à OS e unicidade de consumo. A OS conserva snapshots de cliente, veículo e preços aprovados.

Essa escolha reduz mapeamento e preserva a unidade de leitura e gravação do agregado no MVP. O custo é regravar o JSON completo, consultar arrays nos indicadores e depender da aplicação para invariantes internas que não têm uma chave estrangeira ou restrição própria. Não se trata de um modelo SQL totalmente normalizado. Normalizar itens e versões de orçamento será uma opção caso escala, análise ou evolução independente justifiquem o custo.

## API e Swagger servidos pelo backend

A entrega técnica expõe as funcionalidades por API REST. O mesmo processo Go serve Swagger em `GET /docs` e a especificação estática em `GET /openapi.yaml`. Isso mantém uma única aplicação para executar e demonstrar a solução. Os assets Swagger são carregados da CDN jsDelivr com versão fixa 5.33.1; o custo é precisar de internet para a documentação interativa. O contrato e as operações da API são locais. A autorização do Swagger não persiste após recarregar a página.

## Contrato antes dos endpoints

A especificação OpenAPI é a referência das 40 operações para consumidores da API, implementação e documentação. Alterações de comportamento precisam atualizar contrato e verificações relacionadas no mesmo conjunto de mudanças. O custo é manter a especificação estática sincronizada com os handlers e testes.

## Autenticação administrativa e acesso por OS

Funcionários usam JWT com validade de uma hora. Clientes usam token opaco de 256 bits, restrito à OS, com validade padrão de 30 dias e possibilidade de substituição. Ambos são enviados em `Authorization: Bearer`. Isso dispensa um cadastro de contas de clientes no MVP. Quem possuir o token poderá acessar aquela OS; por isso somente seu hash SHA-256 é armazenado, o valor é retornado apenas na emissão e não deve aparecer em logs. A distribuição da credencial fica fora do sistema.

## Estoque baixado no início da execução

A aprovação não reserva peças. A primeira execução verifica e baixa todos os itens aprovados de forma atômica. Isso mantém o fluxo pequeno, mas uma OS aprovada pode aguardar reposição. A API retorna conflito sem efeitos parciais quando o saldo é insuficiente. Uma política futura de reservas exigirá nova decisão e novos testes.
