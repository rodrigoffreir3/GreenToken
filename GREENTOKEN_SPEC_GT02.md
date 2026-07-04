# SPEC GT-02 — Token Source Abstraction: `/metrics` como Fonte de Verdade

**Versão:** 1.0
**Status:** Aberto — pronto para execução
**Repositório:** `github.com/rodrigoffreir3/GreenToken`
**Executor:** Antigravity squad
**Pré-requisito:** GT-01 implementado e mergeado (commit `a3bf060` ou posterior)
**Bloqueia:** Validação de produção do H3 (token counting < 2% erro sob carga concorrente)

---

## 0. Contexto e justificativa

O GT-00b (mock vLLM, carga concorrente, `max_tokens=50`) provou empiricamente que o parser de log via stdout subconta **80%** dos tokens sob rajada de gerações idênticas. A causa raiz não é o dedup — é a tentativa de reconstruir, a partir de texto não-estruturado, um número que o engine de inferência já expõe de forma estruturada e canônica.

**Decisão arquitetural desta spec:** o token count primário passa a vir do endpoint `/metrics` nativo do vLLM (contador `vllm:generation_tokens_total`), que é monotônico, estruturado e livre de ambiguidade. O log sniffer existente é rebaixado a *fallback best-effort* para engines que não expõem métricas, e fica desativado por padrão.

Esta decisão está alinhada ao roadmap (GT-02 já previsto) e ao princípio validado no GT-00 original, onde o token count veio de fonte estruturada (`completion_tokens`) e atingiu 0% de erro.

---

## 1. Princípios invioláveis (herdados e reforçados)

Toda task desta spec respeita, sem exceção:

1. **Observa. Nunca atua.** Nenhuma chamada a `/metrics` modifica estado do engine. É leitura HTTP GET pura. Zero throttle, zero sinal, zero escrita.
2. **Não quebrar o que funciona.** O fluxo atual (RAPL + NVML + eBPF + gRPC + agregação por PID) permanece intacto. Esta spec adiciona uma fonte de tokens; não reescreve o pipeline de energia.
3. **Zero-trust no transporte.** A coleta de `/metrics` assume que o endpoint pode estar comprometido, indisponível ou retornar lixo. Todo input é validado, com timeout, limite de tamanho de resposta e tratamento de erro explícito. Nunca confiar cegamente no corpo da resposta.
4. **Complexidade justificada.** A abstração introduzida (interface `TokenSource`) existe para permitir múltiplas fontes sem `if/else` espalhado. Não se adiciona nenhuma camada além dessa.
5. **Degradação graciosa.** Se `/metrics` está indisponível, o agente não quebra: loga o erro, reporta 0 tokens na janela, e continua a coleta de energia normalmente. Energia sem token é melhor que agente morto.
6. **Testes em tudo.** Cada unidade nova tem teste. O parser de exposição Prometheus, o cálculo de delta entre janelas, o fallback e o tratamento de erro são todos cobertos.

---

## 2. Decisão de design: a interface `TokenSource`

O problema atual é que `accumulatedTokens` (global, alimentado pelo sniffer) está acoplado ao único modo de obtenção que existe. Introduzimos uma abstração mínima:

```go
// agent/tokens/source.go
package tokens

// TokenSource fornece a contagem de tokens gerados acumulada desde o início do processo.
// Implementações devem ser thread-safe e retornar um contador MONOTÔNICO (sempre crescente),
// permitindo ao chamador calcular o delta entre duas janelas de medição.
type TokenSource interface {
	// CumulativeTokens retorna o total de tokens gerados desde o início da observação.
	// O valor é monotônico. Retorna erro se a fonte está indisponível; nesse caso
	// o chamador deve assumir 0 tokens na janela e prosseguir (degradação graciosa).
	CumulativeTokens() (int64, error)

	// Name identifica a fonte para logging e métricas.
	Name() string
}
```

Por que monotônico cumulativo e não delta-por-janela: o `vllm:generation_tokens_total` é um counter Prometheus, que é cumulativo por natureza. O agente calcula `delta = atual - anterior` na janela. Isso elimina race conditions de "zerar contador" (o `atomic.SwapInt64` atual tem uma janela de corrida onde tokens entre o Swap e o próximo ciclo se perdem). O modelo cumulativo é matematicamente mais correto e mais simples.

Duas implementações:

| Implementação | Fonte | Uso | Default |
|---|---|---|---|
| `PrometheusTokenSource` | GET `/metrics` do vLLM, parse de `vllm:generation_tokens_total` | Produção | **Sim** |
| `LogSnifferTokenSource` | Wrap do `startLogSniffer` existente, adaptado para contador cumulativo | Fallback para engines sem `/metrics` | Não (opt-in) |

