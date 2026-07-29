# Pré-Registro Científico — Experimento E2 (GT-M)

**Título:** Fronteira de Pareto Energia-versus-Acurácia sob Variação de Precisão e Quantização de LLM  
**Data de Pré-Registro:** 2026-07-29  
**Status:** PRÉ-REGISTRADO (Commitado antes da primeira coleta do E2)  
**Especificação-Mãe:** `docs/SPEC_GTM_MEDICAO_INDEPENDENTE.md`  
**Dependência:** Experimento E1 CONCLUÍDO E APROVADO (Gate E1 ✅)  
**Autores:** Rodrigo Freire (Condução) & Antigravity (Tooling & Automação)  

---

## 1. Declaração de Hipótese Pré-Registrada $H_{E2}$

> **Hipótese $H_{E2}$:**  
> *"Ao comparar a inferência de um modelo de linguagem em três níveis de precisão/quantização (ex: FP16/Half, INT8/Quantizada e INT4/Sub-8-bit) sob uma tarefa de avaliação objetiva com dataset fixo, a redução de precisão de FP16 para INT4 reduzirá o consumo de energia por inferência em **50% ± 15%**, enquanto a degradação de acurácia relativa na tarefa será mantida abaixo de **5% ± 2%**."*

---

## 2. Metodologia de Medição da Fronteira de Pareto

1. **Dataset de Avaliação Fixo:** Conjunto fixo de $M = 50$ prompts de avaliação determinística com semente fixa e `temperature=0`.
2. **Configurações de Precisão a Comparar:**
   - Configuração A: **`FP16`** (Precisão meia flutuante original / baseline de acurácia).
   - Configuração B: **`INT8`** (Quantização de 8 bits).
   - Configuração C: **`INT4` / `q4_k_m`** (Quantização agressiva de 4 bits).
3. **Instrumentação de Energia:** Reuso direto do coletor de amostragem contínua em alta frequência ($10\text{ ms}$) validado no Experimento E1 (`ContinuousNVMLSampler`).
4. **Métricas a Registrar por Configuração:**
   - Acurácia / Taxa de Acerto Exato no dataset ($Acc\%$).
   - Energia Média por Inferência em Joules ($E_{\text{net}}$) com desvio padrão e $CV$.
   - Latência Média de Inferência em milissegundos ($t_{\text{ms}}$).

---

## 3. Critérios de Sucesso e Gates Invioláveis (Gate E2 → E3)

| Código | Critério / Regra | Limiar Numérico / Tolerância | Ação em Caso de Falha |
|---|---|---|---|
| **G2.1** | **Repetibilidade da Fronteira de Pareto** | A curva Energia vs Acurácia deve se manter estatisticamente idêntica em dias/runs diferentes | Se houver inversão de curva ou ruído excessivo, descartar e investigar o ambiente. |
| **G2.2** | **Validação Metodológica de Energia** | Herda $CV \le 15\%$ e consistência do E1 para cada ponto da curva | Se uma das precisões falhar no $CV$, re-amostrar. |
| **G2.3** | **Intervalos de Confiança Declarados** | Toda acurácia e energia acompanham intervalo de confiança ($CI_{95\%}$) | **REGRA DE RIGOR:** Não publicar valores pontuais sem margem de erro. |
| **G2.4** | **Declaração de Confounders** | Qualquer diferença que não seja a precisão do modelo deve ser listada | Ex: diferenças na biblioteca de kernel CUDA usada. |

---

## 4. Estrutura do Experimento E2 no Repositório

- **Diretório do Teste:** `docs/experiments/E2_Iso_Acuracia/`
  - `preregistration.md` (Este documento)
  - `e2_iso_accuracy_collector.py` (Script de automação)
  - `artifacts/E2_raw_data.json` (Dados brutos do silício)
  - `report.md` (Relatório científico final do E2)
