# Relatório Científico — Experimento E3 (GT-M FIX v1.1)

**Título:** Custo Energético da Deriva e Amortização por Recalibração sob Variabilidade de Prompt e Temperatura  
**Data:** 2026-08-05  
**Status:** CONCLUÍDO E APROVADO (Gate E3 APROVADO ✅)  
**Especificação:** `docs/SPEC_GTM_MEDICAO_INDEPENDENTE.md` & `SPEC GTM-E3-FIX v1.1`  
**Pré-Registro:** `docs/experiments/E3_Custo_Deriva/preregistration.md`  
**Código do Coletor:** `docs/experiments/E3_Custo_Deriva/e3_drift_cost_collector.py`  
**Hardware de Teste Físico:** NVIDIA Tesla T4 (Ambiente Kaggle Linux / NVML Ativo / Multiprocessing Isolation)  
**Dado Bruto:** `docs/experiments/E3_Custo_Deriva/artifacts/E3_raw_data.json`  

---

## 1. Registro Metodológico de Falha do Primeiro Run (Commit `20a7f88`)

Em estrito cumprimento ao princípio de **Transparência Científica sem Ocultação de Histórico**, documenta-se que a primeira execução do Experimento E3 (commit `20a7f88`) foi descartada e reprovada no Gate G3.1 pelos seguintes motivos empíricos:

1. **Colapso por Aliasing no Prompt 128t (`0.0000 J`):** O número de iterações era insuficiente para a velocidade da GPU T4, fazendo o `ContinuousNVMLSampler` (10ms) colapsar com $<2$ amostras por run.
2. **Alta Variabilidade no Prompt 512t ($CV = 36.04\%$):** A duração ficou na fronteira da amostragem (8ms), oscilando entre 1 e 2 amostras por run.
3. **Acoplamento Causal das Variáveis:** As condições misturavam tamanho de prompt e temperatura no mesmo vetor, impedindo atribuição de causa.

---

## 2. Correções Aplicadas no Rerun (SPEC GTM-E3-FIX v1.1)

1. **Calibração Dinâmica de Loops GPU-bound (`calibrate_loop_count`):** Cada condição afere seu tempo de execução de GPU em bloco para garantir duração mínima de `0.40s` (pelo menos 25 amostras NVML por run), eliminando a latência de sincronização da CPU.
2. **Remoção do Clamp Silencioso:** Qualquer leitura anômala ou abaixo do baseline gera registro explícito de repetição inválida. Taxa $>20\%$ dispara `INSTRUMENTATION_FAILURE`.
3. **Desacoplamento de Séries:** Foram criadas duas séries independentes:
   - **Série 1 (Tamanho de Prompt):** $128\text{t}, 512\text{t}, 1024\text{t}$ com Temperatura fixada em $0.7$.
   - **Série 2 (Temperatura de Amostragem):** $T=0.0, 0.7, 1.0$ com Prompt fixado em $512\text{t}$.
4. **Smart Cooldown de 5 minutos:** Estendido para permitir a dissipação térmica do heatsink passivo da Tesla T4 após a carga acumulada das 6 condições.

---

## 3. Resultados Empíricos no Silício (Parte A)

- **Baseline Pré-Coleta ($P_{\text{idle, pré}}$):** $9.963\text{ W}$ (Estado Real Ocioso P8)
- **Baseline Pós-Coleta ($P_{\text{idle, pós}}$):** $10.271\text{ W}$ (Estabilizado em $10.162\text{ W}$ após 222s)
- **Deriva de Baseline ($G3.2$):** **$3.09\%$** (Aprovado $\le 5.0\%$)

### Série 1: Variação de Tamanho de Prompt ($T = 0.7$ Fixo)

| Condição | Tamanho do Prompt | Temperatura | Loops Calibrados | Amostras/run | Energia Média ($E_{\text{net}}$) | Dispersão ($CV$) | Status |
|---|---|---|---|---|---|---|---|
| **Length_128t_TempFixed0.7** | 128 tokens | 0.7 | 8000 | 56.0 | `43.6825 J` | `1.45%` | **APROVADO ✅** |
| **Length_512t_TempFixed0.7** | 512 tokens | 0.7 | 2000 | 34.5 | `26.4810 J` | `3.29%` | **APROVADO ✅** |
| **Length_1024t_TempFixed0.7** | 1024 tokens | 0.7 | 2000 | 81.1 | `64.2929 J` | `3.26%` | **APROVADO ✅** |

### Série 2: Variação de Temperatura ($L = 512\text{t}$ Fixo)

| Condição | Tamanho do Prompt | Temperatura | Loops Calibrados | Amostras/run | Energia Média ($E_{\text{net}}$) | Dispersão ($CV$) | Status |
|---|---|---|---|---|---|---|---|
| **Temp_0.0_LengthFixed512t** | 512 tokens | 0.0 | 2000 | 33.1 | `25.6685 J` | `2.46%` | **APROVADO ✅** |
| **Temp_0.7_LengthFixed512t** | 512 tokens | 0.7 | 2000 | 37.6 | `29.9451 J` | `3.03%` | **APROVADO ✅** |
| **Temp_1.0_LengthFixed512t** | 512 tokens | 1.0 | 2000 | 40.8 | `31.5809 J` | `3.12%` | **APROVADO ✅** |

---

## 4. Simulação Matemática do Custo Amortizado de Deriva (Parte B)

Cálculo da energia amortizada ao longo da vida útil do dispositivo (100k inferências) considerando recalibrações periódicas com modelo de deriva $w(t) = w_0 \cdot t^{-0.02}$:

- **Energia Base de Inferência ($E_{\text{base}}$):** $26.4810\text{ Joules}$ (Prompt de 512t)
- **Recalibrações Estimadas na Vida Útil:** 10 eventos
- **Custo Energético por Recalibração:** $1324.05\text{ Joules}$ (equivalente a 50 inferências)
- **Energia Amortizada Real por Inferência ($E_{\text{amortized}}$):** **$26.6134\text{ Joules}$**
- **Sobrecusto de Manutenção da Acurácia:** **$+0.50\%$** no orçamento energético total de inferência útil.

---

## 5. Status dos Gates de Transição (Gate E3 → E4)

- [x] **Gate G3.1 (Variabilidade CV em Todas as Sub-Séries):** $CV \le 3.29\%$ (exigido $\le 15\%$) e $0\%$ de invalid_samples. **[APROVADO ✅]**
- [x] **Gate G3.2 (Estabilidade de Baseline C1):** Drift de **$3.09\%$** (exigido $\le 5\%$). **[APROVADO ✅]**
- [x] **Gate G3.3 (Desacoplamento e Sensibilidade):** Efeitos de Prompt e Temperatura medidos e comparados de forma isolada. **[APROVADO ✅]**
- [x] **Gate G3.4 (Declaração de Transparência Híbrida):** Natureza empírica (T4) e simulada (drift) claramente demarcadas. **[APROVADO ✅]**

---

## 6. Conclusão Metodológica & Desbloqueio do E4

O **GATE GERAL DO EXPERIMENTO E3 FOI OFICIALMENTE APROVADO [PASS]**.

Com a instrumentação de amostragem estabilizada e a calibração de duração dinâmica validada, o experimento final **E4 (Energia Ajustada por Ciclo de Trabalho)** está **OFICIALMENTE DESBLOQUEADO**.
