# Pré-Registro Científico — Experimento E4 (GT-M)

**Título:** Energia Ajustada por Ciclo de Trabalho (Duty Cycle Energy) em Deployment Realista de Inferência  
**Data de Pré-Registro:** 2026-08-05  
**Status:** PRÉ-REGISTRADO (Commitado antes da primeira execução do E4)  
**Especificação-Mãe:** `docs/SPEC_GTM_MEDICAO_INDEPENDENTE.md` (Seção E4)  
**Dependências:** Experimentos E1, E2 e E3 CONCLUÍDOS E APROVADOS (Gates E1, E2 e E3 ✅)  
**Autores:** Rodrigo Freire (Condução) & Antigravity (Tooling & Automação)  

---

## 1. Declaração de Hipótese Pré-Registrada $H_{E4}$

> **Hipótese $H_{E4}$:**  
> *"Em um ambiente de produção realista com chegada de requisições esparsas, a energia real por inferência útil entregue aumenta drasticamente em relação à energia de pico (saturada). Especificamente, para um ciclo de trabalho de 5% de utilização (95% de tempo ocioso), o fator de degradação da energia amortizada por inferência útil em relação ao valor de pico saturado será de no mínimo **5.0x ± 1.5x** (500% do consumo nominal), devido à dominância da potência ociosa de repouso ($P_{\text{idle}}$)."*

---

## 2. Metodologia de Medição e Perfis de Ciclo de Trabalho

1. **Perfis de Utilização de Carga (Duty Cycles):**
   - **Saturado (100% Carga):** Inferência contínua sem pausas (baseline tradicional de benchmark/folheto).
   - **Alta Utilização (50% Carga):** Janelas de carga intercaladas com 50% de tempo em repouso ocioso.
   - **Média Utilização (20% Carga):** Janelas de carga intercaladas com 80% de tempo em repouso ocioso.
   - **Baixa Utilização (5% Carga):** Requisições esparsas com 95% de tempo em repouso ocioso.

2. **Instrumentação e Coleta Físico-Empírica (NVIDIA Tesla T4):**
   - **Isolamento de P-State:** Medição contínua via `ContinuousNVMLSampler` em janela temporal longa para capturar o comportamento de estado estacionário.
   - **Separação de Componentes Energéticos:**
     - Energia Total na Janela ($E_{\text{total}}$).
     - Energia Ociosa de Repouso ($E_{\text{idle}} = P_{\text{idle}} \times \Delta t_{\text{janela}}$).
     - Energia Ativa de Carga ($E_{\text{active}} = E_{\text{total}} - E_{\text{idle}}$).

3. **Trava de Integridade Físico-Normalizada Inviolável:**
   - O coletor divide estritamente a energia total $E_{\text{total}}$ pelo número de inferências úteis entregues ($N_{\text{inferências}}$):
     $$E_{\text{amortizada, útil}} = \frac{E_{\text{total}}}{N_{\text{inferências}}}$$
   - O Fator de Degradação é calculado como:
     $$\text{Fator de Degradação} = \frac{E_{\text{amortizada, útil}}}{E_{\text{pico, 100\%}}}$$

---

## 3. Critérios de Sucesso e Gates Invioláveis (Fechamento da Série GT-M)

| Código | Critério / Regra | Limiar Numérico / Tolerância | Ação em Caso de Falha |
|---|---|---|---|
| **G4.1** | **Repetibilidade e Estabilidade Estacionária** | $CV \le 15\%$ entre janelas de medição para cada perfil de utilização | Re-amostrar em janela mais longa. |
| **G4.2** | **Estabilidade do Baseline Ocioso** | Deriva de Baseline pré/pós $P_{\text{idle}} \le 5.0\%$ | Resfriamento térmico via `multiprocessing` e *smart cooldown*. |
| **G4.3** | **Normalização Automática Obrigatória** | Exigência via código de divisão por $N_{\text{inferências}}$ em qualquer comparação | **MECANISMO DE TRAVA:** Código aborta se contagens divirjam sem normalização por unidade útil. |
| **G4.4** | **Teste de Neutralidade de Hardware** | Metodologia aplicável sem alteração a hardware de outros fabricantes | Não depender de chamadas proprietárias exclusivas sem fallback genérico. |

---

## 4. Estrutura do Experimento E4 no Repositório

- **Diretório do Teste:** `docs/experiments/E4_Ciclo_Trabalho/`
  - `preregistration.md` (Este documento)
  - `e4_duty_cycle_collector.py` (Script de automação e amostragem em longo ciclo)
  - `report.md` (Relatório científico final de encerramento da série GT-M)
  - `artifacts/E4_raw_data.json` (Métricas brutas gravadas)
