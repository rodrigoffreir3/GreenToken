# SPEC GT-01 — GreenToken: AI Energy FinOps Observability Agent

**Versão:** 1.0
**Autor:** Rodrigo (arquitetura) + revisão adversarial
**Repositório alvo:** `github.com/rodrigoffreir3/greentoken` (novo, GPL v3)
**Repositório de referência:** `imunno-system` (raiz apontada como `$IMUNNO_ROOT`)
**Executor:** Antigravity squad (Gemini agentic)
**Stack:** Go 1.24, eBPF (cilium/ebpf), NVML (go-nvml), gRPC, Prometheus exporter

---

## 0. Princípio inviolável do produto

GreenToken **OBSERVA. NUNCA ATUA.**

Não há throttling, não há cgroups de contenção, não há `bpf_send_signal`, não há `log.Fatal` que derrube processo. Zero risco de SLA. Toda a tese comercial (FinOps de IA, dor recorrente do CFO) depende de o produto ser provadamente incapaz de derrubar uma inferência em produção. Qualquer task que introduza atuação sobre workload é uma violação de spec e deve ser rejeitada.

A métrica central que o produto entrega:

```
custo_por_token = (W_cpu + W_dram + W_gpu) × tempo_inferência / tokens_gerados
```

Nenhuma ferramenta de mercado (Prometheus, DCGM, OpenTelemetry) correlaciona automaticamente `PID do worker LLM → watts_cpu + watts_gpu + tokens_gerados` com granularidade de eBPF. Esse é o diferencial defensável.

---

## 1. Delta técnico preciso — Imunno → GreenToken

Legenda de ação:
- **COPY** — copiar o arquivo praticamente como está, ajustando apenas `package`/imports/comentário de cabeçalho.
- **ADAPT** — copiar e modificar a lógica interna.
- **REWRITE** — usar como referência conceitual, mas escrever do zero (propósito diferente).
- **NEW** — não existe no Imunno, criar do zero.
- **SKIP** — explicitamente NÃO trazer (acoplamento de segurança, atuação, ou subpacote irrelevante).

### 1.1 Camada de coleta CPU/DRAM (RAPL)

| Arquivo origem (`$IMUNNO_ROOT`) | Arquivo destino (greentoken) | Ação | Observação |
|---|---|---|---|
| `imunno-agent/energy/rapl.go` | `agent/energy/rapl.go` | **COPY** | Já tem `ReadEnergyUJ`, `ReadWatts(d)`, `StartRAPLTelemetry`. Trocar comentário de cabeçalho (não fala mais de "estrangulamento"). Manter o graceful fallback. |
| `imunno-agent/energy/rapl_test.go` | `agent/energy/rapl_test.go` | **COPY** | Testes de tmpdir já válidos. |
| — | `agent/energy/rapl_domains.go` | **NEW** | RAPL multi-domínio: o `rapl.go` atual só lê `intel-rapl:0` (pacote). Precisamos enumerar `intel-rapl:N` e os subdomínios `intel-rapl:0:0` (core) e `intel-rapl:0:1` (dram) para separar W_cpu de W_dram. Tratar wraparound do contador `max_energy_range_uj`. |

### 1.2 Camada de coleta GPU (NVML) — a parte nova crítica

| Arquivo | Ação | Observação |
|---|---|---|
| `agent/gpu/nvml.go` | **NEW** | Bindings `github.com/NVIDIA/go-nvml`. Lê por device: `nvmlDeviceGetPowerUsage` (mW), `nvmlDeviceGetMemoryInfo`, `nvmlDeviceGetUtilizationRates`, `nvmlDeviceGetTemperature`. Graceful fallback OBRIGATÓRIO: se NVML não inicializar (host sem GPU NVIDIA), retornar W_gpu=0 e seguir, espelhando o padrão do `rapl.go`. |
| `agent/gpu/process_map.go` | **NEW** | `nvmlDeviceGetComputeRunningProcesses` → mapeia PID → GPU device + memória usada. Esse é o link PID↔GPU. Em MIG, usar `nvmlDeviceGetComputeRunningProcesses` por GPU instance. |
| `agent/gpu/nvml_stub.go` | **NEW** | Build tag `//go:build !cgo` ou `!gpu` — stub que retorna zeros, para compilar/testar em máquina sem CUDA/NVML. NVML exige cgo; o CI e a máquina Vaio FE16 não têm GPU NVIDIA. |
| `agent/gpu/nvml_test.go` | **NEW** | Testa o stub e o parsing de mW→W. Não testa hardware real. |

