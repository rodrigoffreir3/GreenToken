# Changelog

All notable changes to this project will be documented in this file.

The format is based on [Keep a Changelog](https://keepachangelog.com/en/1.0.0/),
and this project adheres to [Semantic Versioning](https://semver.org/spec/v2.0.0.html).

## [0.1.0] - 2026-07-23

### Added
- Agent: coleta de energia via RAPL (CPU/DRAM) e NVML (GPU) com fallback gracioso.
- Agent: correlação por PID via eBPF (`sched_switch`) com fallback para scanner `/proc`.
- Agent: três fontes de tokens configuráveis via `-token-source`: `prometheus` (padrão, fonte de verdade), `logsniffer` (fallback best-effort), `none`.
- Collector: agregação de eventos de energia por PID e cálculo de custo por token em tempo real.
- Collector: exporter Prometheus (`/metrics`) com métricas `greentoken_watts_cpu`, `_watts_dram`, `_watts_gpu`, `_tokens_total`, `_cost_per_token`, `_joules_per_request`.
- CLI unificada `greentoken` com subcomandos `serve`, `report` e `doctor` (diagnóstico do ambiente).
- Pipeline CI/CD com GitHub Actions para geração de binários multi-plataforma e verificação SHA256 no instalador `install.sh`.
- Validado em benchmark: atribuição PID-GPU via NVML (H1), delta de watts sob carga (H2), contagem de tokens via `/metrics` com 0% de erro sob rajada concorrente (H3), estabilidade de custo/token entre execuções (H4).

### Known Limitations
- `logsniffer` (fallback de contagem de tokens via log) tem viés sob rajadas de gerações idênticas; usar apenas quando o engine de inferência não expõe `/metrics`.
- Atribuição de energia multi-workload no mesmo host é proporcional ao tempo de CPU, não medição exata por processo.
- Build com suporte a GPU (`-tags gpu`) disponível apenas para `linux/amd64` nesta versão.
