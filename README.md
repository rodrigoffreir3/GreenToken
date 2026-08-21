# GreenToken

[![DOI](https://zenodo.org/badge/DOI/10.5281/zenodo.22036311.svg)](https://doi.org/10.5281/zenodo.22036311)

**O medidor de energia que revela quanto cada token da sua IA realmente custa em Watts, Joules e dólares.**

> 🌐 **Página oficial e documentação completa:** [rodrigofreire.pages.dev/greentoken](https://rodrigofreire.pages.dev/greentoken)

**O GreenToken é um projeto de observabilidade de energia e FinOps de IA em constante evolução. Fique à vontade para contribuir e deixar sugestões ou comentários construtivos na página oficial do projeto. Grato pelo apoio!**

---

## 🔬 Research & Metodologia (GT-M)

A metodologia de medição energética independente (GT-M) está publicada com dados brutos e código de coleta reprodutível — decomposição por fase (E1), fronteira de Pareto acurácia-energia (E2) e energia amortizada por ciclo de trabalho (E4), com apêndice sobre deriva simulada (E3):

> Freire, R. (2026). *GT-M — Medição Energética Independente para Inferência de IA: Decomposição por Fase, Fronteira de Pareto Acurácia-Energia e Energia Amortizada por Ciclo de Trabalho (com apêndice sobre deriva simulada)*. Zenodo.  
> https://doi.org/10.5281/zenodo.22036311

---

## 🚀 Chegando em breve (Roadmap público)

O GreenToken já funciona hoje de ponta a ponta em Linux com CPU, DRAM, GPUs NVIDIA (NVML) e kernel eBPF — e o nosso roadmap público garante transparência total do que vem por aí:

- **Suporte a Multi-GPU e MIG (NVIDIA Multi-Instance GPU)** — isole e atribua o custo exato de energia quando múltiplos modelos dividem a mesma placa gráfica. (GT-04)
- **Detecção de Anomalias de Custo** — alertas automáticos se um modelo começar a gastar mais energia por token do que a linha de base esperada. (GT-05)
- **Gêmeo Digital (Digital Twin de Inferência)** — simule o custo mensal de energia de um modelo antes de colocá-lo em produção. (GT-06)

---

## O problema

Imagine receber a conta de luz da sua empresa no fim do mês: ela diz apenas o valor total do prédio. Você sabe que a conta veio alta, mas **não sabe qual sala deixou o ar-condicionado ligado no máximo**.

Na inferência de IA (LLMs como vLLM, Ollama, llama.cpp), acontece exatamente isso. Toda equipe de FinOps acompanha a fatura geral de nuvem (*"pagamos $5.000/mês por este servidor de GPU"*), mas **quase nenhuma sabe quanto custa cada resposta do modelo**.

Se você roda 3 modelos na mesma máquina, qual deles é o gastador? Quanto custou gerar aquela resposta de 1.000 tokens?

Ferramentas tradicionais (Prometheus, DCGM, OpenTelemetry) medem apenas a energia total do computador inteiro. Nenhuma delas consegue ligar os pontos entre **qual processo (PID) gastou qual fatia de Watts na CPU/GPU e quantos tokens ele gerou**. Essa resposta exata é onde o GreenToken atua.

---

## O que o GreenToken faz

Ele roda ao lado do seu servidor de inferência de IA e responde a uma pergunta direta:

> **Quanto custa cada token em Watts, Joules e Dólares?**

A matemática é simples e direta:
```
custo_por_token = (Watts_CPU + Watts_DRAM + Watts_GPU) × tempo_inferencia / tokens_gerados
```

O GreenToken **apenas observa e mede**. Ele nunca desacelera seu modelo, nunca limita o processador e nunca coloca risco no seu SLA de produção.

---

## Onde ele atua

- **Na CPU e Memória (DRAM):** Lê o consumo real de energia em Watts diretamente nos registradores do processador via Intel/AMD RAPL (Running Average Power Limit).
- **Na GPU (NVIDIA):** Lê o consumo em Watts da placa de vídeo em tempo real via NVML (NVIDIA Management Library).
- **No Kernel do Linux:** Acompanha exatamente quais threads pertencem a qual processo usando **eBPF** (`sched_switch`), medindo o tempo de CPU com precisão de microssegundos.
- **No Servidor de IA:** Lê a contagem oficial de tokens diretamente do endpoint `/metrics` do seu engine (ex: vLLM).

---

## Quando ele age

Continuamente, em tempo real, coletando amostras de energia e tokens a cada poucos segundos sem impactar o desempenho da sua aplicação.

---

## Por que ele é diferente

1. **Granularidade por Processo (PID):** Não dá um "palpite" sobre o servidor inteiro. Ele descobre exatamente quanto do consumo de energia pertence a cada modelo de IA rodando na máquina.
2. **Zero Risco de Produção (Observa, Nunca Atua):** Ao contrário de ferramentas de economia que tentam desacelerar o processador e arriscam estourar o tempo de resposta (SLA), o GreenToken é 100% passivo.
3. **Degradação Graciosa:** Se sua máquina não tiver GPU, ele mede a CPU. Se não tiver eBPF, usa o scanner `/proc`. O GreenToken **nunca trava seu sistema**.
4. **Nativo do Prometheus:** Exporta métricas no formato padrão da indústria. Você conecta no seu Grafana atual sem instalar nada complicado.

---

## 🏗️ Arquitetura Visual

```
┌─────────────────────────────────────────────┐
│  Servidor Linux (Inference Host)            │
│                                             │
│  ┌──────────┐  ┌──────────┐  ┌──────────┐  │
│  │ RAPL     │  │ NVML     │  │ eBPF     │  │
│  │ CPU+DRAM │  │ GPU      │  │ sched    │  │
│  │ Watts    │  │ Watts    │  │ janelas  │  │
│  └────┬─────┘  └────┬─────┘  └────┬─────┘  │
│       └─────────────┴─────────────┘         │
│                     │                       │
│              ┌──────▼──────┐                │
│              │  GreenToken │                │
│              │    Agent    │                │
│              └──────┬──────┘                │
└─────────────────────┼───────────────────────┘
                      │ Stream gRPC
              ┌───────▼───────┐
              │  GreenToken   │
              │  Collector    │
              └───────┬───────┘
                      │
          ┌───────────┼───────────┐
          ▼           ▼           ▼
     /metrics      Painel      Relatório
   (Prometheus)   Grafana     CLI / Custo
```

---

## Como instalar

### Instalação em um comando só (Recomendado)

Baixa os binários já compilados da versão mais recente com verificação estrita de hash SHA256 (Zero-Trust — nada é compilado na sua máquina):

```bash
curl -fsSL https://raw.githubusercontent.com/rodrigoffreir3/GreenToken/main/deploy/install.sh | sh
```

Para ambientes com placa de vídeo NVIDIA (suporte a GPU):
```bash
curl -fsSL https://raw.githubusercontent.com/rodrigoffreir3/GreenToken/main/deploy/install.sh | sh -s -- --gpu
```

Isso instala em `/usr/local/bin` três utilitários:
- `greentoken-agent` (o agente de coleta de energia e tokens)
- `greentoken-collector` (o agregador de métricas e servidor Prometheus)
- `greentoken` (a CLI de diagnóstico e relatórios)

---

### Instalação Manual e Verificação de Segurança (Zero-Trust)

Se você prefere não rodar scripts de instalador via pipe (`curl | sh`), pode baixar e conferir o checksum manualmente:

```bash
# 1. Baixe o pacote e o arquivo de checksum oficial
VERSION="v0.1.0"
curl -LO "https://github.com/rodrigoffreir3/GreenToken/releases/download/${VERSION}/greentoken_${VERSION}_linux_amd64.tar.gz"
curl -LO "https://github.com/rodrigoffreir3/GreenToken/releases/download/${VERSION}/greentoken_${VERSION}_linux_amd64.tar.gz.sha256"

# 2. Confirme que o arquivo baixado é 100% autêntico e não foi alterado
sha256sum -c "greentoken_${VERSION}_linux_amd64.tar.gz.sha256"

# 3. Extraia e mova para o diretório de binários do seu sistema
tar -xzf "greentoken_${VERSION}_linux_amd64.tar.gz"
sudo mv greentoken* /usr/local/bin/
```

---

## Como usar (Passo a Passo)

### 1. Teste o seu ambiente primeiro com o `greentoken doctor`

Antes de colocar qualquer coisa para rodar em produção, veja se seu servidor tem suporte a medição de energia e se o seu engine de IA está acessível:

```bash
greentoken doctor --metrics-url http://localhost:8000/metrics --metrics-name vllm:generation_tokens_total
```

O `doctor` analisa seu sistema e diz com clareza o que está `OK`, o que é `AVISO` e o que é `FALHA`, sem alterar absolutamente nada na sua máquina.

### 2. Inicie o Coletor de Métricas

O coletor junta os dados de energia e disponibiliza o endpoint do Prometheus (na porta `:9090` por padrão):

```bash
greentoken-collector &
```

### 3. Inicie o Agente de Coleta (com privilégio de leitura no hardware)

Apontando para o seu servidor de IA (ex: vLLM rodando na porta 8000):

```bash
sudo greentoken-agent \
  --collector localhost:50051 \
  --metrics-url http://localhost:8000/metrics \
  --metrics-name vllm:generation_tokens_total \
  --pid <PID_DO_SEU_ENGINE_DE_IA>
```

### 4. Visualize as Métricas

Você pode ler a saída direta do Prometheus:
```bash
curl localhost:9090/metrics | grep greentoken
```

Ou gerar um relatório simples no seu terminal:
```bash
greentoken report --model llama3 --since 1h
```

---

## Entendendo o comando, pedaço por pedaço

Se você não tem o costume de rodar comandos longos no terminal, um comando como este pode parecer confuso à primeira vista:

```bash
sudo greentoken-agent \
  --collector localhost:50051 \
  --metrics-url http://localhost:8000/metrics \
  --metrics-name vllm:generation_tokens_total \
  --pid 12345
```

Vamos abrir cada parte. Cada pedaço tem um papel claro:

- **`sudo`** — *"Rode este comando como administrador"*. O GreenToken precisa disso porque ele lê sensores físicos de energia da placa-mãe/processador (RAPL) e anexa rastreadores leves no kernel (eBPF), o que requer privilégios no Linux.
- **`greentoken-agent`** — O nome do programa coletor.
- **`--collector localhost:50051`** — *"Envie os dados de energia coletados para o servidor coletor que está rodando nesta porta"*. É o endereço gRPC interno do GreenToken.
- **`--metrics-url http://localhost:8000/metrics`** — *"Onde está o endpoint de métricas do seu engine de IA (vLLM, Ollama, etc.)"*. O GreenToken lê este endereço para saber quantos tokens o seu modelo já gerou.
- **`--metrics-name vllm:generation_tokens_total`** — *"Qual é o nome da métrica exata que conta os tokens dentro do endpoint"*. No vLLM, por exemplo, o nome padrão é `vllm:generation_tokens_total`.
- **`--pid 12345`** — *"Qual é o número de processo (PID) do seu engine de IA no Linux"*. O PID é como o "RG temporário" do programa rodando no sistema. Você descobre o PID do seu vLLM ou Ollama com o comando `pgrep vllm` ou `pgrep ollama`.

> 💡 **Nota importante sobre as portas:** A porta `:8000` usada no exemplo acima é a porta do **seu servidor de IA** (onde o vLLM expõe métricas). A porta `:9090` é a porta do **GreenToken Coletor** (onde o seu Grafana ou Prometheus vai se conectar para ler o custo por token). Elas são portas diferentes!

---

## Fontes de Tokens: Entendendo as opções

O GreenToken permite escolher de onde ele deve ler a quantidade de tokens gerados pela sua IA:

- **`prometheus` (Default e Recomendado):** Fonte de verdade. O agente faz requisições ao endpoint `/metrics` do seu engine de IA e lê os contadores cumulativos. É a opção mais segura, imune a perdas e validada com 0.0% de erro sob alta concorrência.
- **`logsniffer` (Fallback):** Lê os tokens diretamente do arquivo de log do seu engine. Útil para engines antigos que não possuem endpoint de métricas. *(Exemplo: `greentoken-agent --token-source logsniffer --log-file /caminho/do/log`)*.
- **`none`:** Desativa a contagem de tokens. O agente continuará medindo normalmente o consumo de energia em Watts da CPU e GPU, mas sem calcular a divisão por token.

---

## ⚡ Métricas Exportadas

| Métrica | O que ela mede | Por que importa |
|---|---|---|
| `greentoken_watts_cpu` | Watts consumidos pelo pacote da CPU (RAPL) | Mostra a energia gasta no processador principal |
| `greentoken_watts_dram` | Watts consumidos pela memória RAM/DRAM | Mede a energia gasta acessando a memória |
| `greentoken_watts_gpu` | Watts consumidos pela placa de vídeo NVIDIA (NVML) | Identifica o maior gargalo elétrico na inferência de LLMs |
| `greentoken_tokens_total` | Total acumulado de tokens gerados | Acompanha o volume real de trabalho produzido |
| `greentoken_cost_per_token` | Custo estimado em USD por token | Calcula o custo real com base na tarifa elétrica configurada |
| `greentoken_joules_per_request` | Energia em Joules consumida por requisição | Mede a eficiência energética de cada chamada de IA |

*Labels anexadas:* `workload`, `model`, `pid`, `gpu_index`

---

## 🛡️ Princípios Invioláveis de Design

1. **Observa. Nunca atua.**
   O GreenToken contém zero lógica de limitação, zero escrita em cgroups e zero chamadas para derrubar processos. Ele nunca altera o comportamento do seu servidor.
2. **Degradação Graciosa.**
   Sem GPU? `W_gpu = 0`, o agente continua. Sem RAPL? `W_cpu = 0`, o agente continua. Sem eBPF? Fallback para leitura do `/proc`. O agente nunca quebra seu ambiente.
3. **Prometheus-Native.**
   Saída no formato padrão da indústria. Conecte em qualquer stack Grafana existente sem adaptações.
4. **Granularidade Atômica.**
   Usa eBPF `sched_switch` para correlacionar o tempo de CPU com o consumo de energia por processo individual (PID) em microssegundos — e não médias genéricas da máquina.

---

## ⚙️ Variáveis de Ambiente

Você também pode configurar o GreenToken usando variáveis de ambiente:

| Variável | Valor Padrão | Descrição |
|---|---|---|
| `GT_KWH_PRICE` | `0.12` | Preço em USD por kWh (tarifa de energia elétrica) |
| `GT_GPU_INDEX` | `0` | Índice da placa de vídeo GPU (ex: `0` para a primeira GPU) |
| `GT_COLLECTOR_ADDR` | `localhost:50051` | Endereço de comunicação gRPC do coletor |

---

## 📊 Benchmark e Validação de Campo

Durante os testes de release **v0.1.0** (executados em GPUs Tesla T4 com rajadas de 20 requisições simultâneas gerando 1.000 tokens no vLLM):

| Hipótese Testada | Resultado Medido | Status |
|---|---|---|
| **H1** — Atribuição de PID na GPU via NVML | PID do vLLM detectado com sucesso | ✅ Aprovado |
| **H2** — Delta de Watts sob carga | Aumento de +14W a 17W em relação ao idle | ✅ Aprovado |
| **H3** — Precisão na contagem de tokens | **0.0% de erro** via endpoint `/metrics` | ✅ Aprovado |
| **H4** — Estabilidade de custo por token | Variância mínima de apenas **1.8%** entre execuções | ✅ Aprovado |

---

## 🔗 Relação com o Imunno System

O GreenToken compartilha DNA arquitetural com o **[Imunno System](https://github.com/rodrigoffreir3/imunno-system)** (Patente INPI nº 512025006506-0): os padrões de leitura RAPL, o transporte gRPC e os carregadores eBPF derivam da mesma base tecnológica. A missão é complementar — o Imunno defende a segurança de servidores; o GreenToken traz visibilidade financeira e energética para inferência de IA.

---

## ⚖️ Licença

**MPL 2.0 (Mozilla Public License 2.0)** — Use, modifique e aplique livremente, inclusive em ambientes comerciais. A única exigência é que modificações nos arquivos de código-fonte deste projeto permaneçam abertas sob a mesma licença.

---

## ✍️ Autor

**Rodrigo Freire** — [github.com/rodrigoffreir3](https://github.com/rodrigoffreir3)

*Desenvolvedor e pesquisador em Deep Tech. Toda a arquitetura, implementação e validação do GreenToken foram desenvolvidas pelo autor.*
