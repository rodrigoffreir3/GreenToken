# Pré-Registro Científico — Experimento E3 (GT-M FIX v1.0)

**Título:** Custo Energético da Deriva e Amortização por Recalibração sob Variabilidade de Prompt e Temperatura  
**Data de Pré-Registro:** 2026-07-29  
**Status:** PRÉ-REGISTRADO / REVISADO (Conforme SPEC GTM-E3-FIX v1.0)  
**Especificação-Mãe:** `docs/SPEC_GTM_MEDICAO_INDEPENDENTE.md` & `SPEC GTM-E3-FIX v1.0`  
**Dependência:** Experimento E2 CONCLUÍDO E APROVADO (Gate E2 ✅)  
**Autores:** Rodrigo Freire (Condução) & Antigravity (Tooling & Automação)  

---

## 1. Declaração de Hipótese Pré-Registrada $H_{E3}$ e Transparência Metodológica

> **Aviso Metodológico Fundamental (Conforme Seção 3 do SPEC GT-M & SPEC GTM-E3-FIX):**  
> *"O Experimento E3 possui natureza híbrida: combina a **medição física empírica** do custo energético de inferência em hardware real (NVIDIA Tesla T4) sob variação de tamanho de contexto/prompt e temperatura com a **modelagem matemática e simulação** da retenção de pesos e custo de recalibração temporal ($w(t) = w_0 \cdot t^{-\nu}$). Esta mudança de natureza é declarada com destaque para garantir 100% de rigor científico e transparência."*

> **Hipótese $H_{E3}$:**  
> *"Ao isolar a variação do tamanho do prompt de entrada (128, 512 e 1024 tokens com $T=0.7$ constante) e a variação da temperatura de amostragem ($T \in \{0.0, 0.7, 1.0\}$ com $L=512$ constante), a energia líquida por inferência escala de forma linear com o comprimento do contexto, enquanto a variação da temperatura de amostragem introduz variação $< 5\%$ na energia por inferência."*

---

## 2. Metodologia de Medição e Simulação Híbrida (FIX v1.0)

1. **Parte A — Coleta Empírica Desacoplada no Silício (NVIDIA Tesla T4):**
   - **Série 1 (Variação de Tamanho de Prompt / Contexto):**
     - $L_{128}$: 128 tokens ($T = 0.7$ fixo).
     - $L_{512}$: 512 tokens ($T = 0.7$ fixo).
     - $L_{1024}$: 1024 tokens ($T = 0.7$ fixo).
   - **Série 2 (Variação de Temperatura de Amostragem):**
     - $T_{0.0}$: Temperatura $0.0$ ($L = 512$ fixo).
     - $T_{0.7}$: Temperatura $0.7$ ($L = 512$ fixo).
     - $T_{1.0}$: Temperatura $1.0$ ($L = 512$ fixo).
   - **Calibração Dinâmica de Duração:** Cada condição determina o número de loops necessário para durar no mínimo `MIN_DURATION_S = 0.35s`, garantindo $\ge 25$ amostras por run no `ContinuousNVMLSampler`.
   - **Rejeição de Clamp Silencioso:** Qualquer repetição com $J_{\text{gross}} < J_{\text{floor}}$ ou $< 25$ amostras é contabilizada como repetição inválida. Se a taxa de falhas exceder $20\%$, a condição falha como `INSTRUMENTATION_FAILURE`.

2. **Parte B — Simulação do Custo Amortizado de Deriva ($E_{\text{amortized}}$):**
   - Aplicação do modelo empírico de retenção de pesos $w(t) = w_0 \cdot (t / t_0)^{-\nu}$.
   - Cálculo da Energia Amortizada por Inferência:
     $$E_{\text{amortized}} = E_{\text{inference}} + \frac{E_{\text{recalibration}}}{N_{\text{inferences}}}$$

---

## 3. Critérios de Sucesso e Gates Invioláveis (Gate E3 → E4)

| Código | Critério / Regra | Limiar Numérico / Tolerância | Ação em Caso de Falha |
|---|---|---|---|
| **G3.1** | **Repetibilidade da Coleta Empírica** | $CV \le 15\%$ em cada sub-série e taxa de invalid_samples $\le 20\%$ | Reprovar condição como `INSTRUMENTATION_FAILURE`. |
| **G3.2** | **Estabilidade de Baseline C1** | Deriva de Baseline entre Pré e Pós-coleta $\le 5.0\%$ | Manter o resfriamento via `multiprocessing` e *smart cooldown*. |
| **G3.3** | **Análise de Sensibilidade Declarada** | Sub-séries reportadas de forma separada sem acoplamento causal | Descartar médias misturadas onde variáveis foram alteradas simultaneamente. |
| **G3.4** | **Transparência de Natureza Híbrida** | Declaração explícita no relatório destacando as partes empíricas vs simuladas | **REGRA INVIOLÁVEL:** Jamais apresentar dados simulados de deriva como se fossem medição física do silício. |

---

## 4. Histórico de Auditoria de Bugs e Reruns

- **Commit `20a7f88` (Primeiro Run Real):** Reprovado no Gate G3.1 ($CV = 36.04\%$ no prompt 512t e colapso de aliasing no prompt 128t). Motivo: `base_loops` fixo sem calibração, clamp silencioso `max(0.0, ...)`, e acoplamento desnecessário entre prompt e temperatura.
- **Commit Atual (FIX v1.0):** Implementação total do `SPEC GTM-E3-FIX v1.0`.
