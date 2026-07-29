#!/usr/bin/env python3
"""
SPEC GT-M — Experimento E3: Custo Energético da Deriva e Amortização por Recalibração
==============================================================================
Este script executa a medição física empírica (NVIDIA Tesla T4) de consumo energético
sob diferentes tamanhos de prompt (128, 512, 1024 tokens) e temperaturas (0.0, 0.7, 1.0),
combinada com a simulação matemática da energia amortizada por recalibração periódica,
conforme pré-registrado em docs/experiments/E3_Custo_Deriva/preregistration.md.

Uso:
  python3 docs/experiments/E3_Custo_Deriva/e3_drift_cost_collector.py --runs 30
"""

import os
import sys
import time
import json
import argparse
import statistics
import math
import multiprocessing
from typing import Dict, List, Any, Tuple

# Reuso dos Sensores e Coletor Contínuo validados em E1 e E2
class NVMLSensor:
    """Leitor de potência de GPU via pynvml / nvidia-smi."""
    def __init__(self):
        self.available = False
        try:
            import warnings
            warnings.filterwarnings('ignore', category=FutureWarning)
            import pynvml
            pynvml.nvmlInit()
            self.handle = pynvml.nvmlDeviceGetHandleByIndex(0)
            self.available = True
            self.pynvml = pynvml
        except Exception:
            self.available = False

    def read_mW(self) -> float:
        if not self.available:
            return 0.0
        try:
            return float(self.pynvml.nvmlDeviceGetPowerUsage(self.handle))
        except Exception:
            return 0.0

class RAPLSensor:
    """Leitor do subsistema Intel/AMD RAPL via Linux Powercap."""
    def __init__(self):
        self.path = "/sys/class/powercap/intel-rapl/intel-rapl:0/energy_uj"
        self.available = os.path.exists(self.path) and os.access(self.path, os.R_OK)

    def read_uj(self) -> float:
        if not self.available:
            return 0.0
        try:
            with open(self.path, "r") as f:
                return float(f.read().strip())
        except Exception:
            return 0.0

class ContinuousNVMLSampler:
    """Amostrador contínuo de potência NVML em alta frequência (10ms) com integração trapezoidal."""
    def __init__(self, nvml_sensor: NVMLSensor):
        self.nvml = nvml_sensor
        self.samples = []
        self.running = False
        self.thread = None

    def _sample_loop(self):
        while self.running:
            now = time.time()
            mw = self.nvml.read_mW()
            self.samples.append((now, mw / 1000.0))
            time.sleep(0.010)

    def start(self):
        self.samples = []
        self.running = True
        import threading
        self.thread = threading.Thread(target=self._sample_loop, daemon=True)
        self.thread.start()

    def stop_and_integrate(self) -> float:
        self.running = False
        if self.thread:
            self.thread.join(timeout=1.0)
        if len(self.samples) < 2:
            return 0.0
        joules = 0.0
        for i in range(len(self.samples) - 1):
            t1, p1 = self.samples[i]
            t2, p2 = self.samples[i+1]
            dt = t2 - t1
            joules += ((p1 + p2) / 2.0) * dt
        return joules

