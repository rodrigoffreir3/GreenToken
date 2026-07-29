# Pré-Registro Científico — Experimento E3 (GT-M)

**Título:** Custo Energético da Deriva e Amortização por Recalibração sob Variabilidade de Prompt e Temperatura  
**Data de Pré-Registro:** 2026-07-29  
**Status:** PRÉ-REGISTRADO (Commitado antes da primeira execução do E3)  
**Especificação-Mãe:** `docs/SPEC_GTM_MEDICAO_INDEPENDENTE.md`  
**Dependência:** Experimento E2 CONCLUÍDO E APROVADO (Gate E2 ✅)  
**Autores:** Rodrigo Freire (Condução) & Antigravity (Tooling & Automação)  

---

## 1. Declaração de Hipótese Pré-Registrada $H_{E3}$ e Transparência Metodológica

> **Aviso Metodológico Fundamental (Conforme Seção 3 do SPEC GT-M):**  
> *"O Experimento E3 possui natureza híbrida: combina a **medição física empírica** do custo energético de inferência em hardware real (NVIDIA Tesla T4) sob variação de tamanho de contexto/prompt e temperatura com a **modelagem matemática e simulação** da retenção de pesos e custo de recalibração temporal ($w(t) = w_0 \cdot t^{-\nu}$). Esta mudança de natureza é declarada com destaque para garantir 100% de rigor científico e transparência."*

> **Hipótese $H_{E3}$:**  
> *"Ao variar o tamanho do prompt de entrada (Curto: 128 tokens, Médio: 512 tokens, Longo: 1024 tokens) e a temperatura de amostragem ($T \in \{0.0, 0.7, 1.0\}$), a energia bruta consumida escala de forma não linear com o comprimento do contexto devido à fase de prefill, enquanto a energia amortizada por inferência ao longo da vida útil do nó aumenta em **30% ± 10%** quando considerada a necessidade de recalibração/re-gravação periódica para mitigar a deriva de acurácia."*

---

## 2. Metodologia de Medição e Simulação Híbrida

1. **Parte A — Coleta Empírica no Silício (NVIDIA Tesla T4):**
   - **Variação de Tamanho de Prompt / Contexto:** 
     - **Curto ($L_{128}$):** 128 tokens de contexto.
     - **Médio ($L_{512}$):** 512 tokens de contexto.
     - **Longo ($L_{1024}$):** 1024 tokens de contexto.
   - **Variação de Temperatura ($T$):** $T = 0.0$ (determinístico), $T = 0.7$ (amostragem padrão), $T = 1.0$ (alta estocasticidade).
   - **Medição Contínua:** Uso do `ContinuousNVMLSampler` e isolamento via `multiprocessing.Process` (herdado e aprovado no E2).

2. **Parte B — Simulação do Custo Amortizado de Deriva ($E_{\text{amortized}}$):**
   - Aplicação do modelo empírico de retenção de pesos $w(t) = w_0 \cdot (t / t_0)^{-\nu}$.
   - Determinação da frequência de recalibração $N_{\text{recal}}$ necessária para manter a acurácia no limiar aceitável ($\ge 90\%$).
   - Cálculo da Energia Amortizada por Inferência:
     $$E_{\text{amortized}} = E_{\text{inference}} + \frac{E_{\text{recalibration}}}{N_{\text{inferences}}}$$

---

## 3. Critérios de Sucesso e Gates Invioláveis (Gate E3 → E4)

| Código | Critério / Regra | Limiar Numérico / Tolerância | Ação em Caso de Falha |
|---|---|---|---|
| **G3.1** | **Repetibilidade da Coleta Empírica** | $CV \le 15\%$ na medição dos prompts $L_{128}, L_{512}, L_{1024}$ na GPU T4 | Re-amostrar caso ocorra variabilidade anormal nos sensores NVML. |
| **G3.2** | **Estabilidade de Baseline C1** | Deriva de Baseline entre Pré e Pós-coleta $\le 5.0\%$ | Manter o resfriamento via `multiprocessing` e *smart cooldown*. |
| **G3.3** | **Análise de Sensibilidade Declarada** | A retenção e o custo amortizado devem ser reportados como faixa (intervalo $CI_{95\%}$), não como valor pontual único | Descartar estimativas pontuais sem margem de erro modelada. |
| **G3.4** | **Transparência de Natureza Híbrida** | Declaração explícita no relatório destacando as partes empíricas vs simuladas | **REGRA INVIOLÁVEL:** Jamais apresentar dados simulados de deriva como se fossem medição física do silício. |

---

## 4. Estrutura do Experimento E3 no Repositório

- **Diretório do Teste:** `docs/experiments/E3_Custo_Deriva/`
  - `preregistration.md` (Este documento)
  - `e3_drift_cost_collector.py` (Script de medição e simulação)
  - `report.md` (Relatório científico final)
  - `artifacts/E3_raw_data.json` (Dados empíricos e simulados gravados)
