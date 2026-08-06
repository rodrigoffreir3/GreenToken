# Relatório Científico — Experimento E4 (GT-M Encerramento da Série)

**Título:** Energia Ajustada por Ciclo de Trabalho (Duty Cycle Energy) em Deployment Realista de Inferência  
**Data:** 2026-08-05  
**Status:** PENDENTE DE EXECUÇÃO NO KAGGLE (Aguardando dados empíricos)  
**Especificação:** `docs/SPEC_GTM_MEDICAO_INDEPENDENTE.md` (Seção E4)  
**Pré-Registro:** `docs/experiments/E4_Ciclo_Trabalho/preregistration.md`  
**Código do Coletor:** `docs/experiments/E4_Ciclo_Trabalho/e4_duty_cycle_collector.py`  
**Hardware de Teste Físico:** NVIDIA Tesla T4 (Ambiente Kaggle Linux / NVML Ativo / Multiprocessing Isolation)  
**Dado Bruto:** `docs/experiments/E4_Ciclo_Trabalho/artifacts/E4_raw_data.json`  

---

## 1. Declaração Metodológica e Trava de Integridade (Gate G4.3)

> **Declaração de Normalização Obrigatória:** Em conformidade estrita com o aprendizado metodológico dos experimentos anteriores e com a exigência do Gate G4.3, o Experimento E4 aplica a divisão automática da energia total gasta na janela ($E_{\text{total}}$) pelo número exacto de inferências úteis entregues ($N_{\text{inferências}}$):
> $$E_{\text{amortizada, útil}} = \frac{E_{\text{total}}}{N_{\text{inferências}}}$$
> É terminantemente vedada a comparação bruta entre janelas temporais de perfis distintos sem a devida normalização por inferência útil.

---

## 2. Resultados Empíricos no Silício (A ser preenchido após execução no Kaggle)

- **Baseline Pré-Coleta ($P_{\text{idle, pré}}$):** `Pending W` (Estado Real Ocioso P8)
- **Baseline Pós-Coleta ($P_{\text{idle, pós}}$):** `Pending W`
- **Deriva de Baseline ($G4.2$):** `Pending %`

### Tabela de Perfis de Utilização de Carga (Duty Cycle)

| Perfil de Carga | Utilização Carga/Idle | Duração da Janela | Energia Total da Janela | **Energia Amortizada por Inferência Útil** | Fator de Degradação vs Pico | Status |
|---|---|---|---|---|---|---|
| **Profile_100pct_Saturada** | 100% Carga (Pico) | `Pending s` | `Pending J` | `Pending J / inf` | **1.00x** (Baseline) | **Aguardando** |
| **Profile_50pct_Alta** | 50% Carga / 50% Idle | `Pending s` | `Pending J` | `Pending J / inf` | `Pending x` | **Aguardando** |
| **Profile_20pct_Media** | 20% Carga / 80% Idle | `Pending s` | `Pending J` | `Pending J / inf` | `Pending x` | **Aguardando** |
| **Profile_5pct_Baixa** | 5% Carga / 95% Idle | `Pending s` | `Pending J` | `Pending J / inf` | `Pending x` | **Aguardando** |

---

## 3. Confronto com a Hipótese Pré-Registrada $H_{E4}$

- **Hipótese:** Para utilização de 5% de carga, o Fator de Degradação $\ge 5.0\text{x} \pm 1.5\text{x}$.
- **Resultado Medido:** `Pending`

---

## 4. Status dos Gates de Transição (Fechamento Geral da Série GT-M)

- [ ] **Gate G4.1 (Repetibilidade e Estabilidade Estacionária):** $CV \le 15\%$ entre perfis de carga
- [ ] **Gate G4.2 (Estabilidade de Baseline C1):** Deriva de Baseline $\le 5.0\%$
- [ ] **Gate G4.3 (Trava de Normalização por Inferência Útil):** Divisão automática por $N_{\text{inferências}}$ aplicada e verificada
- [ ] **Gate G4.4 (Declaração de Neutralidade de Hardware):** Método reproduzível e independente do vendedor de silício