### 1.3 Camada eBPF (correlação temporal)

| Arquivo origem | Arquivo destino | Ação | Observação |
|---|---|---|---|
| `imunno-agent/bpf/process_monitor.c` | `agent/bpf/sched_monitor.c` | **REWRITE** | Conceito de ringbuffer + maps reaproveitado, mas o `.c` atual hooka eventos de segurança (UID 33, shell redirect, RCE). GreenToken precisa de `tracepoint/sched/sched_switch` ou `perf_event` para janelas de execução por PID. SEM `bpf_send_signal`, SEM modos ENFORCE/HADES. Só observação. |
| `imunno-agent/bpf_loader.go` | `agent/bpf_loader.go` | **ADAPT** | Estrutura de load do cilium/ebpf (`loadBpfObjects`, ringbuf reader) é reaproveitável. Remover `UpdateConfigMode`, `UpdateHadesJailMap` (atuação). Manter graceful fallback para `startPassiveProcScanner`. |
| `imunno-agent/bpf_bpfel.go`, `bpf_bpfeb.go` | regenerar | **NEW** | São gerados por `bpf2go`. Regenerar a partir do novo `.c` via `go generate`. Não copiar os do Imunno. |
| `imunno-agent/ringbuffer.go` | `agent/ringbuffer.go` | **ADAPT** | Wrapper de ringbuffer reaproveitável; trocar o tipo de evento de `ThreatEvent` para `EnergyWindow`. |

### 1.4 Camada de transporte (gRPC)

| Arquivo origem | Arquivo destino | Ação | Observação |
|---|---|---|---|
| `pb/imunno.proto` | `pb/greentoken.proto` | **ADAPT** | Adicionar `EnergyEvent` ao `oneof payload` e o serviço. Spec do proto na seção 2. |
| `pb/imunno.pb.go`, `imunno_grpc.pb.go` | regenerar | **NEW** | Gerar via `protoc` do novo `.proto`. |
| `pb/go.mod` | `pb/go.mod` | **COPY** | Trocar module path para `greentoken/pb`. |
| `imunno-collector/grpc_handler.go` | `collector/grpc_handler.go` | **REWRITE** | O handler atual depende de `hub`, `ml_client`, `publisher`, `wp_verifier`, `analyzer`, `BehavioralScorer` — todo o stack de segurança. **SKIP todos esses subpacotes.** Escrever handler enxuto que recebe `EnergyEvent` e empurra para o agregador. Reaproveitar só o padrão de `StreamEvents`, rate limiter e `processTree sync.Map`. |

### 1.5 Camada de agregação e export (nova)

| Arquivo | Ação | Observação |
|---|---|---|
| `collector/aggregator/aggregator.go` | **NEW** | Junta as três fontes (W_cpu, W_dram, W_gpu) por PID/workload numa janela temporal. Calcula `custo_por_token`. |
| `collector/aggregator/cost.go` | **NEW** | `custo_por_token`, `joules_por_request`, preço de kWh configurável (env `GT_KWH_PRICE`, default tarifa média BR). |
| `collector/exporter/prometheus.go` | **NEW** | Endpoint `/metrics` formato Prometheus. Métricas: `greentoken_watts_cpu`, `_watts_dram`, `_watts_gpu`, `_tokens_total`, `_cost_per_token`, `_joules_per_request`. Labels: `workload`, `model`, `pid`, `gpu_index`. Usar `prometheus/client_golang`. |
| `collector/tokens/hook.go` | **NEW** | Conta tokens gerados. Estratégia v1: parser de stdout/log de `vllm`/`llama.cpp`/Ollama (regex de "generated N tokens" / formato de log conhecido). Estratégia v2 (fora de escopo GT-01): `/metrics` nativo do vllm. |

