#!/usr/bin/env python3
"""
SPEC GT-M — Experimento E3 (FIX v1.1): Custo Energético da Deriva e Amortização por Recalibração
==============================================================================
Correção de Instrumentação e Calibração GPU-bound (v1.1):
1. Calibração de loops em bloco (loops=probe_loops) para medir o tempo real de execução no silício CUDA,
   eliminando a latência de sincronização CPU-driver Python.
2. Garantia de piso de amostragem MIN_DURATION_S = 0.40s (~40 amostras a 10ms).
3. Rejeição estrita e sem silenciamento de repetições anômalas (SPEC GTM-E3-FIX v1.0).
4. Desacoplamento estrito entre Tamanho de Prompt e Temperatura.

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

MIN_DURATION_S = 0.40   # Piso de duração: garante ~40 amostras a 10ms por run
MIN_SAMPLES    = 25     # Número mínimo de amostras aceitas do sampler NVML

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

    def stop_and_integrate(self) -> Tuple[float, int]:
        self.running = False
        if self.thread:
            self.thread.join(timeout=1.0)
        n_samples = len(self.samples)
        if n_samples < 2:
            return 0.0, n_samples
        joules = 0.0
        for i in range(n_samples - 1):
            t1, p1 = self.samples[i]
            t2, p2 = self.samples[i+1]
            dt = t2 - t1
            joules += ((p1 + p2) / 2.0) * dt
        return joules, n_samples

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
                self.prompt = torch.randn(seq_len, dim, device="cuda", dtype=torch.float16)
                self.weights = torch.randn(dim, dim, device="cuda", dtype=torch.float16)
        except Exception:
            self.has_cuda = False

    def run_inference_item(self, item_id: int, loops: int = 100) -> float:
        """Executa a inferência pelo número exato de iterações calibradas para a condição."""
        if self.has_cuda:
            for _ in range(loops):
                out = self.torch.matmul(self.prompt, self.weights)
                if self.temperature > 0.0:
                    out = out / max(0.1, self.temperature)
                    out = self.torch.softmax(out, dim=-1)
            self.torch.cuda.synchronize()
        else:
            acc = 0
            for i in range(10000 * loops):
                acc += i * 0.0001
        return 1.0

def calibrate_loop_count(bench: PromptSensitivityBenchmark, target_duration_s: float = MIN_DURATION_S) -> int:
    """
    SPEC GTM-E3-FIX v1.2:
    Warmup CUDA para eliminar o overhead do primeiro kernel launch e escala min_loops
    inversamente ao tamanho da sequência para evitar aliasing em prompts curtos (128t).
    """
    # 1. Warmup prévio para estabilizar o pipeline CUDA
    bench.run_inference_item(-1, loops=200)
    
    # 2. Probe em bloco
    probe_loops = 2000
    t0 = time.time()
    bench.run_inference_item(-1, loops=probe_loops)
    elapsed = time.time() - t0
    
    min_loops = int(2000 * max(1.0, 512.0 / float(bench.seq_len)))
    
    if elapsed <= 0:
        return min_loops
        
    per_loop = elapsed / float(probe_loops)
    required_loops = max(min_loops, math.ceil(target_duration_s / per_loop))
    return required_loops

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
    Worker executado num sub-processo isolado conforme SPEC GTM-E3-FIX:
    - Calibra iterações dinamicamente no silício CUDA.
    - Elimina clamp silencioso `max(0.0, ...)`.
    - Registra amostragem insuficiente e repetições inválidas.
    """
    try:
        nvml = NVMLSensor()
        bench = PromptSensitivityBenchmark(seq_len=seq_len, temperature=temp)
        
        # 1. Calibração de duração dinâmica GPU-bound (SPEC GTM-E3-FIX 3.1)
        required_loops = calibrate_loop_count(bench, target_duration_s=MIN_DURATION_S)
        
        # 2. Warmup térmico C2
        t_w = time.time()
        while time.time() - t_w < 10:
            bench.run_inference_item(-1, loops=required_loops)

        runs_joules = []
        sample_counts = []
        invalid_samples = 0
        rejection_reasons = []

        for r in range(num_runs):
            t0 = time.time()
            sampler = ContinuousNVMLSampler(nvml) if nvml.available else None
            if sampler:
                sampler.start()

            bench.run_inference_item(r, loops=required_loops)

            j_gross, n_samples = sampler.stop_and_integrate() if sampler else (0.0, 0)
            dt = time.time() - t0
            j_floor = p_idle_pre * dt

            # SPEC GTM-E3-FIX Seção 3.2: Rejeição explícita sem clamp silencioso
            if j_gross < j_floor:
                invalid_samples += 1
                rejection_reasons.append(f"Run {r}: j_gross ({j_gross:.4f}J) < j_floor ({j_floor:.4f}J)")
            elif n_samples < MIN_SAMPLES:
                invalid_samples += 1
                rejection_reasons.append(f"Run {r}: n_samples ({n_samples}) < MIN_SAMPLES ({MIN_SAMPLES})")
            else:
                j_net = j_gross - j_floor
                runs_joules.append(j_net)
                sample_counts.append(n_samples)

        invalid_ratio = invalid_samples / float(num_runs)
        is_instrumentation_failure = invalid_ratio > 0.20

        queue.put({
            "status": "ok",
            "required_loops": required_loops,
            "runs_joules": runs_joules,
            "sample_counts": sample_counts,
            "invalid_samples": invalid_samples,
            "invalid_ratio": invalid_ratio,
            "instrumentation_failure": is_instrumentation_failure,
            "rejection_reasons": rejection_reasons
        })
    except Exception as e:
        queue.put({"status": "error", "error": str(e)})

