# Relatório Científico — Experimento E3 (GT-M)

**Título:** Custo Energético da Deriva e Amortização por Recalibração sob Variabilidade de Prompt e Temperatura  
**Data:** 2026-07-29  
**Status:** PENDENTE DE EXECUÇÃO NO KAGGLE (Aguardando dados empíricos)  
**Especificação:** `docs/SPEC_GTM_MEDICAO_INDEPENDENTE.md`  
**Pré-Registro:** `docs/experiments/E3_Custo_Deriva/preregistration.md`  
**Código do Coletor:** `docs/experiments/E3_Custo_Deriva/e3_drift_cost_collector.py`  
**Hardware de Teste Físico:** NVIDIA Tesla T4 (Ambiente Kaggle Linux / NVML Ativo / Multiprocessing Isolation)  
**Dado Bruto:** `docs/experiments/E3_Custo_Deriva/artifacts/E3_raw_data.json`  

---

## 1. Declaração Metodológica e Natureza Híbrida (Conforme Seção 3 do SPEC GT-M)

> **Declaração de Transparência:** O Experimento E3 combina a medição empírica física no silício (NVIDIA Tesla T4) para aferir a energia da inferência sob diferentes comprimentos de prompt (128, 512, 1024 tokens) e temperaturas (0.0, 0.7, 1.0) com a simulação matemática do modelo de retenção de pesos $w(t) = w_0 \cdot t^{-\nu}$ e a amortização energética do custo de recalibração temporal.

---

## 2. Resultados Empíricos no Silício (Parte A)

*A ser preenchido automaticamente após a execução no Kaggle.*

| Condição de Teste | Tamanho do Prompt | Temperatura ($T$) | Energia Média ($E_{\text{net}}$) | Dispersão ($CV$) | Status |
|---|---|---|---|---|---|
| **Prompt_Short_128t_T0.0** | 128 tokens | 0.0 | `Pending Joules` | `Pending %` | **Aguardando** |
| **Prompt_Med_512t_T0.7** | 512 tokens | 0.7 | `Pending Joules` | `Pending %` | **Aguardando** |
| **Prompt_Long_1024t_T1.0** | 1024 tokens | 1.0 | `Pending Joules` | `Pending %` | **Aguardando** |

---

## 3. Simulação Matemática do Custo Amortizado de Deriva (Parte B)

*A ser preenchido com base na energia $E_{\text{net}}$ medida.*

- **Modelo de Retenção:** $w(t) = w_0 \cdot t^{-0.02}$
- **Energia Base de Inferência ($E_{\text{base}}$):** `Pending J`
- **Recalibrações Necessárias na Vida Útil:** `Pending`
- **Energia Amortizada Real por Inferência ($E_{\text{amortized}}$):** `Pending J` (+`Pending %` de overhead de manutenção)

---

## 4. Status dos Gates de Transição (Gate E3 → E4)

- [ ] **Gate G3.1 (Variabilidade CV em Todas as Condições):** $CV \le 15\%$
- [ ] **Gate G3.2 (Estabilidade de Baseline C1):** Deriva de Baseline $\le 5.0\%$
- [ ] **Gate G3.3 (Análise de Sensibilidade Declarada):** Intervalos $CI_{95\%}$ computados
- [ ] **Gate G3.4 (Declaração de Transparência Híbrida):** Natureza empírica vs simulada declarada sem ocultação