### 1.6 Build, deploy e CLI

| Arquivo origem | Arquivo destino | Ação | Observação |
|---|---|---|---|
| `imunno-agent/Makefile`, raiz `Makefile` | `Makefile` | **ADAPT** | Targets: `build-agent`, `build-collector`, `generate-bpf`, `generate-proto`, `test`, `docker`. |
| `docker-compose.yml` | `docker-compose.yml` | **ADAPT** | Serviços: `greentoken-agent` (privileged, mounts `/sys/class/powercap`, `/sys/fs/cgroup` read-only), `greentoken-collector`, `prometheus`, `grafana`. **SKIP** postgres/rabbitmq do Imunno (não há persistência de ameaça aqui; métricas vivem no Prometheus). |
| `deploy/install.sh`, `imunno-agent.service` | `deploy/` | **ADAPT** | systemd unit do agent. Remover dependências de watchdog de hardware. |
| `imunno-agent/watchdog/hardware.go` | — | **SKIP** | O `log.Fatal` que força reboot físico é inaceitável num observability tool de produção. NÃO trazer. |
| `imunno-agent/cgroups/*` | — | **SKIP** | Atuação/contenção. Viola o princípio da seção 0. |
| `imunno-collector/genetic/*` | — | **SKIP** (GT-01) | Brilhante, mas é tuning de scorer de ameaça. Sem uso em FinOps na v1. Reavaliar em spec futura se houver "perfil de eficiência" a evoluir. |
| `imunno-agent/p2p/*` | — | **SKIP** (GT-01) | Gossip de vacina é conceito de segurança. Fora de escopo. |
| — | `cmd/greentoken/main.go` | **NEW** | CLI: `greentoken report --model llama3 --since 1h`, `greentoken serve`. |

### 1.7 Resumo quantitativo do delta

- **COPY:** 4 arquivos (RAPL + testes + go.mod pb)
- **ADAPT:** 7 arquivos (bpf_loader, ringbuffer, proto, Makefile, compose, deploy, grpc rate-limit pattern)
- **REWRITE:** 3 arquivos (sched_monitor.c, grpc_handler, proto handler)
- **NEW:** ~12 arquivos (toda camada GPU, agregação, exporter, token hook, CLI)
- **SKIP:** cgroups, watchdog, genetic, p2p, hub, ml_client, publisher, wp_verifier, postgres, rabbitmq

Reaproveitamento real honesto: **~30–35% do código**, **~60% da arquitetura e dos padrões** (graceful fallback, ringbuffer, gRPC streaming, estrutura de pacotes). O número "80%" do Gemini era inflado; o real ainda é forte.

---

## 2. Especificação do `greentoken.proto`

```proto
syntax = "proto3";
package greentoken;
option go_package = "./pb";

service GreenTokenCollector {
  rpc StreamEnergy(stream EnergyEvent) returns (stream Ack);
}

message EnergyEvent {
  int64  timestamp_ns   = 1;
  string agent_id       = 2;
  string hostname       = 3;
  int32  pid            = 4;
  string workload       = 5;   // ex: "vllm", "llama.cpp", "ollama"
  string model          = 6;   // ex: "llama3-70b"

  // Energia na janela de medição
  double watts_cpu      = 7;
  double watts_dram     = 8;
  double watts_gpu      = 9;
  int32  gpu_index      = 10;  // -1 se sem GPU

  // Throughput na mesma janela
  int64  tokens_in_window   = 11;
  double window_seconds     = 12;

  // Utilização (para contexto, não para atuar)
  double cpu_util_pct   = 13;
  double gpu_util_pct   = 14;
  uint64 gpu_mem_used   = 15;
}

message Ack {
  bool   ok      = 1;
  string message = 2;
}
```

