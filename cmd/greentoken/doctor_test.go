// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package main

import (
	"fmt"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"testing"
)

func TestCheckRAPL(t *testing.T) {
	tmpDir := t.TempDir()
	origPath := raplSysfsPath
	defer func() { raplSysfsPath = origPath }()

	// Teste: Ausente
	raplSysfsPath = filepath.Join(tmpDir, "intel-rapl-missing")
	res := checkRAPL()
	if res.Status != StatusAviso {
		t.Errorf("Esperava AVISO para RAPL ausente, obteve %s", res.Status)
	}

	// Teste: Presente
	raplSysfsPath = filepath.Join(tmpDir, "intel-rapl-0")
	if err := os.MkdirAll(raplSysfsPath, 0755); err != nil {
		t.Fatal(err)
	}
	res = checkRAPL()
	if res.Status != StatusOK {
		t.Errorf("Esperava OK para RAPL presente, obteve %s", res.Status)
	}
}

func TestCheckEBPF(t *testing.T) {
	tmpDir := t.TempDir()
	origDebug := debugfsTracePath
	origProc := procTracePath
	defer func() {
		debugfsTracePath = origDebug
		procTracePath = origProc
	}()

	// Teste: Ausente
	debugfsTracePath = filepath.Join(tmpDir, "debugfs")
	procTracePath = filepath.Join(tmpDir, "proc")
	res := checkEBPF()
	if res.Status != StatusAviso {
		t.Errorf("Esperava AVISO para eBPF ausente, obteve %s", res.Status)
	}

	// Teste: Presente
	if err := os.MkdirAll(debugfsTracePath, 0755); err != nil {
		t.Fatal(err)
	}
	res = checkEBPF()
	if res.Status != StatusOK {
		t.Errorf("Esperava OK para eBPF presente, obteve %s", res.Status)
	}
}

func TestCheckEBPF_PermissaoNegada(t *testing.T) {
	if isRootFunc() {
		t.Skip("rodando com privilégios elevados — restrição de permissão não é aplicável neste teste")
	}

	tmpDir := t.TempDir()
	origDebug := debugfsTracePath
	origProc := procTracePath
	defer func() {
		debugfsTracePath = origDebug
		procTracePath = origProc
	}()

	restricted := filepath.Join(tmpDir, "debugfs-restrito")
	if err := os.MkdirAll(restricted, 0755); err != nil {
		t.Fatal(err)
	}
	// Remove toda permissão de acesso do diretório para simular negação real.
	if err := os.Chmod(restricted, 0000); err != nil {
		t.Fatal(err)
	}
	defer os.Chmod(restricted, 0755) // garante limpeza do t.TempDir()

	debugfsTracePath = restricted
	procTracePath = filepath.Join(tmpDir, "proc-inexistente")

	res := checkEBPF()
	if res.Status != StatusFalha {
		t.Errorf("Esperava FALHA para permissão negada, obteve %s (msg: %s)", res.Status, res.Message)
	}
}

func TestCheckGPU(t *testing.T) {
	res := checkGPU()
	if res.Status != StatusAviso {
		t.Errorf("Esperava AVISO para GPU stub, obteve %s", res.Status)
	}
}

func TestCheckPermissions(t *testing.T) {
	origRootFunc := isRootFunc
	defer func() { isRootFunc = origRootFunc }()

	// Teste: Non-root
	isRootFunc = func() bool { return false }
	res := checkPermissions()
	if res.Status != StatusFalha {
		t.Errorf("Esperava FALHA para não-root, obteve %s", res.Status)
	}

	// Teste: Root
	isRootFunc = func() bool { return true }
	res = checkPermissions()
	if res.Status != StatusOK {
		t.Errorf("Esperava OK para root, obteve %s", res.Status)
	}
}

func TestCheckCollector(t *testing.T) {
	// Teste: Inalcançável
	res := checkCollector("127.0.0.1:59999")
	if res.Status != StatusAviso {
		t.Errorf("Esperava AVISO para collector inalcançável, obteve %s", res.Status)
	}

	// Teste: Alcançável (listener TCP fake)
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()

	res = checkCollector(ln.Addr().String())
	if res.Status != StatusOK {
		t.Errorf("Esperava OK para collector alcançável, obteve %s", res.Status)
	}
}

func TestCheckTokenSource(t *testing.T) {
	// 1. Métrica presente
	tsOk := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "vllm:generation_tokens_total 100.0")
	}))
	defer tsOk.Close()

	res := checkTokenSource(tsOk.URL, "vllm:generation_tokens_total")
	if res.Status != StatusOK {
		t.Errorf("Esperava OK para métrica presente, obteve %s", res.Status)
	}

	// 2. Métrica ausente
	tsMissing := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		fmt.Fprintln(w, "other_metric 1.0")
	}))
	defer tsMissing.Close()

	res = checkTokenSource(tsMissing.URL, "vllm:generation_tokens_total")
	if res.Status != StatusAviso {
		t.Errorf("Esperava AVISO para métrica ausente, obteve %s", res.Status)
	}

	// 3. Server error / offline
	res = checkTokenSource("http://127.0.0.1:59999/metrics", "vllm:generation_tokens_total")
	if res.Status != StatusAviso {
		t.Errorf("Esperava AVISO para server offline, obteve %s", res.Status)
	}
}
