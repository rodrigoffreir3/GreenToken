# SPEC GT-M — Metodologia de Medição Energética Independente para Inferência de IA

**Versão:** 1.0
**Status:** Aberto — E1 pronto para execução, E2–E4 bloqueados por dependência
**Instrumento base:** GreenToken (agent Go: RAPL + NVML + eBPF + `/metrics`), validado em GT-02 com 0% de erro de contagem de token sob carga concorrente
**Executor:** Rodrigo (condução) + Antigravity (implementação de tooling)
**Destino:** quatro artefatos reprodutíveis, cada um publicável como preprint independente

---

## 0. Declaração de posição (ler antes de tudo)

Esta pesquisa **não tem interesse comercial no resultado**. Não há produto a vender, fornecedor a favorecer, nem tese a confirmar. Isso não é retórica: é a única vantagem competitiva real desta linha de trabalho. Fabricante não pode medir a si mesmo com credibilidade; pesquisador sem produto pode.

Essa posição só vale se for defendida operacionalmente. Portanto:

1. **Pré-registro obrigatório.** A hipótese e o critério de sucesso de cada experimento são escritos e commitados **antes** da primeira medição. Alterar hipótese depois de ver o dado invalida o experimento.
2. **Resultado negativo é resultado.** Se a medição mostrar que uma abordagem é pior do que se afirma, ou que o próprio método não funciona, isso é publicado com o mesmo rigor. Não existe "experimento que deu errado" — existe experimento que respondeu.
3. **Sucesso é metodológico, não empírico.** O critério para avançar de um experimento ao próximo é *a medição ser confiável e reprodutível*, nunca *o número ter dado bonito*. Esta distinção governa toda a seção de gates.
4. **Nenhuma incerteza fica em silêncio.** Todo número publicado vem acompanhado de desvio, número de repetições, e fonte de erro conhecida. Média sem dispersão é dado incompleto.
5. **Nenhuma medição confia em si mesma.** Toda grandeza crítica é validada por pelo menos duas vias independentes antes de virar resultado.

---

## 1. A restrição física que governa tudo (não contornar, declarar)

Antes de qualquer experimento, é preciso registrar honestamente o teto dos instrumentos disponíveis:

| Instrumento | O que mede | Granularidade real |
|---|---|---|
| eBPF `sched_switch` | Quando um PID entra/sai da CPU | Microssegundo |
| RAPL (`/sys/class/powercap`) | Energia acumulada CPU/DRAM | Contador atualiza na ordem de ~1 ms |
| NVML (`nvmlDeviceGetPowerUsage`) | Potência instantânea da GPU | Dependente do device; frequentemente dezenas de ms |

**Consequência inescapável:** o *tempo* é conhecido com precisão de microssegundo, mas a *energia* não. Qualquer afirmação de "decomposição energética em microssegundo por medição direta" seria falsa.

**A saída legítima é reconstrução por ensemble.** A mesma inferência é repetida N vezes; a linha do tempo de fase (obtida via eBPF, precisa) é usada para alinhar as amostras; o perfil energético é reconstruído estatisticamente a partir da superposição. É a mesma técnica de média de osciloscópio, e é válida **desde que declarada** e desde que a variância entre repetições seja reportada.

Esta limitação deve aparecer explicitamente na seção de métodos de qualquer artigo derivado. Ocultá-la seria fraude metodológica.

---

## 2. Controles obrigatórios em todos os experimentos

Aplicam-se sem exceção a E1–E4. Um experimento que não os respeite é inválido, independentemente do resultado.

**C1 — Baseline ocioso.** Antes e depois de cada série, medir a potência do sistema em repouso por no mínimo 60 s. Todo consumo reportado é *delta sobre baseline*, nunca absoluto. Baseline pré e pós devem concordar dentro de 5%; se não concordarem, houve deriva térmica ou interferência e a série é descartada.

**C2 — Estado térmico estacionário.** Silício frio consome diferente de silício quente. Executar carga de aquecimento até a temperatura estabilizar (variação < 2 °C por minuto) antes de iniciar a coleta. Registrar temperatura ao longo de toda a série.

