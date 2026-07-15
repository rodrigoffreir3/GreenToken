package tokens

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"os"
	"testing"
	"time"
)

func TestPrometheusTokenSource_Normal(t *testing.T) {
	fixture, err := os.ReadFile("testdata/vllm_metrics.txt")
	if err != nil {
		t.Fatalf("falha ao ler fixture: %v", err)
	}

	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(fixture)
	}))
	defer srv.Close()

	source := NewPrometheusTokenSource(srv.URL, "vllm:generation_tokens_total")
	tokens, err := source.CumulativeTokens()
	
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	
	// 12345.0 + 20.0 = 12365
	if tokens != 12365 {
		t.Errorf("esperava 12365, obteve %d", tokens)
	}
}

func TestPrometheusTokenSource_HTTPError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	source := NewPrometheusTokenSource(srv.URL, "vllm:generation_tokens_total")
	_, err := source.CumulativeTokens()
	
	if err == nil {
		t.Errorf("esperava erro para HTTP 500")
	}
}

func TestPrometheusTokenSource_LargeBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Escreve um corpo maior que 1MB
		largeBody := bytes.Repeat([]byte("a"), 2*1024*1024)
		w.Write(largeBody)
	}))
	defer srv.Close()

	source := NewPrometheusTokenSource(srv.URL, "vllm:generation_tokens_total")
	_, err := source.CumulativeTokens()
	
	if err == nil {
		t.Errorf("esperava erro devido a métrica não encontrada ou corpo grande")
	}
}

func TestPrometheusTokenSource_NonNumeric(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write([]byte("vllm:generation_tokens_total not_a_number\n"))
	}))
	defer srv.Close()

	source := NewPrometheusTokenSource(srv.URL, "vllm:generation_tokens_total")
	_, err := source.CumulativeTokens()
	
	if err == nil {
		t.Errorf("esperava erro de métrica não encontrada (valor não numérico ignorado)")
	}
}

func TestPrometheusTokenSource_Regression(t *testing.T) {
	var body []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		w.Write(body)
	}))
	defer srv.Close()

	source := NewPrometheusTokenSource(srv.URL, "vllm:generation_tokens_total")
	
	// Round 1: 500
	body = []byte("vllm:generation_tokens_total 500.0\n")
	tokens, err := source.CumulativeTokens()
	if err != nil || tokens != 500 {
		t.Fatalf("round 1 falhou: %d %v", tokens, err)
	}

	// Round 2: 30 (Engine restart)
	body = []byte("vllm:generation_tokens_total 30.0\n")
	tokens, err = source.CumulativeTokens()
	if err != nil || tokens != 30 {
		t.Fatalf("round 2 falhou: %d %v (deveria retornar 30 como novo baseline)", tokens, err)
	}
	
	// PreviousCumulative deve ser atualizado para 30
	if source.previousCumulative != 30 {
		t.Errorf("previousCumulative incorreto: %d", source.previousCumulative)
	}
}

func TestPrometheusTokenSource_Timeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(2 * time.Second)
		w.WriteHeader(http.StatusOK)
	}))
	defer srv.Close()

	source := NewPrometheusTokenSource(srv.URL, "vllm:generation_tokens_total")
	_, err := source.CumulativeTokens()
	
	if err == nil {
		t.Errorf("esperava erro de timeout")
	}
}

func TestPrometheusTokenSource_WithTimestamp(t *testing.T) {
	// Testa se o parser ignora o timestamp opcional no final da linha (especificação Prometheus)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
		// Simula output real com labels que contêm espaços e um timestamp no final (171829384756)
		w.Write([]byte("vllm:generation_tokens_total{model=\"model space\"} 456.0 171829384756\nvllm:generation_tokens_total 10.0 99999\n"))
	}))
	defer srv.Close()

	source := NewPrometheusTokenSource(srv.URL, "vllm:generation_tokens_total")
	tokens, err := source.CumulativeTokens()
	
	if err != nil {
		t.Fatalf("erro inesperado: %v", err)
	}
	
	// 456 + 10 = 466. Se o parser errar, ele pegaria 171829384756 + 99999
	if tokens != 466 {
		t.Errorf("esperava 466 (ignorando timestamps), obteve %d", tokens)
	}
}