def compute_amortized_drift_cost(e_base_joules: float, nu: float = 0.02) -> Dict[str, Any]:
    """
    Simulação matemática da retenção de pesos e custo de recalibração temporal (Seção 3 do SPEC).
    """
    num_inferences_total = 100000
    e_recalibration_joules = e_base_joules * 50.0
    recalibrations_needed = math.ceil(10.0 * (nu / 0.02))
    
    total_recal_energy = recalibrations_needed * e_recalibration_joules
    e_amortized = e_base_joules + (total_recal_energy / num_inferences_total)
    overhead_pct = ((e_amortized - e_base_joules) / e_base_joules) * 100.0 if e_base_joules > 0 else 0.0

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
    print("      INICIANDO EXPERIMENTO E3 (GT-M FIX v1.1): CUSTO DE DERIVA  ")
    print("=================================================================")

    rapl = RAPLSensor()
    nvml = NVMLSensor()

    p_idle_pre, _ = measure_baseline(rapl, nvml, duration_s=10)
    print(f"[C1] Baseline Pré-Coleta (Estado Real Ocioso P8): {p_idle_pre:.3f} W")

    TEMP_FIXED = 0.7
    LENGTH_FIXED = 512

    length_conditions = [
        {"name": "Length_128t_TempFixed0.7",  "seq_len": 128,  "temp": TEMP_FIXED},
        {"name": "Length_512t_TempFixed0.7",  "seq_len": 512,  "temp": TEMP_FIXED},
        {"name": "Length_1024t_TempFixed0.7", "seq_len": 1024, "temp": TEMP_FIXED},
    ]

    temperature_conditions = [
        {"name": "Temp_0.0_LengthFixed512t", "seq_len": LENGTH_FIXED, "temp": 0.0},
        {"name": "Temp_0.7_LengthFixed512t", "seq_len": LENGTH_FIXED, "temp": 0.7},
        {"name": "Temp_1.0_LengthFixed512t", "seq_len": LENGTH_FIXED, "temp": 1.0},
    ]

    all_conditions = length_conditions + temperature_conditions
    empirical_results = {}

    for cond in all_conditions:
        c_name = cond["name"]
        print(f"\n[*] Testando Condição Empírica: {c_name} (seq_len={cond['seq_len']}, temp={cond['temp']})...")
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
        else:
            raise RuntimeError(f"Sub-processo falhou sem retornar dados para {c_name}.")
        
        runs_joules = res["runs_joules"]
        is_failure = res["instrumentation_failure"]

        if len(runs_joules) > 1 and not is_failure:
            mean_j = statistics.mean(runs_joules)
            std_j = statistics.stdev(runs_joules)
            cv_j = std_j / mean_j if mean_j > 0 else 0.0
            mean_samples = statistics.mean(res["sample_counts"]) if res["sample_counts"] else 0
            cv_pass = cv_j <= 0.15
        else:
            mean_j = 0.0
            std_j = 0.0
            cv_j = 999.0
            mean_samples = 0
            cv_pass = False

        empirical_results[c_name] = {
            "seq_len": cond["seq_len"],
            "temperature": cond["temp"],
            "required_loops": res["required_loops"],
            "mean_net_joules": mean_j,
            "stdev_net_joules": std_j,
            "cv_ratio": cv_j,
            "valid_runs": len(runs_joules),
            "invalid_samples": res["invalid_samples"],
            "invalid_ratio": res["invalid_ratio"],
            "mean_samples_per_run": mean_samples,
            "instrumentation_failure": is_failure,
            "cv_pass": cv_pass
        }
        
        status_str = "FAIL ❌ (Falha de Instrumentação)" if is_failure else ("PASS ✅" if cv_pass else "FAIL ❌")
        print(f"    -> {c_name}: {mean_j:.4f} J (CV: {cv_j*100:.2f}%, Amostras/run: {mean_samples:.1f}, Loops: {res['required_loops']}) -> {status_str}")
        print(f"    -> Contexto CUDA limpo pelo SO. Cooldown inter-condição (8s)...")
        time.sleep(8)

    print("\n[C1] Aguardando resfriamento térmico final (até 5 min) para garantir o P-State Ocioso...")
    consecutive_ok = 0
    for i in range(100):
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
    ref_name = "Length_512t_TempFixed0.7"
    e_base_ref = empirical_results[ref_name]["mean_net_joules"]
    simulation_results = compute_amortized_drift_cost(e_base_joules=e_base_ref)

    cv_pass_all = all(r["cv_pass"] and not r["instrumentation_failure"] for r in empirical_results.values())
    drift_pass = drift <= 0.05
    overall_pass = cv_pass_all and drift_pass

    report = {
        "experiment": "E3_Custo_Deriva_E_Sensibilidade_Prompt",
        "spec_version": "SPEC GTM-E3-FIX v1.1",
        "timestamp_utc": time.strftime("%Y-%m-%dT%H:%M:%SZ", time.gmtime()),
        "num_runs_per_condition": num_runs,
        "baseline_pre_W": p_idle_pre,
        "baseline_post_W": p_idle_post,
        "baseline_drift_ratio": drift,
        "length_sweep_results": {k: v for k, v in empirical_results.items() if k.startswith("Length_")},
        "temperature_sweep_results": {k: v for k, v in empirical_results.items() if k.startswith("Temp_")},
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
    print("               RESULTADOS DO EXPERIMENTO E3 (FIX v1.1)           ")
    print("=================================================================")
    print("--- [Série 1: Variação de Tamanho de Prompt (Temp = 0.7 Fixo)] ---")
    for k, v in report["length_sweep_results"].items():
        print(f" [*] {k}: Energia = {v['mean_net_joules']:.4f} J (CV: {v['cv_ratio']*100:.2f}%, Amostras/run: {v['mean_samples_per_run']:.1f})")
    
    print("\n--- [Série 2: Variação de Temperatura (Prompt = 512t Fixo)] ---")
    for k, v in report["temperature_sweep_results"].items():
        print(f" [*] {k}: Energia = {v['mean_net_joules']:.4f} J (CV: {v['cv_ratio']*100:.2f}%, Amostras/run: {v['mean_samples_per_run']:.1f})")

    print("\n--- [Parte B: Simulação Matemática de Energia Amortizada] ---")
    print(f" [*] Energia Base de Inferência (512t): {simulation_results['e_base_inference_J']:.4f} J")
    print(f" [*] Recalibrações Estimadas na Vida Útil: {simulation_results['recalibrations_needed']} eventos")
    print(f" [*] Energia Amortizada Real por Inferência: {simulation_results['e_amortized_inference_J']:.4f} J (+{simulation_results['amortized_overhead_pct']:.2f}% de custo de manutenção)")

    print(f"\n [*] Status do Gate G3.1 (CV <= 15% e sem falhas de instrumentação): {'PASS ✅' if cv_pass_all else 'FAIL ❌'}")
    print(f" [*] Status do Gate G3.2 (Deriva de Baseline <= 5%): {drift*100:.2f}% -> {'PASS ✅' if drift_pass else 'FAIL ❌'}")
    print(f" [*] GATE GERAL DO EXPERIMENTO E3: {'APROVADO [PASS]' if overall_pass else 'REPROVADO [FAIL]'}")
    print(f" [*] Artefato gravado em: {artifact_path}\n")

    return report

if __name__ == "__main__":
    parser = argparse.ArgumentParser(description="Executa o experimento E3 (FIX v1.1)")
    parser.add_argument("--runs", type=int, default=30, help="Número de repetições por condição (padrão: 30)")
    args = parser.parse_args()
    
    multiprocessing.set_start_method('spawn', force=True)
    run_experiment_e3(num_runs=args.runs)