**C3 — Repetição mínima.** Nenhuma conclusão a partir de série única. Mínimo de 30 repetições por condição, com desvio padrão e coeficiente de variação reportados. Séries com CV > 15% exigem investigação da fonte de ruído antes de qualquer interpretação.

**C4 — Isolamento de ruído de sistema.** Registrar e reportar: processos concorrentes ativos, governor de frequência da CPU, estado do turbo boost, e qualquer throttling térmico ocorrido durante a série. Idealmente fixar o governor em modo determinístico.

**C5 — Validação cruzada de instrumento.** A energia total reportada por RAPL+NVML durante uma série deve ser confrontada com uma segunda fonte independente sempre que houver uma disponível (medidor de tomada, telemetria do provedor de nuvem, contador do BMC). Divergência acima de 10% é reportada, não escondida.

**C6 — Artefato de reprodução.** Cada experimento produz: código de coleta, dado bruto (não só agregado), procedimento passo a passo, e versão exata de kernel, driver, modelo e engine de inferência. Sem isso, o resultado não é publicável.

---

## 3. Os quatro experimentos

### E1 — Decomposição energética dentro de uma única inferência

**Pergunta:** dentro de uma inferência, quanto da energia vai para carregamento de dado, processamento de prompt (prefill), geração de token (decode), e devolução do resultado?

**Por que importa:** toda a promessa de hardware alternativo (analógico, in-memory, acelerador dedicado) é que o *cálculo* fica barato. Se o cálculo já não for a fração dominante do custo em carga real, o ganho prometido evapora no sistema. Ninguém publicou essa decomposição de forma neutra.

**Hipótese pré-registrada (a ser escrita antes da coleta, formato):**
> "Em inferência de LLM em [hardware X] com [modelo Y], a fração de energia atribuível ao cálculo propriamente dito será de ___% ± ___, com o restante distribuído entre movimentação de dado e overhead de orquestração."

Preencher com estimativa **antes** de medir. Errar a estimativa não é problema; alterá-la depois é.

**Método:**
1. Instrumentar o engine de inferência para emitir marcadores de fase (início/fim de prefill, início/fim de decode). Fonte preferencial: `/metrics` do engine. Fallback: tracepoint via eBPF em pontos conhecidos do processo.
2. Coletar simultaneamente: timeline de escalonamento via eBPF (µs), RAPL (CPU/DRAM), NVML (GPU), na maior taxa que o hardware permitir.
3. Repetir a mesma inferência (prompt idêntico, `temperature=0`, `max_tokens` fixo) no mínimo 30 vezes.
4. Alinhar as N séries pelo marcador de início de fase e reconstruir o perfil energético médio por ensemble.
5. Integrar a potência sobre cada janela de fase para obter energia por fase.

**Fontes de erro a declarar:** latência de atualização do sensor contra duração da fase (se uma fase for mais curta que o período de amostragem, ela **não** é resolvível — declarar quais fases caem nessa categoria); jitter de escalonamento; sobreposição de fases em execução assíncrona.

**Critério de sucesso (gate para E2):**
- [ ] As fases são separáveis com incerteza declarada, e as fases não-resolvíveis estão explicitamente listadas como tal.
- [ ] CV da energia total por repetição < 15% (C3).
- [ ] Soma das energias por fase reconstrói a energia total medida independentemente, com erro < 10%. **Este é o teste de consistência interna: se as partes não somam o todo, o método está errado.**
- [ ] Baseline pré e pós concordam dentro de 5%.
- [ ] Terceiro reproduz o procedimento a partir do artefato e chega ao mesmo resultado dentro da incerteza declarada.

Note que nenhum desses critérios diz respeito a *qual* fase dominou. O gate é sobre confiabilidade, não sobre o número.

---

### E2 — Energia a iso-acurácia

**Pergunta:** com a acurácia igualada entre configurações, quanta energia cada uma consome por inferência?

**Por que importa:** "3,5 W" não significa nada sem "entregando qual acurácia". Hardware e quantização que trocam precisão por energia são frequentemente comparados contra baselines de precisão diferente, o que torna a comparação inválida. A métrica honesta é a fronteira de Pareto energia-versus-acurácia.

**Hipótese pré-registrada:** estimar, antes de medir, a curva esperada de degradação de acurácia por nível de quantização e o ganho energético correspondente.

