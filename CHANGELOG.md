# Changelog

All notable changes to this project will be documented in this file.

## [Unreleased]

### Added
- **GT-02:** Abstração `TokenSource` para gerenciar diferentes formas de contagem de tokens gerados.
- Nova implementação de token source via `PrometheusTokenSource` para buscar tokens cumulativos de forma robusta e zero-trust direto do `/metrics` de engines como o vLLM.
- Adicionado flags `-token-source`, `-metrics-url`, `-metrics-name` no `agent`.

### Changed
- **BREAKING CHANGE:** O comportamento padrão do agente mudou. A leitura de tokens via log de `stdout` (sniffer) não é mais o padrão, mesmo passando `-log-file`. O padrão agora é buscar do `prometheus` (`http://localhost:8000/metrics`, metrica `vllm:generation_tokens_total`). Para voltar ao uso de logs, inicie com `-token-source logsniffer -log-file /path/to/log`.
- O sniffer de logs teve sua falha de deduplicação removida. Se a engine duplicar os logs muito rápido, os tokens sofrerão duplo-contagem em `logsniffer` — esse é o comportamento esperado para o fallback. Use `/metrics` se precisar de extrema precisão.
- `collectAndEnqueue` foi refatorado para utilizar uma abordagem cumulativa com deltas de tempo (subtraindo o acumulado da janela anterior do total retornado pelo endpoint).
