"""
GT-02 — Mock vLLM com /metrics no formato Prometheus real
Reproduz o cenário exato do GT-00b (carga concorrente, tokens idênticos em rajada),
mas agora expondo o contador via /metrics em vez de log de stdout.

Instalar: pip install fastapi uvicorn --break-system-packages
Rodar:    python3 mock_vllm_metrics.py
"""

import random
import threading
import time

from fastapi import FastAPI, Response
from fastapi.responses import PlainTextResponse
import uvicorn

app = FastAPI()

# Contador cumulativo global — simula o comportamento real de um counter Prometheus:
# nunca zera, só cresce. É exatamente o que o vLLM real faz com generation_tokens_total.
_lock = threading.Lock()
_total_tokens = 0
_total_prompt_tokens = 0

MODEL_NAME = "mock-qwen-0.5b"


@app.post("/v1/chat/completions")
def chat_completions():
    """
    Simula uma chamada de geração. Sempre retorna exatamente 50 tokens de completion,
    reproduzindo o cenário do GT-00b que quebrou o parser antigo: múltiplas requisições
    concorrentes gerando o MESMO número de tokens, quase no mesmo milissegundo.
    """
    global _total_tokens, _total_prompt_tokens

    completion_tokens = 50  # fixo de propósito — é o que quebrou o dedup por valor/linha
    prompt_tokens = random.randint(10, 30)

    # Simula tempo de "geração" real, pequeno o suficiente para permitir alta concorrência
    time.sleep(0.05)

    with _lock:
        _total_tokens += completion_tokens
        _total_prompt_tokens += prompt_tokens

    return {
        "id": f"cmpl-{random.randint(100000,999999)}",
        "object": "chat.completion",
        "model": MODEL_NAME,
        "choices": [{"message": {"role": "assistant", "content": "mock response"}}],
        "usage": {
            "prompt_tokens": prompt_tokens,
            "completion_tokens": completion_tokens,
            "total_tokens": prompt_tokens + completion_tokens,
        },
    }


@app.get("/metrics")
def metrics():
    """
    Expõe o contador cumulativo no formato Prometheus, espelhando o nome de métrica
    real do vLLM: vllm:generation_tokens_total
    """
    with _lock:
        tokens = _total_tokens
        prompt = _total_prompt_tokens

    body = (
        f'# HELP vllm:generation_tokens_total Number of generation tokens.\n'
        f'# TYPE vllm:generation_tokens_total counter\n'
        f'vllm:generation_tokens_total{{model_name="{MODEL_NAME}"}} {tokens}.0\n'
        f'# HELP vllm:prompt_tokens_total Number of prefill tokens.\n'
        f'# TYPE vllm:prompt_tokens_total counter\n'
        f'vllm:prompt_tokens_total{{model_name="{MODEL_NAME}"}} {prompt}.0\n'
    )
    return PlainTextResponse(content=body)


@app.get("/health")
def health():
    return {"status": "ok"}


@app.get("/debug/total")
def debug_total():
    """Endpoint auxiliar só para o script de validação conferir o total real via API."""
    with _lock:
        return {"completion_tokens_total": _total_tokens}


if __name__ == "__main__":
    print(f"[MOCK] Subindo mock vLLM com /metrics em http://0.0.0.0:8000")
    print(f"[MOCK] POST /v1/chat/completions -> gera exatamente 50 tokens por request")
    print(f"[MOCK] GET  /metrics             -> contador cumulativo Prometheus")
    print(f"[MOCK] GET  /debug/total         -> total real via API (fonte de verdade p/ comparar)")
    uvicorn.run(app, host="0.0.0.0", port=8000, log_level="warning")
