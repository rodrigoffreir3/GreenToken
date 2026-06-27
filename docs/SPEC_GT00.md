# SPEC GT-00 — T-Spike: Validação Numérica da Correlação PID→Watts→Tokens

**Versão:** 1.0
**Tipo:** Spike de validação (descartável — código não vai pra produção)
**Pré-requisito de:** GT-01 Fase 2 (NVML). NÃO comece a Fase 2 sem passar este gate.
**Duração alvo:** 1 sessão de ~3–4h numa instância GPU alugada.
**Custo estimado:** US$ 2–6 (instância spot/sob demanda por poucas horas).
**Executor:** Rodrigo manual + Antigravity para os scripts de coleta.

---

## 0. Por que este spike existe

A tese inteira do GreenToken depende de uma única afirmação ainda não provada:

> É possível, num host de inferência real, atribuir o consumo de energia da GPU a um PID específico e correlacionar isso com os tokens que esse PID gerou, com erro aceitável.

Se isso não fechar numericamente, não existe produto — você estaria medindo energia agregada do host (que qualquer ferramenta já faz) sem o link com o workload. Este spike custa ~US$ 5 e umas horas. A Fase 2 custa ~120k tokens de squad e semanas. Errar a ordem é caro.

**Decisão de saída:** ao fim do spike, você tem um GO ou NO-GO objetivo, baseado em três números (seção 4).

---

## 1. Hipóteses a falsificar

| # | Hipótese | Como falsifica |
|---|---|---|
| H1 | `nvmlDeviceGetComputeRunningProcesses` retorna o PID do worker de inferência (não só "algum processo CUDA"). | Subir vllm, ler a lista, conferir se o PID bate com `pgrep`. |
| H2 | `nvmlDeviceGetPowerUsage` sobe mensuravelmente durante geração e cai em idle. | Medir watts em idle vs sob carga; delta tem que ser claro (>30W típico em H100/A100). |
| H3 | O número de tokens gerados é obtível do vllm com precisão (não estimativa). | Comparar contagem do `/metrics` do vllm com a resposta da API. |
| H4 | `custo_por_token` calculado é estável entre runs idênticos (variância < 15%). | Rodar o mesmo prompt 5×, comparar o custo/token. |

Se H1 ou H3 falham → o produto não é viável como descrito (sem link PID↔token).
Se H2 falha → você está numa GPU que não expõe power telemetry (raro, mas checar).
Se H4 falha com variância alta → a métrica existe mas é ruidosa demais pra vender; vira problema de agregação.

---

## 2. Ambiente mínimo

- **Instância:** 1× GPU NVIDIA com NVML (A100, A10, L4, ou até T4 serve pro spike). Provedores baratos: RunPod, Vast.ai, Lambda, ou GPU spot na sua Magalu Cloud se tiver. T4/L4 é suficiente e mais barato — não precisa de H100 pra validar a mecânica.
- **Driver:** NVIDIA driver + `nvidia-smi` funcional. NVML vem junto.
- **Engine de inferência:** vllm (tem `/metrics` Prometheus nativo — é o que torna H3 testável sem hack). Modelo pequeno serve: `facebook/opt-1.3b` ou `Qwen2.5-0.5B`. Não precisa de modelo grande pro spike.
- **Acesso:** RAPL (`/sys/class/powercap/intel-rapl`) pode não existir em VM cloud — tudo bem, o spike foca na GPU, que é onde está a dúvida real. CPU/DRAM você já provou localmente.

---

## 3. Procedimento (4 medições)

### Medição A — Baseline idle (10 min)
1. Subir a instância, instalar driver + vllm + modelo. NÃO mandar requests.
2. Coletar a cada 1s por 60s: `nvmlDeviceGetPowerUsage`, lista de `ComputeRunningProcesses`.
3. Registrar: watts médio idle, e se algum PID aparece sem carga.

### Medição B — Identificação de PID (H1)
1. Subir o vllm serve. Anotar PID via `pgrep -f vllm`.
2. Ler `nvmlDeviceGetComputeRunningProcesses`.
3. **Aceite H1:** o PID do vllm aparece na lista NVML.

### Medição C — Carga controlada (H2 + H3)
1. Enviar um lote conhecido: 100 requests, cada um pedindo exatamente `max_tokens=200` (total esperado ≈ 20.000 tokens de saída).
2. Durante a carga, coletar watts da GPU a cada 200ms.
3. Ao fim, ler `vllm:generation_tokens_total` do `/metrics`.
4. **Aceite H2:** watts sob carga − watts idle > 30W (ou claramente acima do ruído).
5. **Aceite H3:** tokens do `/metrics` batem com 20.000 ± 2% (margem pra tokens de EOS/padding).

### Medição D — Estabilidade (H4)
1. Repetir a Medição C exatamente 5×.
2. Para cada run, calcular:
   ```
   energia_joules = média_watts_carga × duração_segundos
   custo_por_token = (energia_joules / 3.6e6) × preço_kWh / tokens_gerados
   ```
   (3.6e6 J = 1 kWh)
3. **Aceite H4:** desvio padrão de `custo_por_token` / média < 15%.

---

## 4. Critério de decisão GO / NO-GO

Preencher esta tabela ao fim do spike:

| Número | Medido | Threshold | Passou? |
|---|---|---|---|
| PID do worker visível no NVML (H1) | sim/não | tem que ser SIM | |
| Delta watts carga−idle (H2) | ___ W | > 30 W | |
| Erro de contagem de tokens (H3) | ___ % | < 2% | |
| Variância de custo/token entre runs (H4) | ___ % | < 15% | |

**GO** (segue para GT-01 Fase 2) se: H1=sim **E** as outras três dentro do threshold.

**GO CONDICIONAL** se H1, H2, H3 passam mas H4 entre 15–30%: o produto existe mas precisa de janela de agregação maior pra suavizar ruído. Anota como requisito pro agregador (GT-01 T5.1) e segue.

**NO-GO** se H1=não **OU** H3 > 2%: sem link confiável PID↔token, a tese cai. Pivota para o Caminho B (eficiência em CPU/edge, onde RAPL é o gargalo e você não depende de NVML) ou repensa o ângulo.

---

## 5. Entregável do spike

Um único arquivo `spike_results.md` com:
- A tabela da seção 4 preenchida.
- Os 5 valores de custo/token da Medição D.
- Print/log do `nvmlDeviceGetComputeRunningProcesses` mostrando o PID.
- Uma frase de decisão: "GO" / "GO CONDICIONAL" / "NO-GO" + justificativa em 2 linhas.

Não precisa de código bonito. Scripts Python jogados com `pynvml` + `requests` resolvem. O Antigravity pode gerar os dois scripts (coletor de watts e disparador de carga) em ~20k tokens. O objetivo é o número, não a engenharia.

---

## 6. Nota honesta

Este spike pode matar o projeto numa tarde. Isso é o ponto — é exatamente o tipo de validação barata que separa "tive uma ideia" de "construí um negócio". Se passar, você entra na Fase 2 com convicção e um número real pra mostrar num pitch ("medimos custo/token com X% de precisão"). Se falhar, você gastou US$ 5 em vez de 6 semanas. Os dois desfechos são vitórias.
