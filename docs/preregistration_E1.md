# Pré-Registro Científico — Experimento E1 (GT-M)

**Título:** Decomposição Energética Intra-Inferência de IA e Validação de Reconstrução por Ensemble  
**Data de Pré-Registro:** 2026-07-29  
**Status:** PRÉ-REGISTRADO (Commitado antes da primeira coleta de dados)  
**Especificação-Mãe:** `docs/SPEC_GTM_MEDICAO_INDEPENDENTE.md`  
**Autores:** Rodrigo Freire (Condução) & Antigravity (Tooling & Automação)  

---

## 1. Declaração de Hipótese Pré-Registrada

> **Hipótese $H_{E1}$:**  
> *"Em inferência de LLM no ambiente de teste (CPU Host Intel / DRAM com aceleração de kernel eBPF e engine de inferência local), a fração de energia do sistema atribuível ao cálculo numérico puro (multiplicação de matrizes nas fases de Prefill e Decode) será de **45% ± 10%**, com o restante (**55% ± 10%**) distribuído entre movimentação de dados na memória (DRAM / cache misses), conversão/marshalling de entrada/saída e overhead de escalonamento do sistema operacional."*

### Justificativa da Estimativa Prévia:
A arquitetura Von Neumann impõe uma barreira severa de movimentação de pesos do modelo entre a memória principal (DRAM) e os registradores da CPU/GPU. Hipotetizamos que a transferência contínua de parâmetros a cada token na fase de Decode consome uma parcela desproporcional da energia total do sistema, superando o custo energético das operações de ponto flutuante (FLOPs) em si.

---

## 2. Metodologia de Reconstrução por Ensemble

Dado que o eBPF `sched_switch` fornece resolução temporal de **microssegundos** ($\mu s$), enquanto os sensores de energia física (Intel/AMD RAPL e NVIDIA NVML) possuem janelas de atualização de **1 ms a 50 ms**, a decomposição energética de uma única inferência é fisicamente irresolvível em amostragem direta de uma única execução.

Adotamos a **Reconstrução por Ensemble de Fases**:
1. Execução de $N \ge 30$ inferências estritamente idênticas (mesmo prompt, `temperature=0`, `max_tokens` fixo, mesmo estado térmico).
2. Marcação de timestamps de alta precisão via eBPF / `/metrics` para quatro janelas de fase:
   - **Fase 0 ($F_0$):** Carga de dados, parsing e orquestração do evento HTTP/gRPC.
   - **Fase 1 ($F_1$):** Processamento do Prompt (Prefill / Context Evaluation).
   - **Fase 2 ($F_2$):** Geração autoregressiva de tokens (Decode).
   - **Fase 3 ($F_3$):** Desalocação, retorno do payload e encerramento do ciclo.
3. Superposição alinhada no tempo e integração estatística do consumo de potência ($\Delta \text{Watts} = P_{\text{total}} - P_{\text{idle}}$) sobre as janelas temporais de cada fase.

---

## 3. Critérios de Sucesso e Gates Invioláveis (Gate E1 → E2)

O experimento $E1$ será considerado **VALIDADO** e apto a desbloquear o experimento $E2$ se, e somente se, todos os 5 critérios a seguir forem satisfeitos simultaneamente:

| Código | Critério / Regra | Limiar Numérico / Tolerância | Ação em Caso de Falha |
|---|---|---|---|
| **G1.1** | **Consistência Interna de Energia** | $|E_{\text{total}} - \sum_{i=0}^{3} E_{F_i}| / E_{\text{total}} \le 10\%$ | **FALHA METODOLÓGICA:** O experimento é descartado e a instrumentação deve ser revisada. As partes **devem** somar o todo. |
| **G1.2** | **Repetibilidade do Total (CV)** | $CV = \frac{\sigma_{E_{\text{total}}}{\mu_{E_{\text{total}}}} \le 15\%$ | **RUÍDO EXCESSIVO:** Investigar processos concorrentes no host ou ruído térmico. Reduzir interferência. |
| **G1.3** | **Estabilidade do Baseline (C1)** | $|P_{\text{idle, pós}} - P_{\text{idle, pré}}| / P_{\text{idle, pré}} \le 5\%$ | **DERIVA TÉRMICA/SISTEMA:** A série de 30 execuções é descartada integralmente. |
| **G1.4** | **Estabilidade Térmica (C2)** | $\Delta T_{\text{silício}} < 2^\circ\text{C / min}$ durante a amostragem | Aguardar estabilização do aquecimento prévio antes de iniciar. |
| **G1.5** | **Resolução de Fases** | $\Delta t_{\text{fase}} > \Delta t_{\text{amostragem\_sensor}}$ | Fases com duração menor que a resolução do sensor devem ser explicitamente declaradas como **não-resolvíveis**. |

---

## 4. Plano de Execução do Tooling de Amostragem

1. **`cmd/gtm-collector` / `scripts/e1_ensemble_collector.py`**:
   - Mede 60 segundos de baseline ocioso $P_{\text{idle, pré}}$.
   - Aquecimento térmico de 2 minutos sob carga sintética constante.
   - Disparo de 30 requisições idênticas com salvamento de traces eBPF e logs brutos de potência (RAPL/NVML).
   - Mede 60 segundos de baseline ocioso final $P_{\text{idle, pós}}$.
2. **Processamento Estatístico**:
   - Cálculo de desvio padrão ($\sigma$), coeficiente de variação ($CV$), intervalo de confiança de 95% ($CI_{95\%}$) e verificação de conservação de energia.
3. **Publicação Bruta**:
   - Salvamento dos dados brutos em `docs/artifacts/E1_raw_data.json` sem qualquer filtragem manual não-justificada antes da pré-análise.

---

## 5. Matriz de Declaração de Limitações Prévia

- **Sensores virtuais/WSL2:** Se o ambiente WSL2 não expuser o contador MSR `/sys/class/powercap/intel-rapl` devido a restrições de hipervisor, o agente fará o fallback declarado para estimativa por ciclos de CPU e a limitação será registrada com destaque no relatório final.
- **Granularidade do sensor GPU:** O NVML em GPUs comerciais pode apresentar taxa de amostragem limitada a 10 Hz–100 Hz (10 ms a 100 ms), o que impede a medição direta de tokens individuais de Decode ultra-rápidos (< 10 ms). Essa limitação é conhecida e será superada pela média de ensemble de 30+ repetições.