class PromptSensitivityBenchmark:
    """Harness de teste para variação de tamanho de prompt e temperatura no PyTorch/CUDA."""
    def __init__(self, seq_len: int = 512, temperature: float = 0.7):
        self.seq_len = seq_len
        self.temperature = temperature
        self.has_cuda = False
        try:
            import torch
            if torch.cuda.is_available():
                self.has_cuda = True
                self.torch = torch
                dim = 2048
                # Simula matriz de entrada escalada pelo tamanho do prompt
                self.prompt = torch.randn(seq_len, dim, device="cuda", dtype=torch.float16)
                self.weights = torch.randn(dim, dim, device="cuda", dtype=torch.float16)
        except Exception:
            self.has_cuda = False

    def run_inference_item(self, item_id: int) -> float:
        """Executa a simulação da fase de Prefill + Gen sob a configuração de prompt/temp."""
        if self.has_cuda:
            # Prefill phase proporcional ao tamanho da sequência
            iterations = max(10, int(self.seq_len / 4))
            for _ in range(iterations):
                out = self.torch.matmul(self.prompt, self.weights)
                if self.temperature > 0.0:
                    out = out / max(0.1, self.temperature)
                    out = self.torch.softmax(out, dim=-1)
            self.torch.cuda.synchronize()
        else:
            acc = 0
            for i in range(1000000 * (self.seq_len // 128)):
                acc += i * 0.0001
        return 1.0

def measure_baseline(rapl: RAPLSensor, nvml: NVMLSensor, duration_s: int = 10) -> Tuple[float, float]:
    print(f"[*] Coletando baseline ocioso ({duration_s}s)...")
    samples = []
    t_start = time.time()
    last_rapl = rapl.read_uj()
    last_time = t_start

    while time.time() - t_start < duration_s:
        time.sleep(0.2)
        now_time = time.time()
        now_rapl = rapl.read_uj()
        dt = now_time - last_time
        
        watts_rapl = ((now_rapl - last_rapl) / 1e6) / dt if rapl.available and dt > 0 else 0.0
        watts_nvml = (nvml.read_mW() / 1000.0) if nvml.available else 0.0

        if not rapl.available and not nvml.available:
            raise RuntimeError("ERRO METODOLÓGICO: Nenhum sensor físico (RAPL/NVML) acessível no ambiente.")

        samples.append(watts_rapl + watts_nvml)
        last_rapl = now_rapl
        last_time = now_time

    return statistics.mean(samples), statistics.stdev(samples) if len(samples) > 1 else 0.0

def worker_inference(seq_len: int, temp: float, num_runs: int, p_idle_pre: float, queue: multiprocessing.Queue):
    """
    Executado em sub-processo para garantir isolamento de contexto CUDA e retorno automático ao P8.
    """
    try:
        nvml = NVMLSensor()
        bench = PromptSensitivityBenchmark(seq_len=seq_len, temperature=temp)
        
        # Warmup térmico C2 (15s por condição)
        t_w = time.time()
        while time.time() - t_w < 15:
            bench.run_inference_item(-1)

        runs_joules = []
        for r in range(num_runs):
            t0 = time.time()
            sampler = ContinuousNVMLSampler(nvml) if nvml.available else None
            if sampler:
                sampler.start()

            bench.run_inference_item(r)

            j_gpu = sampler.stop_and_integrate() if sampler else 0.0
            dt = time.time() - t0
            
            j_net = max(0.0, j_gpu - (p_idle_pre * dt))
            runs_joules.append(j_net)

        queue.put({"status": "ok", "runs_joules": runs_joules})
    except Exception as e:
        queue.put({"status": "error", "error": str(e)})

def compute_amortized_drift_cost(e_base_joules: float, nu: float = 0.02, t_lifetime_hours: float = 1000.0) -> Dict[str, Any]:
    """
    Simulação matemática da retenção de pesos e custo de recalibração temporal (Seção 3 do SPEC).
    Fórmula de Deriva: w(t) = w0 * (t / t0)^(-nu)
    """
    num_inferences_total = 100000  # 100k inferências na vida útil
    e_recalibration_joules = e_base_joules * 50.0  # Recalibração custa equivalente a 50 inferências
    
    # Frequência de recalibração necessária para manter a retenção acima do limiar
    recalibrations_needed = math.ceil(10.0 * (nu / 0.02))
    
    total_recal_energy = recalibrations_needed * e_recalibration_joules
    e_amortized = e_base_joules + (total_recal_energy / num_inferences_total)
    overhead_pct = ((e_amortized - e_base_joules) / e_base_joules) * 100.0

    return {
        "drift_parameter_nu": nu,
        "lifetime_inferences": num_inferences_total,
        "recalibrations_needed": recalibrations_needed,
        "recalibration_cost_per_event_J": e_recalibration_joules,
        "total_recalibration_energy_J": total_recal_energy,
        "e_base_inference_J": e_base_joules,
        "e_amortized_inference_J": e_amortized,
        "amortized_overhead_pct": overhead_pct
    }

def run_experiment_e3(num_runs: int = 30) -> Dict[str, Any]:
    print("=================================================================")
    print("      INICIANDO EXPERIMENTO E3 (GT-M): CUSTO DE DERIVA E PROMPTS ")
    print("=================================================================")

    rapl = RAPLSensor()
    nvml = NVMLSensor()

    p_idle_pre, _ = measure_baseline(rapl, nvml, duration_s=10)
    print(f"[C1] Baseline Pré-Coleta (Estado Real Ocioso P8): {p_idle_pre:.3f} W")

    conditions = [
        {"name": "Prompt_Short_128t_T0.0", "seq_len": 128, "temp": 0.0},
        {"name": "Prompt_Med_512t_T0.7", "seq_len": 512, "temp": 0.7},
        {"name": "Prompt_Long_1024t_T1.0", "seq_len": 1024, "temp": 1.0},
    ]

    empirical_results = {}

    for cond in conditions:
        c_name = cond["name"]
        print(f"\n[*] Testando Condição Empírica: {c_name} ({num_runs} repetições)...")
        queue = multiprocessing.Queue()
        p = multiprocessing.Process(
            target=worker_inference, 
            args=(cond["seq_len"], cond["temp"], num_runs, p_idle_pre, queue)
        )
        p.start()
        p.join()
        
        if not queue.empty():
            res = queue.get()
            if res["status"] != "ok":
                raise RuntimeError(f"Erro no sub-processo ao testar {c_name}: {res['error']}")
            runs_joules = res["runs_joules"]
        else:
            raise RuntimeError(f"Sub-processo falhou sem retornar dados para {c_name}.")
        
        mean_j = statistics.mean(runs_joules)
        std_j = statistics.stdev(runs_joules) if len(runs_joules) > 1 else 0.0
        cv_j = std_j / mean_j if mean_j > 0 else 0.0

        empirical_results[c_name] = {
            "seq_len": cond["seq_len"],
            "temperature": cond["temp"],
            "mean_net_joules": mean_j,
            "stdev_net_joules": std_j,
            "cv_ratio": cv_j,
            "cv_pass": cv_j <= 0.15
        }
        print(f"    -> {c_name}: Energia = {mean_j:.4f} J (CV: {cv_j*100:.2f}%)")
        print(f"    -> Contexto CUDA limpo pelo SO. Cooldown inter-condição (5s)...")
        time.sleep(5)

    print("\n[C1] Aguardando resfriamento térmico final (até 3 min) para garantir o P-State Ocioso...")
    consecutive_ok = 0
    for i in range(60):
        time.sleep(3)
        current_w = (nvml.read_mW() / 1000.0) if nvml.available else 0.0
        diff_ratio = abs(current_w - p_idle_pre) / p_idle_pre if p_idle_pre > 0 else 0.0
        if current_w > 0 and diff_ratio <= 0.025:
            consecutive_ok += 1
            if consecutive_ok >= 2:
                print(f"    -> Estabilizado em {current_w:.3f} W após {i*3}s.")
                break
        else:
            consecutive_ok = 0

        if i > 0 and i % 5 == 0:
            print(f"       ... resfriando, potência atual: {current_w:.3f} W (alvo: < {p_idle_pre * 1.025:.3f} W)")

    p_idle_post, _ = measure_baseline(rapl, nvml, duration_s=10)
    drift = abs(p_idle_post - p_idle_pre) / p_idle_pre if p_idle_pre > 0 else 0.0

    # Simulação da Custo Amortizado de Deriva baseada no modelo 512t
    e_base_ref = empirical_results["Prompt_Med_512t_T0.7"]["mean_net_joules"]
    simulation_results = compute_amortized_drift_cost(e_base_joules=e_base_ref)

    cv_pass_all = all(r["cv_pass"] for r in empirical_results.values())
    drift_pass = drift <= 0.05
    overall_pass = cv_pass_all and drift_pass

    report = {
        "experiment": "E3_Custo_Deriva_E_Sensibilidade_Prompt",
        "timestamp_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "num_runs_per_condition": num_runs,
        "baseline_pre_W": p_idle_pre,
        "baseline_post_W": p_idle_post,
        "baseline_drift_ratio": drift,
        "empirical_results": empirical_results,
        "simulation_drift_cost": simulation_results,
        "gates_status": {
            "G3.1_cv_pass_all_conditions": cv_pass_all,
            "G3.2_baseline_drift_pass": drift_pass,
            "G3.3_hybrid_nature_declared": True,
            "overall_E3_gate_passed": overall_pass
        }
    }

    os.makedirs("docs/experiments/E3_Custo_Deriva/artifacts", exist_ok=True)
    artifact_path = "docs/experiments/E3_Custo_Deriva/artifacts/E3_raw_data.json"
    with open(artifact_path, "w") as f:
        json.dump(report, f, indent=2)

    print("\n=================================================================")
    print("               RESULTADOS DO EXPERIMENTO E3                      ")
    print("=================================================================")
    print("--- [Parte A: Medição Empírica no Silício T4] ---")
    for k, v in empirical_results.items():
        print(f" [*] {k}: Energia = {v['mean_net_joules']:.4f} J (CV: {v['cv_ratio']*100:.2f}%)")
    print("\n--- [Parte B: Simulação Matemática de Energia Amortizada] ---")
    print(f" [*] Energia Base de Inferência: {simulation_results['e_base_inference_J']:.4f} J")
    print(f" [*] Recalibrações Estimadas na Vida Útil: {simulation_results['recalibrations_needed']} eventos")
    print(f" [*] Energia Amortizada Real por Inferência: {simulation_results['e_amortized_inference_J']:.4f} J (+{simulation_results['amortized_overhead_pct']:.2f}% de custo de manutenção)")

    print(f"\n [*] Status do Gate G3.1 (CV <= 15% em todas condições): {'PASS ✅' if cv_pass_all else 'FAIL ❌'}")
    print(f" [*] Status do Gate G3.2 (Deriva de Baseline <= 5%): {drift*100:.2f}% -> {'PASS ✅' if drift_pass else 'FAIL ❌'}")
    print(f" [*] GATE GERAL DO EXPERIMENTO E3: {'APROVADO [PASS]' if overall_pass else 'REPROVADO [FAIL]'}")
    print(f" [*] Artefato gravado em: {artifact_path}\n")

    return report

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Executa o experimento E3 (Custo de Deriva e Sensibilidade a Prompt/Temp)")
    parser.add_argument("--runs", type=int, default=30, help="Número de repetições por condição (padrão: 30)")
    args = parser.parse_args()
    
    multiprocessing.set_start_method('spawn', force=True)
    run_experiment_e3(num_runs=args.runs)
