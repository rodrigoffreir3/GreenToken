# GreenToken

**AI Energy FinOps Observability Agent**

> Measure what your inference actually costs — down to the token.

---

## What it does

GreenToken is a lightweight observability agent that runs alongside your LLM inference workload and answers one question:

**How much does each token cost in watts, joules, and dollars?**

```
cost_per_token = (W_cpu + W_dram + W_gpu) × inference_time / tokens_generated
```

No throttling. No risk to SLA. GreenToken **observes and measures only** — it never acts on running workloads.

---

## The problem

Every FinOps team tracks cloud spend. Almost none track energy spend at the workload level. The gap between "we pay $X/month for GPU compute" and "model Y costs $Z per 1000 tokens to run" is where GreenToken lives.

Existing tools (Prometheus, DCGM, OpenTelemetry) measure aggregate host energy. None correlate `PID → watts_cpu + watts_gpu → tokens_generated` at eBPF granularity. That correlation is GreenToken's defensible edge.

---

## Architecture

```
┌─────────────────────────────────────────────┐
│  Host Linux (inference server)              │
│                                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
│  │ RAPL     │  │ NVML     │  │ eBPF     │  │
│  │ CPU+DRAM │  │ GPU      │  │ sched    │  │
│  │ watts    │  │ watts    │  │ windows  │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  │
│       └─────────────┴─────────────┘         │
│                     │                       │
│              ┌──────▼──────┐                │
│              │  GreenToken │                │
│              │    Agent    │                │
│              └──────┬──────┘                │
└─────────────────────┼───────────────────────┘
                      │ gRPC stream
              ┌───────▼───────┐
              │  GreenToken   │
              │  Collector    │
              └───────┬───────┘
                      │
          ┌───────────┼───────────┐
          ▼           ▼           ▼
     /metrics      Grafana     CLI report
   (Prometheus)   dashboard   cost/token
```

---

## Metrics exported

| Metric | Description |
|---|---|
| `greentoken_watts_cpu` | CPU package watts (RAPL) per workload |
| `greentoken_watts_dram` | DRAM watts (RAPL) per workload |
| `greentoken_watts_gpu` | GPU watts (NVML) per workload |
| `greentoken_tokens_total` | Tokens generated per workload |
| `greentoken_cost_per_token` | USD cost per token (configurable kWh price) |
| `greentoken_joules_per_request` | Energy per request |

Labels: `workload`, `model`, `pid`, `gpu_index`

---

## Validated

GT-00 spike ran on Kaggle Tesla T4, Qwen2.5-0.5B-Instruct-Q4, 3 runs × 20 requests × 200 tokens:

| Hypothesis | Result | Status |
|---|---|---|
| H1 — PID visible in NVML | PID 214 detected | ✅ |
| H2 — Measurable watts delta under load | +14–17W above idle (scales with model size) | ✅ |
| H3 — Token counting accuracy | 0.0% error | ✅ |
| H4 — Cost/token stability across runs | 1.8% variance | ✅ |

> Validated: 0% token counting error, 1.8% cost/token variance, PID-to-GPU attribution confirmed via NVML.

---

## Current Limitations (GT-01)

Transparency is a core engineering principle for GreenToken. The v1.0 architecture currently operates under these limitations:

1. **Token Allocation is a Heuristic:** In multi-process environments matching the same workload name, tokens sniffed from logs are *distributed proportionally* based on CPU time (`cpuNs`). While this maintains atomicity for the workload as a whole, it assumes "more CPU time = more tokens", which is an approximation (especially if two different models run simultaneously on the same host).
2. **Log Parser Validation:** The 0% error rate for token counting was validated on the GT-00 spike using native API responses (`completion_tokens`). The regex-based log sniffer is a fallback and has not yet been stress-tested against the native API numbers in production.
3. **PID Matching Fragility:** By default, if `--workload` is a name, GreenToken matches by process name (`comm`), which could catch unrelated processes (e.g., matching all `python` processes). **Fix:** You can now pass an exact PID (e.g., `--workload 12345`) to bypass string matching, or rely on GreenToken's NVML heuristic (it will only match `comm` if the process is actually mapped to a GPU, avoiding idle background processes).

---

## Supported inference engines (token counting)

- `vllm` — native `/metrics` Prometheus endpoint
- `llama.cpp` / `llama-cpp-python` — `usage.completion_tokens` from API response
- `Ollama` — `/api/generate` response metadata

---

## Design principles

**1. Observe. Never act.**
GreenToken contains zero throttling logic, zero cgroup writes, zero `bpf_send_signal` calls. It cannot affect a running inference. This is a hard architectural constraint, not a configuration option.

**2. Graceful degradation.**
No GPU? W_gpu = 0, agent continues. No RAPL? W_cpu = 0, agent continues. No eBPF? Falls back to `/proc` polling. The agent never crashes the host.

**3. Prometheus-native.**
Output is standard Prometheus exposition format. Plug into any existing Grafana stack without custom integrations.

**4. Atomic granularity.**
eBPF `sched_switch` tracepoints correlate energy windows to specific PIDs with microsecond precision — not host-level averages.

---

## Quickstart

```bash
# Clone
git clone https://github.com/rodrigoffreir3/greentoken
cd greentoken

# Build
make build-agent build-collector

# Run (requires root for eBPF + RAPL access)
docker compose up

# View metrics
curl localhost:9090/metrics | grep greentoken
```

**Environment variables:**

| Variable | Default | Description |
|---|---|---|
| `GT_KWH_PRICE` | `0.12` | USD per kWh (electricity cost) |
| `GT_GPU_INDEX` | `0` | GPU device index |
| `GT_COLLECTOR_ADDR` | `localhost:50051` | Collector gRPC address |

---

## Relation to Imunno System

GreenToken shares architectural DNA with [Imunno System](https://github.com/rodrigoffreir3/imunno-system) (INPI Patent #512025006506-0): the RAPL energy reader, gRPC streaming transport, and eBPF loader patterns are derived from Imunno's agent. The mission is orthogonal — Imunno defends web servers; GreenToken makes AI inference costs visible.

---

## Roadmap

- **GT-01** — MVP: RAPL + NVML + eBPF sched + Prometheus exporter + Grafana dashboard
- **GT-02** — vllm native `/metrics` integration (replaces stdout token parsing)
- **GT-03** — Multi-GPU + MIG support
- **GT-04** — Cost anomaly detection (statistical baseline per model)
- **GT-05** — Digital Twin: simulate cost of a model before deploying

---

## License

MPL 2.0 — see [LICENSE](LICENSE)

---

## Author

Rodrigo Freire — [github.com/rodrigoffreir3](https://github.com/rodrigoffreir3)

*Solo developer. All architecture, implementation, and validation by the author.*
