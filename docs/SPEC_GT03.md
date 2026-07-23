# SPEC GT-03 — Release Engineering: Primeiro Release Público `v0.1.0`

**Versão:** 1.0
**Status:** Aberto — pronto para execução
**Repositório:** `github.com/rodrigoffreir3/GreenToken`
**Executor:** Antigravity squad
**Pré-requisito:** GT-02 mergeado e validado (commit `10a679b` ou posterior — H3 confirmado com 0% de erro via `/metrics`)
**Referência de padrão:** pipeline de release do SyscallCage (SCC)

---

## 0. Objetivo e escopo

Esta spec **não adiciona nenhuma funcionalidade de produto**. O GreenToken já está tecnicamente validado (GT-00 → GT-02). O que falta é o aparato que transforma "código que funciona no meu ambiente" em "software que qualquer pessoa instala e usa com um comando". Escopo estritamente:

1. Housekeeping (specs duplicados, organização de `docs/`).
2. Pipeline de build multi-plataforma via GitHub Actions.
3. Instalador de um comando com verificação de integridade (SHA256).
4. Subcomando `doctor` de diagnóstico do ambiente.
5. CHANGELOG e seção de instalação no README.
6. Tag `v0.1.0` e publicação da release no GitHub.

Nenhuma task desta spec altera lógica de agent, collector, proto ou agregação. Se durante a execução aparecer necessidade de mudar código de produto, isso é desvio de escopo e deve ser interrompido e escalado, não decidido silenciosamente pela squad.

---

## 1. Princípios invioláveis (herdados)

1. **Observa. Nunca atua.** O `doctor` é diagnóstico puro — lê, nunca escreve nem corrige automaticamente sem confirmação do usuário.
2. **Não quebrar o que funciona.** As flags e comportamento do `agent`, `collector` e `cmd/greentoken` existentes permanecem idênticos. Esta spec empacota, não reescreve.
3. **Zero-trust no instalador.** Todo binário baixado é verificado por checksum antes de ser executado ou movido para PATH. Nenhum `curl | bash` sem verificação.
4. **Complexidade justificada.** Usa GitHub Actions nativo e ferramentas padrão do ecossistema Go. Não introduzir sistema de build customizado quando `go build` com matriz de OS/ARCH resolve.
5. **Testes em tudo que for testável.** O `doctor` tem teste unitário por checagem. O instalador é validado manualmente em pelo menos duas plataformas (checklist na seção 7).

---

## 2. Particularidade técnica que esta spec precisa respeitar: cgo e a flag `gpu`

Diferente do SCC (Rust, binário estático simples), o GreenToken tem uma bifurcação real de build:

- **Build default (stub):** `go build ./agent` — sem cgo, sem dependência de NVML, roda em qualquer Linux. É o caminho para quem não tem GPU NVIDIA.
- **Build com suporte a GPU:** `go build -tags gpu ./agent` — requer cgo habilitado e a lib NVML disponível no ambiente de build (não em runtime — o binário resultante ainda faz `dlopen` da NVML em tempo de execução, com fallback gracioso se ausente).

Isso significa que a matriz de release tem uma dimensão a mais que builds Go triviais: **arquitetura × variante (stub/gpu)**. Publicar só o binário stub seria enganoso (usuário com GPU não teria telemetria de GPU); publicar só o binário `gpu` quebraria em ambientes sem `libnvidia-ml.so` disponível no build (CI precisa do dev package da NVIDIA para linkar, mesmo que o binário resultante degrade graciosamente em runtime).

**Decisão desta spec:** publicar as duas variantes com sufixo explícito no nome do artefato (`greentoken-agent-linux-amd64` e `greentoken-agent-linux-amd64-gpu`), documentando claramente qual usar. Isso é mais simples e honesto do que uma única build "universal" com detecção mágica de runtime.

---

## 3. Especificação de componentes

### 3.1 Housekeeping — `docs/` como única fonte

Estado atual: `GREENTOKEN_SPEC_GT01.md` e `GREENTOKEN_SPEC_GT02.md` existem duplicados na raiz e em `docs/`. Padronizar:
- Specs vivem exclusivamente em `docs/` (`docs/SPEC_GT00.md`, `docs/SPEC_GT01.md`, `docs/SPEC_GT02.md`, `docs/SPEC_GT03.md` — este arquivo).
- Raiz do repo mantém apenas `README.md`, `LICENSE`, `CHANGELOG.md`, arquivos de build (`Makefile`, `go.work`) e o `.gitignore`.