**Método:**
1. Escolher uma tarefa com métrica de acurácia objetiva e um conjunto de avaliação fixo (mesmos prompts, mesma ordem, semente fixa).
2. Executar o mesmo modelo em múltiplas configurações de precisão (por exemplo FP16, INT8, INT4) mantendo todo o resto constante.
3. Para cada configuração, medir acurácia na tarefa **e** energia por inferência, usando a instrumentação validada em E1.
4. Plotar a fronteira de Pareto. Reportar, para cada ponto, acurácia com intervalo de confiança e energia com desvio.

**Fontes de erro a declarar:** variabilidade da própria métrica de acurácia entre execuções; efeito do tamanho do conjunto de avaliação sobre a incerteza; diferenças de implementação de kernel entre níveis de precisão que não são atribuíveis à precisão em si.

**Critério de sucesso (gate para E3):**
- [ ] A acurácia de cada configuração tem intervalo de confiança reportado, e a diferença entre configurações é estatisticamente distinguível (ou declarada como indistinguível).
- [ ] A energia por inferência usa a metodologia validada em E1, com a mesma consistência interna.
- [ ] A fronteira é reprodutível: repetir a série completa em outro dia produz a mesma curva dentro da incerteza.
- [ ] Confounders declarados: qualquer diferença entre configurações que **não** seja a precisão está listada.

---

### E3 — Custo energético da deriva

**Pergunta:** quanto custa, em energia, *manter* a acurácia ao longo do tempo em hardware sujeito a deriva?

**Aviso metodológico crítico:** este experimento é **simulação, não medição**, salvo se houver acesso a hardware analógico real. A mudança de natureza em relação a E1/E2 deve ser declarada com destaque em qualquer publicação. Simulação responde "o que aconteceria se o modelo de deriva estiver correto"; não responde "o que acontece no silício".

**Por que importa:** dispositivo analógico deriva com tempo e temperatura e exige recalibração. Recalibração consome energia e tempo. A métrica publicada pela indústria é energia *no instante da inferência*, nunca energia amortizada pela manutenção da acurácia ao longo da vida útil. É a pergunta mais desconfortável do campo e ninguém a responde.

**Método:**
1. Usar um simulador de hardware analógico com modelo de deriva e ruído de dispositivo (o AIHWKIT da IBM é a referência aberta; CrossSim, do Sandia, é alternativa).
2. Estabelecer acurácia inicial de um modelo mapeado no hardware simulado.
3. Aplicar modelo de deriva ao longo de janelas de tempo simulado, medindo degradação da acurácia.
4. Definir um limiar de acurácia aceitável e determinar a frequência de recalibração necessária para mantê-lo.
5. Estimar o custo energético de cada recalibração e computar a energia amortizada por inferência ao longo da vida útil.

**Fontes de erro a declarar:** o resultado é inteiramente dependente do modelo de deriva adotado — declarar explicitamente qual modelo, qual parametrização, e qual a base empírica dela; sensibilidade do resultado a essa parametrização deve ser reportada como análise de sensibilidade, não como número único.

**Critério de sucesso (gate para E4):**
- [ ] O modelo de deriva usado está declarado, referenciado e parametrizado de forma reproduzível.
- [ ] Análise de sensibilidade executada: o resultado é reportado como faixa sob variação dos parâmetros de deriva, não como valor pontual.
- [ ] A natureza simulada está declarada em destaque, e as condições sob as quais a simulação seria falseada por medição real estão listadas.
- [ ] Artefato permite que terceiro rode a mesma simulação e obtenha os mesmos números.

---

### E4 — Energia ajustada por ciclo de trabalho

**Pergunta:** em deployment realista, com o acelerador ocioso a maior parte do tempo, qual a energia real por inferência útil?

**Por que importa:** TOPS/W e "watts sob carga" medem o pico. Em produção, hardware de inferência passa boa parte do tempo ocioso — mas continua consumindo. A energia amortizada pelo uso efetivo pode ser ordens de grandeza pior que o número de folheto, e essa é a conta que o pagador da fatura enfrenta.

**Hipótese pré-registrada:** estimar, antes de medir, o fator de degradação entre energia de pico e energia amortizada em ciclos de trabalho de 5%, 20% e 50%.