---

## 3. Arquitetura do fluxo novo

```
┌────────────────────────────────────────────────┐
│  vLLM worker (PID alvo)                         │
│  ┌──────────────────────────────────────────┐  │
│  │  GET /metrics  →  vllm:generation_tokens_total 12345 │
│  └──────────────────┬───────────────────────┘  │
└─────────────────────┼──────────────────────────┘
                      │ HTTP GET (timeout 1s, max 1MB)
              ┌───────▼────────────┐
              │ PrometheusTokenSource │
              │  CumulativeTokens()  │  → 12345
              └───────┬────────────┘
                      │
         ┌────────────▼─────────────┐
         │ collectAndEnqueue()      │
         │  delta = atual - anterior │  → tokens na janela
         │  (substitui SwapInt64)    │
         └────────────┬─────────────┘
                      │ rateio proporcional por PID (já existe)
              ┌───────▼────────┐
              │  EnergyEvent    │
              │  TokensInWindow │
              └─────────────────┘
```

A única mudança no `collectAndEnqueue` é a origem de `tokensInWindow`: deixa de ser `atomic.SwapInt64(&accumulatedTokens, 0)` e passa a ser `current - previous` de uma `TokenSource`. Todo o resto do rateio por PID permanece idêntico.

---

## 4. Especificação de componentes

### 4.1 `agent/tokens/source.go` — NEW
A interface da seção 2. Apenas a definição da interface, sem implementação. ~15 linhas.

### 4.2 `agent/tokens/prometheus_source.go` — NEW

```go
package tokens

// PrometheusTokenSource lê tokens do endpoint /metrics de uma engine compatível
// com exposição Prometheus (vLLM, TGI). Thread-safe.
type PrometheusTokenSource struct {
	endpoint   string        // ex: "http://localhost:8000/metrics"
	metricName string        // ex: "vllm:generation_tokens_total"
	client     *http.Client  // timeout configurado
}
```

Requisitos estritos:
- Construtor `NewPrometheusTokenSource(endpoint, metricName string) *PrometheusTokenSource` com `http.Client{Timeout: 1 * time.Second}`.
- `CumulativeTokens()`:
  - GET no endpoint.
  - **Zero-trust:** limitar corpo da resposta a 1MB via `io.LimitReader` (proteção contra resposta maliciosa/gigante).
  - Status != 200 → retorna erro, não 0 silencioso.
  - Parse: localizar a linha que começa com `metricName` seguido de espaço (ignorar linhas `# HELP` / `# TYPE` e labels se presentes). Somar valores se a métrica tiver múltiplas séries com labels (ex: por modelo).
  - Valor é float no formato Prometheus; converter para int64 truncando (tokens são inteiros).
  - Validar que o valor é >= ao último lido (counter monotônico); se vier menor, o engine reiniciou — logar e tratar como novo baseline (retornar o valor atual, não negativo).
- `Name()` retorna `"prometheus:" + endpoint`.

**Não usar** biblioteca de parsing Prometheus pesada (`prometheus/common/expfmt`) se um parser de linha simples resolve. A métrica alvo é uma linha `nome valor`. Complexidade justificada: parser de ~20 linhas é mais legível e auditável que arrastar uma dependência de parsing completa. Se múltiplas séries com labels forem necessárias, aí sim justifica `expfmt` — decidir no T-02.2 com base no formato real capturado.

### 4.3 `agent/tokens/logsniffer_source.go` — REFACTOR

Adapta o `startLogSniffer` existente para implementar `TokenSource`:
- Mantém a lógica de tail de arquivo já existente e testada.
- O contador interno passa a ser cumulativo (nunca zera); `CumulativeTokens()` retorna o acumulado atual.
- **Remove o dedup por valor numérico** (a causa do bug GT-00b). Como agora é fonte de fallback explícito e cumulativa, o dedup frágil sai. Se um engine emitir a mesma linha duas vezes para o mesmo evento, documenta-se como limitação conhecida do fallback — o caminho correto é `/metrics`.
- `Name()` retorna `"logsniffer:" + path`.

### 4.4 `agent/main.go` — MODIFY (cirúrgico, não reescrever)