### 3.2 `cmd/greentoken doctor` — NEW

Novo subcomando, terceiro ao lado de `serve` e `report`. Propósito: diagnosticar o ambiente antes do usuário rodar `serve` às cegas e ficar confuso com telemetria zerada.

Checagens, cada uma reportando `OK`, `AVISO` ou `FALHA` com explicação de uma linha:

| Checagem | Como | Resultado se ausente |
|---|---|---|
| RAPL disponível | Verifica existência de `/sys/class/powercap/intel-rapl:0` | AVISO — "W_cpu/W_dram serão 0. Normal em VM ou CPU não-Intel." |
| eBPF/kernel | Verifica versão do kernel ≥ 4.x e existência de `/sys/kernel/debug/tracing` ou capacidade de carregar programa BPF simples | AVISO — "Correlação por PID via eBPF indisponível; fallback para scanner `/proc`." |
| NVML/GPU | Tenta `dlopen` de `libnvidia-ml.so`; se o binário foi compilado sem a tag `gpu`, reporta isso explicitamente | AVISO — "Binário sem suporte a GPU (build stub) ou driver NVIDIA ausente. W_gpu será 0." |
| Permissões | Verifica se está rodando como root ou com as capabilities necessárias (`CAP_BPF`, `CAP_PERFMON` ou root simples) | FALHA — "eBPF e RAPL exigem privilégios elevados. Rode com sudo ou configure capabilities." |
| Conectividade com collector | Se `-collector` foi passado, tenta um dial gRPC rápido (timeout 2s) | AVISO — "Collector inalcançável em {addr}. Agent vai enfileirar localmente até reconectar (se aplicável) ou falhar ao enviar." |
| Fonte de tokens | Se `-token-source prometheus`, faz um GET rápido no `-metrics-url` e verifica se a métrica `-metrics-name` aparece na resposta | AVISO — "Métrica '{nome}' não encontrada em {url}. Verifique se o engine de inferência expõe /metrics." |

Saída exemplo:
```
$ greentoken doctor --metrics-url http://localhost:8000/metrics --metrics-name vllm:generation_tokens_total

GreenToken Doctor — diagnóstico de ambiente
─────────────────────────────────────────────
[OK]    RAPL disponível em /sys/class/powercap/intel-rapl:0
[AVISO] eBPF indisponível (kernel sem debugfs montado) — fallback /proc ativo
[OK]    NVML disponível, binário compilado com suporte a GPU (tag: gpu)
[FALHA] Permissões insuficientes — rode com sudo
[OK]    Métrica 'vllm:generation_tokens_total' encontrada em http://localhost:8000/metrics

Resumo: 3 OK, 1 aviso, 1 falha. Corrija a falha antes de rodar 'serve' em produção.
```

- **Aceite:** cada checagem é uma função isolada e testável, retornando um struct `{Status, Message}`. Teste unitário simula cada cenário (arquivo presente/ausente, servidor mock respondendo/não) via `httptest` e diretórios temporários — sem depender de hardware real.
- Nunca corrige nada automaticamente. É leitura, conforme princípio 1.

### 3.3 Pipeline de release — `.github/workflows/release.yml` — NEW

Dispara em push de tag `v*.*.*`. Matriz:

```yaml
strategy:
  matrix:
    include:
      - os: linux, arch: amd64, variant: stub, tags: ""
      - os: linux, arch: amd64, variant: gpu,  tags: "gpu"
      - os: linux, arch: arm64, variant: stub, tags: ""
```

Nota: a variante `gpu` só é publicada para `amd64` nesta primeira release — ARM64 com GPU NVIDIA (Jetson) é caso real mas fora do escopo do `v0.1.0`; documentar como limitação conhecida, não silenciar.