**Método:**
1. Definir perfis de carga realistas (por exemplo: 5%, 20%, 50% de utilização) usando traço de chegada de requisição, não carga sintética contínua.
2. Executar cada perfil por período longo o suficiente para capturar comportamento de estado estacionário, incluindo os períodos ociosos.
3. Medir energia total consumida na janela inteira (carga + ocioso) e dividir pelo número de inferências úteis entregues.
4. Comparar com a energia por inferência medida sob carga saturada (que é o número usualmente publicado).

**Fontes de erro a declarar:** o resultado é sensível ao perfil de chegada escolhido — declarar o traço usado e justificar sua representatividade; políticas de gerenciamento de energia do hardware (estados de baixo consumo, suspensão) afetam fortemente o resultado e devem ser registradas.

**Critério de sucesso (fechamento da série):**
- [ ] Perfis de carga documentados e reproduzíveis.
- [ ] Energia ociosa medida e reportada separadamente da energia de carga.
- [ ] A razão entre energia amortizada e energia de pico reportada com incerteza para cada perfil.
- [ ] Metodologia aplicável, sem alteração, a hardware de fabricantes distintos — este é o teste de neutralidade: se o método só funciona no hardware que você tem, ele não serve como instrumento independente.

---

## 4. Sequenciamento e dependências

```
E1 (decomposição)  →  valida a instrumentação de energia por fase
   └── gate: consistência interna (partes somam o todo)
        ↓
E2 (iso-acurácia)  →  reusa a instrumentação de E1, adiciona eixo de acurácia
   └── gate: reprodutibilidade da fronteira em dias diferentes
        ↓
E3 (deriva)        →  MUDA DE NATUREZA: simulação, não medição
   └── gate: análise de sensibilidade + declaração de natureza simulada
        ↓
E4 (ciclo de trabalho) → volta a medição real, escala temporal longa
   └── gate: neutralidade (método funciona em hardware de terceiros)
```

**Observação adversarial sobre a ordem:** E3 é metodologicamente descolado dos outros três (simulação versus medição). Mantido na posição pedida, mas registra-se que, se o objetivo for publicar uma série coesa de *medição independente*, E3 pode fazer mais sentido como artigo separado, de natureza declaradamente teórica, em vez de terceiro item de uma série empírica. Decisão fica com o condutor da pesquisa.

---

## 5. Formato do artefato publicável (por experimento)

Cada experimento gera um pacote autocontido:

1. **Pré-registro** — hipótese e critério de sucesso, com timestamp de commit anterior à primeira coleta.
2. **Código de coleta e análise** — versionado, com dependências fixadas.
3. **Dado bruto** — não apenas o agregado. Séries completas, incluindo as descartadas (com o motivo do descarte).
4. **Procedimento** — passo a passo suficiente para um terceiro replicar sem contato com o autor.
5. **Ambiente** — versão de kernel, driver, firmware, modelo, engine, e especificação exata do hardware.
6. **Seção de limitações** — o que o experimento **não** mostra, escrito antes da conclusão, não depois.
7. **Resultado** — com incerteza, dispersão, e número de repetições em toda grandeza reportada.

---

## 6. Falhas que invalidam um experimento (lista de exclusão)

Um experimento é descartado, não corrigido, se:

1. A hipótese foi alterada depois do início da coleta.
2. Séries foram descartadas sem motivo registrado *antes* de olhar o resultado.
3. Baseline pré e pós divergiram além de 5% e a série foi mantida.
4. A soma das partes não reconstrói o todo em E1 e o resultado foi publicado assim mesmo.
5. Alguma grandeza foi reportada sem dispersão.
6. Alguma limitação conhecida do instrumento não foi declarada na seção de métodos.

---

## 7. Nota final

A única coisa que dá valor a este trabalho é o fato de não haver nada a ganhar com um resultado específico. No momento em que uma medição for ajustada — por ansiedade, por vontade de que o número seja interessante, por conveniência de narrativa — o ativo inteiro se perde, e não se recupera.

Um resultado morno, medido com rigor e reproduzível por qualquer um, vale mais do que um resultado espetacular que ninguém consegue repetir.
