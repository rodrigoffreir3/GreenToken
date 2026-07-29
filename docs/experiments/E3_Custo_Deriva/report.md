# Relatório Científico — Experimento E3 (GT-M FIX v1.0)

**Título:** Custo Energético da Deriva e Amortização por Recalibração sob Variabilidade de Prompt e Temperatura  
**Data:** 2026-07-29  
**Status:** PENDENTE DE EXECUÇÃO NO KAGGLE (Rerun sob SPEC GTM-E3-FIX v1.0)  
**Especificação:** `docs/SPEC_GTM_MEDICAO_INDEPENDENTE.md` & `SPEC GTM-E3-FIX v1.0`  
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

## 2. Correções Aplicadas no Rerun (SPEC GTM-E3-FIX v1.0)

1. **Calibração Dinâmica de Loops (`calibrate_loop_count`):** Cada condição afere seu tempo de execução antes do teste para garantir duração mínima de `0.35s` ($\ge 25$ amostras NVML por run).
2. **Remoção do Clamp Silencioso:** A linha `max(0.0, ...)` foi eliminada. Amostragens insuficientes ou abaixo do baseline geram registro explícito de repetição inválida. Taxa $>20\%$ dispara `INSTRUMENTATION_FAILURE`.
3. **Desacoplamento de Séries:** Foram criadas duas séries independentes:
   - **Série 1 (Tamanho de Prompt):** $128\text{t}, 512\text{t}, 1024\text{t}$ com Temperatura fixada em $0.7$.
   - **Série 2 (Temperatura de Amostragem):** $T=0.0, 0.7, 1.0$ com Prompt fixado em $512\text{t}$.

---

## 3. Resultados Empíricos no Silício (Parte A - A ser preenchido após rerun)

### Série 1: Variação de Tamanho de Prompt ($T = 0.7$ Fixo)

| Condição | Tamanho do Prompt | Temperatura | Loops Calibrados | Amostras/run | Energia Média ($E_{\text{net}}$) | Dispersão ($CV$) | Status |
|---|---|---|---|---|---|---|---|
| **Length_128t_TempFixed0.7** | 128 tokens | 0.7 | `Pending` | `Pending` | `Pending J` | `Pending %` | **Aguardando** |
| **Length_512t_TempFixed0.7** | 512 tokens | 0.7 | `Pending` | `Pending` | `Pending J` | `Pending %` | **Aguardando** |
| **Length_1024t_TempFixed0.7** | 1024 tokens | 0.7 | `Pending` | `Pending` | `Pending J` | `Pending %` | **Aguardando** |

### Série 2: Variação de Temperatura ($L = 512\text{t}$ Fixo)

| Condição | Tamanho do Prompt | Temperatura | Loops Calibrados | Amostras/run | Energia Média ($E_{\text{net}}$) | Dispersão ($CV$) | Status |
|---|---|---|---|---|---|---|---|
| **Temp_0.0_LengthFixed512t** | 512 tokens | 0.0 | `Pending` | `Pending` | `Pending J` | `Pending %` | **Aguardando** |
| **Temp_0.7_LengthFixed512t** | 512 tokens | 0.7 | `Pending` | `Pending` | `Pending J` | `Pending %` | **Aguardando** |
| **Temp_1.0_LengthFixed512t** | 512 tokens | 1.0 | `Pending` | `Pending` | `Pending J` | `Pending %` | **Aguardando** |

---

## 4. Simulação Matemática do Custo Amortizado de Deriva (Parte B)

*A ser preenchido após a execução do rerun.*

---

## 5. Status dos Gates de Transição (Gate E3 → E4)

- [ ] **Gate G3.1 (Variabilidade CV em Todas as Sub-Séries):** $CV \le 15\%$ e taxa de invalid_samples $\le 20\%$
- [ ] **Gate G3.2 (Estabilidade de Baseline C1):** Deriva de Baseline $\le 5.0\%$
- [ ] **Gate G3.3 (Desacoplamento e Sensibilidade):** Sub-séries reportadas de forma independente
- [ ] **Gate G3.4 (Declaração de Transparência Híbrida):** Natureza empírica vs simulada declarada sem ocultação
