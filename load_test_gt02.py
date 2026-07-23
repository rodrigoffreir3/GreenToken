"""
GT-02 — Validação de regressão do GT-00b via /metrics
Dispara carga concorrente idêntica ao GT-00b original (tokens fixos, alta concorrência)
e compara o total real (via API) contra o total lido pelo agente via -token-source prometheus.

Uso:
    1. Sobe o mock:      python3 mock_vllm_metrics.py
    2. Sobe o agent:     ./agent -token-source prometheus -metrics-url http://localhost:8000/metrics \
                              -metrics-name vllm:generation_tokens_total -pid <PID_do_mock> -interval 2
    3. Roda esta carga:  python3 load_test_gt02.py
    4. Compara os logs [TELEMETRIA] do agent com o "Total real (API)" impresso aqui.
       Erro < 2% = GT-00b corrigido. GO para v0.1.0.

Instalar: pip install requests --break-system-packages
"""

import concurrent.futures
import time

import requests

BASE_URL = "http://localhost:8000"
N_REQUESTS = 20
CONCURRENCY = 20  # dispara tudo de uma vez -> reproduz a rajada que quebrou o parser antigo


def fire_request(i):
    r = requests.post(f"{BASE_URL}/v1/chat/completions", timeout=10)
    r.raise_for_status()
    return r.json()["usage"]["completion_tokens"]


def main():
    print(f"[LOAD] Disparando {N_REQUESTS} requests com concorrência {CONCURRENCY}...")
    print(f"[LOAD] Cada request gera exatamente 50 tokens — mesmo cenário do GT-00b.\n")

    t0 = time.time()
    with concurrent.futures.ThreadPoolExecutor(max_workers=CONCURRENCY) as executor:
        results = list(executor.map(fire_request, range(N_REQUESTS)))
    duration = time.time() - t0

    total_from_responses = sum(results)
    expected = N_REQUESTS * 50

    # Fonte de verdade independente: pergunta pro mock quanto ele registrou de verdade
    debug = requests.get(f"{BASE_URL}/debug/total").json()
    total_from_server_state = debug["completion_tokens_total"]

    print(f"[LOAD] Concluído em {duration:.2f}s")
    print(f"[LOAD] Soma dos completion_tokens das respostas da API : {total_from_responses}")
    print(f"[LOAD] Total real registrado no servidor (debug/total) : {total_from_server_state}")
    print(f"[LOAD] Esperado (N_REQUESTS x 50)                      : {expected}")

    if total_from_responses != expected or total_from_server_state != expected:
        print("\n[LOAD] ⚠️  Inconsistência na própria carga de teste — investigar antes de validar o agent.")
        return

    print(f"\n[LOAD] ✅ Carga de teste consistente: {expected} tokens gerados de verdade.")
    print(f"[LOAD] Agora verifique o log do agent (-token-source prometheus).")
    print(f"[LOAD] Ele deve reportar, na(s) janela(s) de coleta cobrindo este período, um total")
    print(f"[LOAD] de tokens somado próximo de {expected} (erro < 2% = {expected*0.02:.0f} tokens de tolerância).")
    print(f"\n[LOAD] Cheque também via curl direto:")
    print(f"       curl -s {BASE_URL}/metrics | grep generation_tokens_total")


if __name__ == "__main__":
    main()
