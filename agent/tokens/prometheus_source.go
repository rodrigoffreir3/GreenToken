// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package tokens

import (
	"bufio"
	"fmt"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"
)

// PrometheusTokenSource lê tokens do endpoint /metrics de uma engine compatível
// com exposição Prometheus (vLLM, TGI). É thread-safe.
type PrometheusTokenSource struct {
	endpoint   string
	metricName string
	client     *http.Client
	
	mu                 sync.Mutex
	previousCumulative int64
}

// NewPrometheusTokenSource cria uma nova fonte conectada ao endpoint /metrics do engine.
func NewPrometheusTokenSource(endpoint, metricName string) *PrometheusTokenSource {
	return &PrometheusTokenSource{
		endpoint:   endpoint,
		metricName: metricName,
		client: &http.Client{
			Timeout: 1 * time.Second,
			// Prevê que o endpoint não siga redirects para hosts externos acidentalmente ou maliciosamente.
			CheckRedirect: func(req *http.Request, via []*http.Request) error {
				if len(via) >= 10 {
					return fmt.Errorf("stopped after 10 redirects")
				}
				if req.URL.Host != via[0].URL.Host {
					return fmt.Errorf("redirect para host diferente rejeitado")
				}
				return nil
			},
		},
	}
}

// CumulativeTokens busca as métricas no endpoint via HTTP GET e retorna o total de tokens cumulativos.
func (p *PrometheusTokenSource) CumulativeTokens() (int64, error) {
	req, err := http.NewRequest("GET", p.endpoint, nil)
	if err != nil {
		return 0, fmt.Errorf("erro criando request: %w", err)
	}

	resp, err := p.client.Do(req)
	if err != nil {
		return 0, fmt.Errorf("erro acessando endpoint %s: %w", p.endpoint, err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return 0, fmt.Errorf("endpoint retornou status não-OK: %d", resp.StatusCode)
	}

	// Zero-trust: limitar o tamanho da resposta a 1MB para evitar sobrecarga de memória (OOM) 
	// no parse se a métrica retornar lixo.
	limitReader := io.LimitReader(resp.Body, 1024*1024)
	scanner := bufio.NewScanner(limitReader)

	var currentTotal float64
	var found bool

	// Parse defensivo por linha
	for scanner.Scan() {
		line := scanner.Text()
		
		// O formato padrão do prometheus é:
		// metric_name 123
		// metric_name{label="val"} 123
		
		// Ignoramos linhas de HELP e TYPE
		if strings.HasPrefix(line, "#") {
			continue
		}

		// A linha tem que começar com o nome da métrica
		if !strings.HasPrefix(line, p.metricName) {
			continue
		}
		
		// Certifica-se de que não é apenas um prefixo de outra métrica maior, 
		// verificando o próximo caractere. Tem que ser um espaço ou uma chave {
		if len(line) > len(p.metricName) {
			nextChar := line[len(p.metricName)]
			if nextChar != ' ' && nextChar != '{' {
				continue
			}
		}

		// Extrai a parte da string que contém o valor ignorando metadados
		var valuePart string
		idx := strings.LastIndex(line, "}")
		if idx != -1 {
			valuePart = line[idx+1:]
		} else {
			valuePart = line[len(p.metricName):]
		}

		fields := strings.Fields(valuePart)
		if len(fields) == 0 {
			continue // Malformada
		}

		// O primeiro campo após as labels (ou nome da métrica) é SEMPRE o valor.
		// O segundo campo (se houver e ignorado aqui) seria o timestamp.
		valStr := fields[0]
		val, err := strconv.ParseFloat(valStr, 64)
		if err != nil {
			log.Printf("[TOKEN] Prometheus parse aviso: valor não numérico '%s' na linha: %s", valStr, line)
			continue
		}

		currentTotal += val
		found = true
	}

	if err := scanner.Err(); err != nil {
		return 0, fmt.Errorf("erro lendo corpo da resposta: %w", err)
	}

	if !found {
		return 0, fmt.Errorf("métrica '%s' não encontrada na resposta", p.metricName)
	}

	currentInt := int64(currentTotal)

	p.mu.Lock()
	defer p.mu.Unlock()

	// Validação de counter monotônico
	if currentInt < p.previousCumulative {
		log.Printf("[TOKEN] Prometheus: counter regrediu de %d para %d. Engine reiniciou?", p.previousCumulative, currentInt)
		// Trata como novo baseline e atualiza prev, retornando o valor atual ao invés de manter o antigo ou falhar.
		// Assim a próxima janela já calculará corretamente com base no novo ciclo de vida.
		p.previousCumulative = currentInt
		return currentInt, nil
	}

	p.previousCumulative = currentInt
	return currentInt, nil
}

// Name identifica a fonte para logging.
func (p *PrometheusTokenSource) Name() string {
	return "prometheus:" + p.endpoint
}
