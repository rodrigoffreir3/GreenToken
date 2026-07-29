# Relatório Científico — Experimento E1 (GT-M)

**Título:** Decomposição Energética Intra-Inferência e Validação de Reconstrução por Ensemble  
**Data:** 2026-07-29  
**Status:** PENDENTE / AGUARDANDO NÓ DE EXECUÇÃO FÍSICO (Gate E1 BLOQUEADO 🛑)  
**Especificação:** `docs/SPEC_GTM_MEDICAO_INDEPENDENTE.md`  
**Pré-Registro:** `docs/preregistration_E1.md` (Commit: `171248e`)  
**Código do Coletor:** `scripts/e1_ensemble_collector.py` (Purgado de fallbacks sintéticos)  

---

## 1. Declaração de Rigor e Auditoria de Sensores

Em estrito cumprimento à **Seção 0 do Protocolo GT-M** ("Nenhum dado é ajustado para parecer bonito"), o coletor `scripts/e1_ensemble_collector.py` foi auditado e purgado de qualquer estimativa sintética.

- **Status da Amostragem Local (Host Windows):**
  - RAPL (`/sys/class/powercap/intel-rapl`): **AUSENTE**
  - NVML (`nvidia-smi` / `pynvml`): **AUSENTE**
  - Resultado: Interrupção imediata com `RuntimeError` para evitar contaminação por dados sintéticos.

---

## 2. Status dos Gates de Transição (Gate E1 → E2)

- [ ] **Gate G1.1 (Consistência Interna de Energia):** [BLOQUEADO — Aguardando leitura física]
- [ ] **Gate G1.2 (Repetibilidade CV):** [BLOQUEADO — Aguardando leitura física]
- [ ] **Gate G1.3 (Estabilidade do Baseline C1):** [BLOQUEADO — Aguardando leitura física]
- [ ] **Gate G1.4 (Estabilidade Térmica C2):** [BLOQUEADO — Aguardando leitura física]
- [ ] **Gate G1.5 (Resolução de Fases):** [BLOQUEADO — Aguardando leitura física]

---

## 3. Próximo Passo Requerido

Para avançar com a coleta física real sem mascaramento:
1. Execução do `scripts/e1_ensemble_collector.py` em um nó Linux nativo com RAPL ou em ambiente Kaggle (Tesla T4) com NVML.
2. Análise dos dados brutos reais coletados para validação dos gates.