Mudanças mínimas e localizadas:
- Novas flags:
  - `-token-source` (string, default `"prometheus"`, valores: `prometheus` | `logsniffer` | `none`)
  - `-metrics-url` (string, default `"http://localhost:8000/metrics"`)
  - `-metrics-name` (string, default `"vllm:generation_tokens_total"`)
  - Manter `-log-file` para o modo `logsniffer` (compatibilidade).
- Env vars correspondentes (`GT_TOKEN_SOURCE`, `GT_METRICS_URL`, `GT_METRICS_NAME`), seguindo o padrão já existente.
- No setup: instanciar a `TokenSource` conforme a flag. Modo `none` → fonte nula que sempre retorna 0 (energia sem token, válido).
- Em `collectAndEnqueue`: substituir `atomic.SwapInt64(&accumulatedTokens, 0)` por:
  ```go
  current, err := tokenSource.CumulativeTokens()
  if err != nil {
      log.Printf("[TOKEN] fonte %s indisponível: %v — janela com 0 tokens", tokenSource.Name(), err)
      current = previousCumulative // delta = 0 nesta janela
  }
  tokensInWindow := current - previousCumulative
  if tokensInWindow < 0 {
      tokensInWindow = 0 // engine reiniciou; protege contra negativo
  }
  previousCumulative = current
  ```
- `previousCumulative` é estado do loop de coleta (variável local ao loop ou campo de struct), inicializado com a primeira leitura no startup para evitar um delta gigante na primeira janela.
- O `accumulatedTokens` global e o `startLogSniffer` direto saem do `main`; sua lógica migra para `LogSnifferTokenSource`.

### 4.5 Compatibilidade retroativa
- O comportamento default muda de "sniffer se `-log-file` setado" para "prometheus". Documentar no README e no CHANGELOG.
- Quem dependia do `-log-file` usa `-token-source logsniffer -log-file /path`. Documentado.
- Nenhuma mudança no proto, no collector, no exporter ou na agregação. O `EnergyEvent.TokensInWindow` continua sendo preenchido — só muda de onde vem o número.

---

## 5. Segurança zero-trust (detalhamento)

A introdução de uma chamada HTTP de saída exige disciplina:

1. **Timeout obrigatório** (1s) — um `/metrics` lento não pode travar o loop de coleta.
2. **Limite de corpo** (`io.LimitReader`, 1MB) — resposta maliciosa não pode estourar memória.
3. **Sem follow de redirect para hosts externos** — `http.Client` com `CheckRedirect` que rejeita redirect para fora do host configurado (previne SSRF se o endpoint for comprometido). O alvo é localhost/rede interna por design.
4. **Sem TLS verify bypass** — se o endpoint for HTTPS, validação de certificado padrão. Nunca `InsecureSkipVerify`.
5. **Parse defensivo** — qualquer linha malformada é ignorada, não causa panic. Valor não-numérico → erro tratado, não crash.
6. **Endpoint configurável, default localhost** — não assume rede aberta.

---

## 6. TASK BUDGET

Convenção: **S** ≤15k tokens, **M** ~40k, **L** ~70k. Total estimado: ~210k.
Ordem topológica obrigatória.

### T-02.1 [S] — Interface `TokenSource`
Criar `agent/tokens/source.go` com a interface da seção 2 e uma implementação `NullTokenSource` (sempre retorna 0, sem erro) para o modo `none`.
- **Aceite:** `go build ./agent/tokens/` compila; `NullTokenSource` satisfaz a interface (teste de asserção de tipo).
- **Não quebra:** nenhum arquivo existente é tocado.

### T-02.2 [M] — Capturar formato real do `/metrics` do vLLM
Antes de escrever o parser, capturar a saída real. Usar o mock do GT-00b estendido para expor `/metrics` no formato Prometheus do vLLM, OU documentar o formato canônico de `vllm:generation_tokens_total` a partir da doc oficial do vLLM.
- **Aceite:** um fixture `agent/tokens/testdata/vllm_metrics.txt` com a saída real/canônica, incluindo linhas `# HELP`, `# TYPE`, e a métrica com e sem labels.
- **Decisão registrada:** parser de linha simples vs `expfmt`, justificada pelo formato observado.

### T-02.3 [M] — `PrometheusTokenSource`
Implementar `agent/tokens/prometheus_source.go` conforme 4.2, respeitando todos os requisitos zero-trust da seção 5.
- **Aceite:** testes cobrindo: (a) parse correto do fixture; (b) soma de múltiplas séries com labels; (c) status != 200 → erro; (d) corpo > 1MB → truncado/erro; (e) valor não-numérico → erro tratado; (f) counter que regride (reinício de engine) → tratado sem negativo; (g) timeout → erro. Todos via `httptest.Server`, sem rede real.

