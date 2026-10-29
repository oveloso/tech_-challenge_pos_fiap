# Vídeo da fase 1

**Grupo 1 — Lucas Velôso — RM 377934.**

Vídeo final: **11min06,85s**, Full HD, 30 fps, 27 capítulos e 201 entradas de legenda em português. A duração está dentro do limite de 15 minutos.

A demonstração mostra o preenchimento e a execução de operações no Swagger: cliente, veículo, serviço, peça, insumo, ordem de serviço, orçamento, aprovação, execução e entrega. O atendimento é a OS nº 5, com orçamento de R$ 338 e saldos de peças 8→6 e insumo 5→4.

As capturas de 29/10/2026 usam dados sintéticos e credenciais ocultas. A edição reduz esperas; a narração é sintética em português. C4, Event Storming, testes e análise de vulnerabilidades completam a apresentação. A versão final registra Docker validado e cobertura Linux/CI de 85,8% global e 99,0% no domínio. Os jobs `backend` e `compose-smoke` passaram na [execução 40883124842](https://github.com/oveloso/tech_-challenge_pos_fiap/actions/runs/40883124842).

O desenvolvimento teve apoio de IA da OpenAI/Codex (GPT-6), sob responsabilidade acadêmica do integrante. A API é demonstrada localmente por HTTP em loopback; o vídeo explica o uso de HTTPS em uma futura exposição pública, que não faz parte desta entrega.

## Arquivos finais

Os arquivos da apresentação acompanham a [release fase-1-v1](https://github.com/oveloso/tech_-challenge_pos_fiap/releases/tag/fase-1-v1):

- [Vídeo MP4 — tech-challenge-fase-1-grupo-1.mp4](https://github.com/oveloso/tech_-challenge_pos_fiap/releases/download/fase-1-v1/tech-challenge-fase-1-grupo-1.mp4).
- [Legendas SRT — tech-challenge-fase-1-grupo-1.srt](https://github.com/oveloso/tech_-challenge_pos_fiap/releases/download/fase-1-v1/tech-challenge-fase-1-grupo-1.srt).
- [PDF de entrega — entrega-fase-1-grupo-1.pdf](https://github.com/oveloso/tech_-challenge_pos_fiap/releases/download/fase-1-v1/entrega-fase-1-grupo-1.pdf).

O [PR #1](https://github.com/oveloso/tech_-challenge_pos_fiap/pull/1) registra a implementação e seus checks. Os arquivos de produção e versões anteriores do vídeo ficam fora do histórico Git.

## Evidências

- [Capítulos finais](capitulos-final.txt) e [verificação técnica e hashes](verificacao-final.json).
- [Respostas observadas no Swagger](../../docs/evidencias/fluxo-swagger.json) e [ordem entregue](../../docs/evidencias/fluxo-swagger.png).
- [Relatórios de verificação](../../docs/evidencias/verificacao.md) e [validação Docker](../../docs/evidencias/docker.txt).
- [C4](../../docs/ddd/exports/c4.pdf), [Event Storming](../../docs/ddd/exports/event-storming.pdf) e [linguagem ubíqua](../../docs/ddd/dominio.md).
- [Fonte editável dos diagramas](../../docs/ddd/exports/oficina-ddd.excalidraw). Quadro compartilhado: [abrir no Excalidraw](https://excalidraw.com/#json=y-XhNVpZNgjkyZGcGTIug,NSCIC96zvDJ97e1Kz-Tjig).
