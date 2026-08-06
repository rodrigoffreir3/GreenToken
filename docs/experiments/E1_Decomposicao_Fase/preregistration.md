# Pré-Registro Científico — Experimento E1-FIX (GT-M v1.0)

**Título:** Decomposição Energética Intra-Inferência Escalada por Comprimento de Prompt  
**Data de Pré-Registro:** 2026-08-06  
**Status:** PRÉ-REGISTRADO (Commitado antes da primeira coleta da spec GTM-E1-FIX)  
**Especificação-Mãe:** `docs/SPEC_GTM_MEDICAO_INDEPENDENTE.md` & `SPEC GTM-E1-FIX v1.0`  
**Autores:** Rodrigo Freire (Condução) & Antigravity (Tooling & Automação)  

---

## 1. Declaração de Hipótese Pré-Registrada $H_{\text{E1-FIX}}$

> **Hipótese $H_{\text{E1-FIX}}$:**  
> *"Se a fração de energia dedicada ao cálculo numérico puro ($F_1 + F_2$, Prefill e Decode) for governada pela física de atenção e multiplicação de matrizes, ela crescerá monotonicamente com o comprimento do prompt ($N$), porque o custo de Prefill escala quadraticamente $O(N^2 \cdot \text{dim})$ enquanto o overhead de preparação e pós-processamento ($F_0 + F_3$) escala de forma aproximadamente linear $O(N)$."*
>
> **Estimativas Pré-Registradas por Comprimento de Prompt:**
> - **128 tokens:** Fração de Cálculo ($F_1 + F_2$) estimadamente em **70% ± 5%** (Overhead $F_0 + F_3 \approx 30\%$).
> - **512 tokens:** Fração de Cálculo ($F_1 + F_2$) estimadamente em **85% ± 5%** (Overhead $F_0 + F_3 \approx 15\%$).
> - **1024 tokens:** Fração de Cálculo ($F_1 + F_2$) estimadamente em **93% ± 4%** (Overhead $F_0 + F_3 \approx 7\%$).

---

## 2. Metodologia de Escalamento Físico Dinâmico por Fase

Substitui-se qualquer constante hardcoded (`range(300)`, `range(5000000)`) por contagens de loop derivadas da complexidade assintótica real de cada fase:

```python
def compute_phase_loops(seq_len: int, gen_len: int, dim: int, base_unit_ops: int) -> dict:
    f0 = max(50, seq_len // 4)
    f1 = max(base_unit_ops, (seq_len ** 2) * dim // base_unit_ops)
    f2 = max(base_unit_ops, gen_len * seq_len * dim // base_unit_ops)
    f3 = max(50, gen_len * 2)
    return {"F0": f0, "F1": f1, "F2": f2, "F3": f3}
```

- **Piso de Amostragem sem Aliasing:** `base_unit_ops` é calibrado via probe dinâmico para garantir tempo por fase $\ge 0.35\text{ s}$ ($\ge 25$ amostras NVML a $10\text{ ms}$).
- **Três Condições de Prompt:** $128$, $512$ e $1024$ tokens.
- **Série de Repetição:** $N = 30$ execuções independentes por comprimento de prompt, sob isolamento de processo `multiprocessing.Process`.

---

## 3. Critérios de Sucesso e Gates Invioláveis (SPEC GTM-E1-FIX)

| Código | Critério / Regra | Limiar Numérico / Tolerância | Ação em Caso de Falha |
|---|---|---|---|
| **G1.1** | **Consistência Interna por Condição** | $|E_{\text{total}} - \sum E_{F_i}| / E_{\text{total}} \le 10\%$ para cada prompt | **FALHA DE INTEGRALIZAÇÃO:** Descartar a condição. |
| **G1.2** | **Repetibilidade por Condição (CV)** | $CV \le 15\%$ para cada comprimento de prompt | **RUÍDO EXCESSIVO:** Re-amostrar. |
| **G1.3** | **Estabilidade do Baseline (C1)** | $|P_{\text{idle, pós}} - P_{\text{idle, pré}}| / P_{\text{idle, pré}} \le 5\%$ | **DERIVA TÉRMICA:** Aplicar Smart Thermal Cooldown. |
| **G1.4** | **Substituição de Constantes** | Nenhuma constante hardcoded sem fórmula assintótica | **VIOLAÇÃO METODOLÓGICA:** Abortar script. |