### T-02.4 [M] — Refatorar log sniffer para `TokenSource`
Migrar `startLogSniffer` para `agent/tokens/logsniffer_source.go` implementando `TokenSource` cumulativo. **Remover o dedup por valor.**
- **Aceite:** o teste de regressão do GT-00b (carga concorrente, tokens idênticos) agora conta corretamente — sem subcontagem, porque o dedup frágil saiu e o contador é cumulativo. Teste reproduz o cenário de rajada e confirma contagem exata.
- **Não quebra:** a lógica de tail de arquivo existente é preservada.

### T-02.5 [M] — Integrar no `agent/main.go`
Aplicar as mudanças cirúrgicas de 4.4: flags, env, instanciação da fonte, substituição do `SwapInt64` pelo modelo delta-cumulativo.
- **Aceite:** `go build ./agent` limpo; agente sobe com `-token-source prometheus` e com `-token-source logsniffer`; modo `none` reporta energia com 0 tokens sem erro. Os testes existentes do `agent` continuam verdes (não-regressão).

### T-02.6 [S] — Teste de integração delta-cumulativo
Teste que simula duas janelas consecutivas com uma `TokenSource` mock retornando valores cumulativos crescentes (ex: 100 → 250), confirmando que o delta da janela é 150 e que `previousCumulative` avança corretamente. Incluir caso de engine reiniciado (250 → 30 → delta tratado como 0 ou novo baseline).
- **Aceite:** teste verde cobrindo janela normal, primeira janela (sem delta gigante) e reinício de engine.

### T-02.7 [S] — Documentação e CHANGELOG
Atualizar README: nova seção "Token Sources" explicando `prometheus` (default, fonte de verdade), `logsniffer` (fallback best-effort, limitação conhecida) e `none`. Documentar as flags e env vars. Registrar no CHANGELOG a mudança de default e o racional (GT-00b).
- **Aceite:** README cobre as três fontes, o quando-usar-qual, e a limitação do fallback.

### Gate de validação final do GT-02

- [ ] `grep -rn "bpf_send_signal\|log.Fatal\|InsecureSkipVerify" agent/` retorna vazio.
- [ ] `go test ./agent/... ./collector/... ./pb/... ./cmd/...` verde nos quatro módulos.
- [ ] Teste de regressão GT-00b: carga concorrente de N requests × max_tokens fixo → erro de contagem < 2% via `PrometheusTokenSource`.
- [ ] Modo `none` e fonte indisponível não quebram o agente (energia continua fluindo).
- [ ] Nenhuma mudança no proto/collector/exporter/agregação (diff restrito a `agent/`).
- [ ] `previousCumulative` inicializado no startup (sem delta gigante na primeira janela).

---

## 7. Validação de produção (após o gate)

Ambiente real (RunPod RTX 3090/4090 por centavos, ou vLLM CPU local se disco permitir):
1. Sobe vLLM real com `/metrics` exposto.
2. Agente com `-token-source prometheus -metrics-url http://localhost:8000/metrics -pid <vllm_pid>`.
3. Carga concorrente conhecida: N requests × `max_tokens` fixo, alta concorrência (reproduz a rajada que quebrou o GT-00b).
4. Comparar: soma de `completion_tokens` da API vs `vllm:generation_tokens_total` lido pelo agente.
- **GO de produção:** erro < 2%. Aí o H3 vale para produção e o GreenToken está validado ponta a ponta com a fonte de verdade.

---

## 8. Riscos e decisões em aberto (honestidade adversarial)

1. **`vllm:generation_tokens_total` conta tokens de geração, não de prompt.** Isso é o correto para custo de inferência (você paga energia por token gerado), mas confirmar que é a métrica certa para a tese de custo. Se o produto quiser também custo de prompt processing, adicionar `vllm:prompt_tokens_total` numa spec futura.

2. **Atribuição multi-modelo no mesmo endpoint.** Se um vLLM serve múltiplos modelos, `/metrics` pode ter séries com label `model`. O parser soma todas por padrão. Para custo por-modelo, usar o label — decisão para GT-03, não bloqueia GT-02.

3. **O log sniffer fallback fica com limitação conhecida documentada**, não resolvida. É consciente: investir em robustez de parsing de stdout é otimizar o caminho errado. `/metrics` é a resposta.

4. **TGI e outros engines** expõem `/metrics` com nomes diferentes (`tgi_request_generated_tokens`). A flag `-metrics-name` cobre isso sem código novo. Documentar os nomes conhecidos no README.