Note: `oneof` não é necessário aqui porque GreenToken tem um único tipo de evento. Mais simples que o Imunno de propósito.

---

## 3. Estrutura final do repositório

```
greentoken/
├── agent/
│   ├── energy/
│   │   ├── rapl.go            COPY
│   │   ├── rapl_domains.go    NEW
│   │   └── rapl_test.go       COPY
│   ├── gpu/
│   │   ├── nvml.go            NEW
│   │   ├── process_map.go     NEW
│   │   ├── nvml_stub.go       NEW
│   │   └── nvml_test.go       NEW
│   ├── bpf/
│   │   └── sched_monitor.c    REWRITE
│   ├── tokens/
│   │   └── parser.go          NEW
│   ├── bpf_loader.go          ADAPT
│   ├── ringbuffer.go          ADAPT
│   ├── main.go                NEW
│   └── go.mod                 NEW
├── collector/
│   ├── aggregator/
│   │   ├── aggregator.go      NEW
│   │   └── cost.go            NEW
│   ├── exporter/
│   │   └── prometheus.go      NEW
│   ├── grpc_handler.go        REWRITE
│   ├── main.go                NEW
│   └── go.mod                 NEW
├── pb/
│   ├── greentoken.proto       ADAPT
│   └── go.mod                 COPY
├── cmd/greentoken/
│   └── main.go                NEW
├── dashboard/grafana/
│   └── greentoken.json        NEW
├── deploy/
│   ├── greentoken-agent.service  ADAPT
│   └── install.sh                ADAPT
├── docker-compose.yml         ADAPT
├── Makefile                   ADAPT
├── LICENSE                    NEW (GPL v3)
└── README.md                  NEW
```

---

## 4. TASK BUDGET para o Antigravity

Formato: cada task é atômica, tem critério de aceite verificável, e um teto de tokens/iterações para não estourar o orçamento da squad. Ordem é topológica — respeitar dependências.

Convenção de teto: **S** = pequena (≤1 arquivo, ~15k tokens), **M** = média (~40k), **L** = grande (~80k). Total estimado do budget: ~620k tokens de execução.

### FASE 0 — Bootstrap do repositório (budget: ~30k)

**T0.1 [S]** Criar estrutura de diretórios vazia + `go.mod` dos três módulos (`agent`, `collector`, `pb`) com `replace greentoken/pb => ../pb`. Adicionar `LICENSE` GPL v3 e `README.md` esqueleto.
- Aceite: `tree` bate com a seção 3; `go mod verify` passa nos três módulos.
- Ref: `$IMUNNO_ROOT/imunno-agent/go.mod` para o padrão de replace.

**T0.2 [S]** Configurar `.gitignore`, `Makefile` esqueleto com targets vazios e `go generate` directives comentadas.
- Aceite: `make help` lista os targets.

### FASE 1 — Camada de energia CPU/DRAM (budget: ~70k)

**T1.1 [S]** COPY `rapl.go` + `rapl_test.go` de `$IMUNNO_ROOT/imunno-agent/energy/` para `agent/energy/`. Trocar package (mantém `energy`), reescrever comentário de cabeçalho removendo menção a throttling.
- Aceite: `go test ./agent/energy/` verde.

**T1.2 [M]** NEW `rapl_domains.go`: enumerar `/sys/class/powercap/intel-rapl:*`, identificar subdomínio `core` vs `dram` lendo o arquivo `name`. Expor `ReadDomainWatts(d time.Duration) (cpu, dram float64)`. Tratar wraparound via `max_energy_range_uj`.
- Aceite: teste com tmpdir simulando hierarquia `intel-rapl:0/{name=package-0}`, `intel-rapl:0:0/{name=core}`, `intel-rapl:0:1/{name=dram}` retorna valores separados corretos; wraparound (delta negativo) não produz watts negativo.
- Ref: padrão de tmpdir em `rapl_test.go`.

### FASE 2 — Camada GPU NVML (budget: ~120k) — CAMINHO CRÍTICO