Passos do job, por combinação da matriz:
1. Checkout, setup Go 1.24.
2. Se `variant == gpu`: instalar dev headers da NVIDIA necessários para linkar (`apt-get install -y libnvidia-ml-dev` ou equivalente disponível no runner) e habilitar `CGO_ENABLED=1`.
3. Se `variant == stub`: `CGO_ENABLED=0` (binário estático, sem dependência de libc dinâmica — mais fácil de distribuir).
4. Build dos três binários (`agent`, `collector`, `cmd/greentoken`) com `GOOS`/`GOARCH` da matriz e a tag `gpu` quando aplicável.
5. Gerar SHA256 de cada artefato: `sha256sum greentoken-agent-linux-amd64 > greentoken-agent-linux-amd64.sha256`.
6. Empacotar em `.tar.gz` por combinação, nome padrão: `greentoken_{version}_{os}_{arch}{_gpu?}.tar.gz`.
7. Upload de todos os artefatos + checksums para a GitHub Release da tag, usando a action oficial de release do GitHub (não script customizado de upload).

- **Aceite:** um push de tag `v0.1.0-rc1` (tag de teste, apagável) produz uma release com todos os artefatos e arquivos `.sha256` correspondentes, visível na aba Releases do repo.

### 3.4 Instalador — `deploy/install.sh` — MODIFY

Estado atual: existe mas sem verificação de checksum. Adicionar:
1. Detectar OS/ARCH do usuário (`uname -s`, `uname -m`).
2. Perguntar (ou aceitar flag `--gpu`) se deve baixar a variante `gpu` ou `stub`. Default: `stub` (mais seguro, sempre funciona).
3. Baixar o binário e o `.sha256` correspondente da última release (via GitHub API, não hardcoded).
4. **Validar o checksum antes de mover para PATH.** Se não bater, abortar com mensagem clara — nunca instalar binário não verificado.
5. Instalar em `/usr/local/bin/greentoken` (ou local configurável via `--prefix`).
6. Ao final, sugerir rodar `greentoken doctor` para validar o ambiente.

```bash
# Exemplo de uso alvo
curl -fsSL https://raw.githubusercontent.com/rodrigoffreir3/GreenToken/main/deploy/install.sh | sh
```

- **Aceite:** script testado manualmente baixando um artefato real da release de teste, validando que checksum incorreto (simulado alterando um byte do arquivo local antes da comparação) faz o script abortar sem instalar.

### 3.5 CHANGELOG e README — MODIFY

`CHANGELOG.md` segue formato Keep a Changelog. Entrada para `v0.1.0`:

```markdown
## [0.1.0] - 2026-07-XX

### Added
- Agent: coleta de energia via RAPL (CPU/DRAM) e NVML (GPU) com fallback gracioso.
- Agent: correlação por PID via eBPF (sched_switch) com fallback para scanner /proc.
- Agent: três fontes de tokens configuráveis via -token-source: prometheus (padrão, fonte de verdade), logsniffer (fallback best-effort), none.
- Collector: agregação de eventos de energia por PID, cálculo de custo-por-token.
- Collector: exporter Prometheus (/metrics) com métricas greentoken_watts_cpu, _watts_dram, _watts_gpu, _tokens_total, _cost_per_token, _joules_per_request.
- CLI unificada greentoken com subcomandos serve, report, doctor.
- Validado: correlação PID-GPU via NVML (H1), delta de watts sob carga (H2), contagem de tokens via /metrics com 0% de erro sob carga concorrente (H3), estabilidade de custo/token entre execuções (H4).

### Known Limitations
- logsniffer (fallback de contagem de tokens via log) tem viés de subcontagem sob rajadas de gerações idênticas; usar apenas quando o engine não expõe /metrics.
- Atribuição de energia multi-workload no mesmo host é proporcional ao tempo de CPU, não medição exata por processo.
- Build com suporte a GPU (-tags gpu) disponível apenas para linux/amd64 nesta versão.
```

README ganha seção "Installation" com o comando `curl | sh`, e "Verifying your download" explicando a checagem manual de SHA256 para quem preferir não confiar no instalador automático.

---

## 4. TASK BUDGET

Convenção: **S** ≤15k tokens, **M** ~40k, **L** ~70k. Total estimado: ~180k. Ordem topológica.

### T-03.1 [S] — Housekeeping de specs
Mover `GREENTOKEN_SPEC_GT01.md` e `GREENTOKEN_SPEC_GT02.md` da raiz para `docs/`, renomeando para `docs/SPEC_GT01.md` / `docs/SPEC_GT02.md` se ainda não estiverem lá com esse nome. Adicionar este arquivo como `docs/SPEC_GT03.md`. Remover duplicatas da raiz.
- **Aceite:** `ls *.md` na raiz mostra apenas `README.md` e `CHANGELOG.md` (após T-03.5); `docs/` contém os quatro specs.

