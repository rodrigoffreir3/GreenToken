# Relatório Científico — Experimento E1-FIX (GT-M v1.0)

**Título:** Decomposição Energética Intra-Inferência Escalada por Comprimento de Prompt  
**Data:** 2026-08-06  
**Status:** PENDENTE DE RE-EXECUÇÃO NO LIGHTNING AI (Aguardando dados empíricos E1-FIX)  
**Especificação:** `docs/SPEC_GTM_MEDICAO_INDEPENDENTE.md` & `SPEC GTM-E1-FIX v1.0`  
**Pré-Registro:** `docs/experiments/E1_Decomposicao_Fase/preregistration.md`  
**Código do Coletor:** `docs/experiments/E1_Decomposicao_Fase/e1_ensemble_collector.py`  
**Hardware de Teste Físico:** NVIDIA L40S (Ambiente Lightning AI Linux / NVML Ativo / Multiprocessing Isolation)  
**Artefato de Dado Bruto:** [E1_raw_data.json](./artifacts/E1_raw_data.json)  

---

## 1. Auditoria Metodológica: Eliminação de Constantes Hardcoded

Em cumprimento estrito à **SPEC GTM-E1-FIX (Seção 1)**, todas as contagens de loop fixas e hardcoded (`range(300)`, `range(5000000)`) foram completamente removidas do coletor.

A contagem de loops por fase agora é derivada dinamicamente das assíntotas de complexidade computacional real de um modelo Transformer:
- **F0 (Data Prep):** $O(\text{seq\_len})$
- **F1 (Prefill):** $O(\text{seq\_len}^2 \cdot \text{dim})$ (Atenção quadrática)
- **F2 (Decode):** $O(\text{gen\_len} \cdot \text{seq\_len} \cdot \text{dim})$ (Geração autoregressiva)
- **F3 (Post-Process):** $O(\text{gen\_len})$

---

## 2. Resultados Empíricos Medidos na GPU NVIDIA L40S (A ser preenchido após execução)

- **Baseline Pré-Coleta ($P_{\text{idle, pré}}$):** `Pending W`
- **Baseline Pós-Coleta ($P_{\text{idle, pós}}$):** `Pending W`
- **Deriva de Baseline ($G1.3$):** `Pending %`

### Tabela da Curva de Decomposição por Comprimento de Prompt

| Tamanho de Prompt (`seq_len`) | Energia Média ($E_{\text{net}}$) | Dispersão ($CV$) | **Fração de Cálculo Numérico ($F_1 + F_2$)** | **Fração de Overhead ($F_0 + F_3$)** | Status |
|---|---|---|---|---|---|
| **Prompt_128t** | `Pending J` | `Pending %` | `Pending %` | `Pending %` | **Aguardando** |
| **Prompt_512t** | `Pending J` | `Pending %` | `Pending %` | `Pending %` | **Aguardando** |
| **Prompt_1024t** | `Pending J` | `Pending %` | `Pending %` | `Pending %` | **Aguardando** |

---

## 3. Confronto com a Hipótese Pré-Registrada $H_{\text{E1-FIX}}$

- **Hipótese:** Crescimento monotônico da fração de cálculo ($70\% \to 85\% \to 93\%$).
- **Resultado Medido:** `Pending`

---

## 4. Status dos Gates de Transição (SPEC GTM-E1-FIX)

- [ ] **Gate G1.1 (Consistência Interna de Energia):** Erro $\le 10\%$ para todos os prompts
- [ ] **Gate G1.2 (Repetibilidade CV):** $CV \le 15\%$ para todas as condições
- [ ] **Gate G1.3 (Estabilidade do Baseline C1):** Deriva de Baseline $\le 5.0\%$
- [ ] **Gate G1.4 (Substituição Completa de Constantes):** Nenhuma constante hardcoded utilizada