**T2.1 [S]** Adicionar dependência `github.com/NVIDIA/go-nvml` ao `agent/go.mod`. Criar `nvml_stub.go` com build tag `//go:build !gpu` retornando zeros para toda a interface.
- Aceite: `go build ./agent/...` compila sem GPU e sem cgo (tag default = stub).

**T2.2 [M]** NEW `nvml.go` (build tag `//go:build gpu`): init NVML com graceful fallback, `ReadGPUPower(index) (watts float64, err)` via `nvmlDeviceGetPowerUsage` (mW→W), util, mem, temp. Espelhar o padrão de fallback do `rapl.go`.
- Aceite: code review confirma fallback retorna zeros sem panic se `nvmlInit` falha; conversão mW→W correta (dividir por 1000); função documentada.

**T2.3 [M]** NEW `process_map.go`: `MapProcessesToGPU() map[int32]GPUUsage` via `nvmlDeviceGetComputeRunningProcesses`. Suportar enumeração de múltiplas GPUs.
- Aceite: assinatura e estrutura `GPUUsage{PID, GPUIndex, MemUsed}` definidas; teste do stub retorna mapa vazio sem erro.

**T2.4 [S]** NEW `nvml_test.go`: testar parsing mW→W e o comportamento do stub.
- Aceite: `go test -tags '' ./agent/gpu/` verde (roda contra stub).

### FASE 3 — eBPF sched monitor (budget: ~110k)

**T3.1 [L]** REWRITE `agent/bpf/sched_monitor.c`: hook `tracepoint/sched/sched_switch`, emitir `EnergyWindow{pid, comm, on_cpu_ns}` via ringbuffer. SEM `bpf_send_signal`, SEM config_map de modos, SEM jail map.
- Aceite: `clang` compila o objeto BPF sem erro; `bpftool` valida o programa; nenhuma chamada de atuação presente (grep por `bpf_send_signal` retorna vazio).
- Ref: estrutura de maps e ringbuffer de `$IMUNNO_ROOT/imunno-agent/bpf/process_monitor.c` (só a forma, não o conteúdo de segurança).

**T3.2 [M]** ADAPT `bpf_loader.go` + `ringbuffer.go`: regenerar via `go generate` (bpf2go), remover `UpdateConfigMode`/`UpdateHadesJailMap`, trocar tipo de evento para `EnergyWindow`, manter graceful fallback para scanner passivo via `/proc`.
- Aceite: `go generate ./agent/...` gera os `bpf_bpfel.go`; agent compila; fallback testado quando eBPF indisponível.

### FASE 4 — Proto e transporte gRPC (budget: ~70k)

**T4.1 [S]** ADAPT `pb/greentoken.proto` conforme seção 2. COPY `pb/go.mod` ajustando module path. Gerar `.pb.go` via `protoc`.
- Aceite: `make generate-proto` produz os arquivos; `go build ./pb/` verde.

**T4.2 [M]** REWRITE `collector/grpc_handler.go`: handler enxuto de `StreamEnergy`. Reaproveitar APENAS o padrão de rate limiter e `processTree sync.Map` de `$IMUNNO_ROOT/imunno-collector/grpc_handler.go`. Empurrar eventos para o agregador. ZERO imports de hub/ml_client/publisher/wp_verifier/analyzer.
- Aceite: `go build ./collector/` verde; grep confirma ausência dos imports proibidos; teste de stream com evento mock chega ao agregador.

### FASE 5 — Agregação, custo e token hook (budget: ~90k)

**T5.1 [M]** NEW `collector/aggregator/aggregator.go` + `cost.go`: janela temporal por PID, soma das três fontes de energia, cálculo de `custo_por_token` e `joules_por_request`. Preço kWh via env `GT_KWH_PRICE`.
- Aceite: teste unitário com entradas conhecidas (ex: 100W total, 2s, 500 tokens) produz custo/token esperado; divisão por zero tokens tratada (retorna 0, não NaN).