### T-03.2 [M] — `cmd/greentoken doctor`
Implementar o subcomando conforme 3.2, com as seis checagens isoladas em funções testáveis.
- **Aceite:** `greentoken doctor` roda e imprime as seis checagens com status correto para o ambiente; testes unitários cobrem cada checagem via mock/tmpdir/httptest, sem depender de hardware real; nenhuma checagem corrige algo automaticamente (grep confirma ausência de escrita em /sys ou /proc dentro do doctor).

### T-03.3 [L] — Pipeline `.github/workflows/release.yml`
Implementar a matriz de build conforme 3.3, incluindo geração de checksum e upload para GitHub Release.
- **Aceite:** push de tag de teste (v0.1.0-rc1) gera release com os artefatos esperados: greentoken-agent-linux-amd64.tar.gz + .sha256, greentoken-agent-linux-amd64-gpu.tar.gz + .sha256, greentoken-agent-linux-arm64.tar.gz + .sha256, mais os binários collector e cmd/greentoken (CLI) para cada combinação relevante. Delete a tag/release de teste após validar.

### T-03.4 [M] — `deploy/install.sh` com verificação SHA256
Adaptar o instalador conforme 3.4.
- **Aceite:** rodar o instalador contra a release de teste do T-03.3 e confirmar que instala corretamente; simular corrupção do binário baixado (alterar um byte) e confirmar que o script aborta antes de instalar, com mensagem de erro clara.

### T-03.5 [S] — CHANGELOG e README
Criar CHANGELOG.md com a entrada v0.1.0 da seção 3.5. Adicionar seções "Installation" e "Verifying your download" ao README.
- **Aceite:** CHANGELOG segue formato Keep a Changelog; README tem o comando de instalação testável copy-paste.

### T-03.6 [S] — Tag e release final v0.1.0
Após T-03.1 a T-03.5 mergeados e o pipeline validado com a tag de teste (já apagada), criar a tag real:
```bash
git tag -a v0.1.0 -m "GreenToken v0.1.0 — first public release

Validated: PID-to-GPU attribution (NVML), token counting via /metrics
with 0% error under concurrent load, cost-per-token stability across runs."
git push origin v0.1.0
```
- **Aceite:** GitHub Actions dispara automaticamente, release v0.1.0 aparece publicada na aba Releases com todos os artefatos e o changelog correspondente.

### Gate de validação final do GT-03

- [ ] `curl -fsSL <install.sh> | sh` funciona de ponta a ponta numa máquina limpa (VM ou container Ubuntu novo).
- [ ] `greentoken doctor` roda sem privilégio root e reporta corretamente as FALHAs de permissão (não trava, não dá panic).
- [ ] Checksum incorrupto verificado com sucesso; checksum corrompido (teste manual) barra a instalação.
- [ ] Nenhum arquivo de spec duplicado na raiz.
- [ ] `git tag -l` mostra v0.1.0.
- [ ] README tem link funcional para a release e para o CHANGELOG.

---

## 5. Riscos e decisões em aberto

1. **Build gpu no CI depende de o runner do GitHub Actions ter os headers de desenvolvimento da NVIDIA disponíveis para instalar.** Se o runner padrão (ubuntu-latest) não permitir apt-get install libnvidia-ml-dev sem GPU física presente, pode ser necessário usar um runner self-hosted com GPU ou aceitar que o binário gpu seja compilado via cross-linking sem executar testes de GPU real no CI (os testes de GPU já rodam contra o stub, conforme GT-01; isso é aceitável — o CI valida que compila e linka, não que a GPU funciona).

2. **Distribuição via curl | sh tem uma tensão inerente com "zero-trust"** — por mais que o script valide checksum do binário, o próprio script precisa ser baixado e executado confiando na conexão HTTPS e no domínio raw.githubusercontent.com. Documentar no README a alternativa manual (baixar o .tar.gz, verificar o SHA256 manualmente, extrair) para o usuário que prefere não confiar em pipe-to-shell.

3. **Versionamento semântico a partir daqui.** v0.1.0 sinaliza "funcional mas não API-estável". Mudanças de flag (como o rename futuro de alguma métrica) devem esperar v1.0.0 ou vir documentadas como breaking change no CHANGELOG. Registrar esse compromisso agora evita ambiguidade em releases futuras.
