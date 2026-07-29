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

---

## 2. Metodologia de Reconstrução por Ensemble

Dado que o eBPF `sched_switch` fornece resolução temporal de **microssegundos** ($\mu s$), enquanto os sensores de energia física (Intel/AMD RAPL e NVIDIA NVML) possuem janelas de atualização de **1 ms a 50 ms**, a decomposição energética de uma única inferência é fisicamente irresolvível em amostragem direta de uma única execução.

Adotamos a **Reconstrução por Ensemble de Fases**:
1. Execução de $N \ge 30$ inferências estritamente idênticas.
2. Marcação de timestamps de alta precisão via eBPF / `/metrics` para quatro janelas de fase:
   - **Fase 0 ($F_0$):** Carga de dados, parsing e orquestração do evento HTTP/gRPC.
   - **Fase 1 ($F_1$):** Processamento do Prompt (Prefill / Context Evaluation).
   - **Fase 2 ($F_2$):** Geração autoregressiva de tokens (Decode).
   - **Fase 3 ($F_3$):** Desalocação, retorno do payload e encerramento do ciclo.
3. Superposição alinhada no tempo e integração estatística contínua da potência ($\Delta \text{Watts} = P_{\text{total}} - P_{\text{idle}}$) sobre as janelas temporais de cada fase.

---

## 3. Critérios de Sucesso e Gates Invioláveis (Gate E1 → E2)

| Código | Critério / Regra | Limiar Numérico / Tolerância | Ação em Caso de Falha |
|---|---|---|---|
| **G1.1** | **Consistência Interna de Energia** | $|E_{\text{total}} - \sum_{i=0}^{3} E_{F_i}| / E_{\text{total}} \le 10\%$ | **FALHA METODOLÓGICA:** O experimento é descartado. |
| **G1.2** | **Repetibilidade do Total (CV)** | $CV = \frac{\sigma_{E_{\text{total}}}}{\mu_{E_{\text{total}}}} \le 15\%$ | **RUÍDO EXCESSIVO:** Ajustar estabilidade de amostragem. |
| **G1.3** | **Estabilidade do Baseline (C1)** | $|P_{\text{idle, pós}} - P_{\text{idle, pré}}| / P_{\text{idle, pré}} \le 5\%$ | **DERIVA TÉRMICA:** Descartar a série. |
| **G1.4** | **Estabilidade Térmica (C2)** | $\Delta T_{\text{silício}} < 2^\circ\text{C / min}$ | Aquecimento prévio por 30s. |
| **G1.5** | **Resolução de Fases** | Integração em alta frequência ($10\text{ ms}$) | Amostragem contínua via `ContinuousNVMLSampler`. |