**T5.2 [M]** NEW `agent/tokens/parser.go`: parser de stdout de vllm/llama.cpp/Ollama via regex de formatos de log conhecidos. Interface `CountTokens(line string) (n int64, matched bool)`.
- Aceite: testes com linhas de log reais de cada engine (fixtures) extraem a contagem correta; linha irrelevante retorna `matched=false`.

### FASE 6 — Prometheus exporter e dashboard (budget: ~60k)

**T6.1 [M]** NEW `collector/exporter/prometheus.go`: endpoint `/metrics` com `client_golang`. Métricas e labels da seção 1.5. Todos os valores numéricos arredondados.
- Aceite: `curl localhost:PORT/metrics` retorna formato Prometheus válido; `promtool check metrics` passa.

**T6.2 [S]** NEW `dashboard/grafana/greentoken.json`: dashboard com custo/token por workload, watts por fonte ao longo do tempo, top workloads por consumo.
- Aceite: JSON importa no Grafana sem erro de schema.

### FASE 7 — CLI, deploy e integração (budget: ~70k)

**T7.1 [M]** NEW `cmd/greentoken/main.go`: subcomandos `serve` (sobe collector+exporter) e `report --model X --since 1h` (consulta Prometheus, imprime tabela de custo/token).
- Aceite: `greentoken report --help` documenta flags; `serve` sobe sem panic.

**T7.2 [M]** ADAPT `docker-compose.yml` + `deploy/`: serviços agent (privileged, mounts read-only de `/sys/class/powercap`), collector, prometheus, grafana. systemd unit sem watchdog.
- Aceite: `docker compose config` valida; `docker compose up` sobe os 4 serviços; agent reporta watts no log; `/metrics` responde.

**T7.3 [S]** NEW `README.md` completo: arquitetura, o princípio "observa, nunca atua", quickstart, a equação de custo/token, nota de GPL v3.
- Aceite: README cobre build, run e a tese do produto.

### Gate de validação final (não é task, é checklist de aceite do GT-01)

- [ ] `grep -rn "bpf_send_signal\|cgroup.procs\|log.Fatal" agent/ collector/` retorna vazio (princípio da seção 0).
- [ ] `go test ./...` verde nos três módulos (contra stub de GPU).
- [ ] `docker compose up` funciona numa máquina SEM GPU NVIDIA (graceful fallback W_gpu=0).
- [ ] `/metrics` expõe `greentoken_cost_per_token`.
- [ ] Nenhum import de subpacote de segurança do Imunno presente.

---

## 5. Riscos e decisões em aberto (honestidade adversarial)

1. **Token counting é frágil na v1.** Parser de stdout quebra se o engine muda o formato de log. É o elo mais fraco. Aceitável para MVP/validação, mas a v2 precisa do `/metrics` nativo do vllm ou de um hook no próprio runtime. Não venda isso como robusto ainda.

2. **Correlação PID↔GPU em MIG é não-trivial.** Em GPUs particionadas (MIG), o mapeamento processo→instância exige cuidado extra. T2.3 cobre o caso simples; MIG completo pode virar spec própria.

3. **NVML exige cgo e a Vaio FE16 não tem GPU NVIDIA.** Por isso o stub (T2.1) é caminho crítico e não opcional — sem ele você não compila nem testa localmente. Toda a Fase 2 real só valida em máquina com GPU (cloud com instância GPU, ou validação remota).

4. **O valor depende de você ter acesso a um host de inferência real para medir.** Antes de investir as 6–8 semanas, vale alugar uma instância GPU por algumas horas e provar que a correlação PID→watts_gpu→tokens fecha numericamente. Se não fechar, a tese inteira treme. Sugiro um T-spike de validação ANTES da Fase 2 completa.

5. **Concorrência existe** (DCGM exporter, Kepler do CNCF que já faz energy attribution via eBPF+RAPL). O diferencial do GreenToken precisa ser a correlação com *tokens* e o ângulo FinOps de custo/token — não "medir watts", que o Kepler já faz. **Pesquisar o Kepler antes de codar** é tarefa zero não-negociável; pode ser que parte do caminho já esteja resolvido e você deva construir em cima dele em vez de do zero.
