package tokens

import (
	"regexp"
	"strconv"
	"sync"
	"time"
)

var regexes = []*regexp.Regexp{
	// Padrão geral / vLLM: "generated 45 tokens"
	regexp.MustCompile(`(?i)\bgenerated\s+(\d+)\s+tokens\b`),
	// Padrão Ollama / vLLM: "eval count = 120"
	regexp.MustCompile(`(?i)\beval\s+count\s*=\s*(\d+)\b`),
	// Padrão llama.cpp: "eval tokens = 80"
	regexp.MustCompile(`(?i)\beval\s+tokens\s*=\s*(\d+)\b`),
	// Padrão genérico de sumário: "tokens: 256" (evitar metadados como max_tokens)
	regexp.MustCompile(`(?i)(?:^|\s)tokens\s*:\s*(\d+)\b`),
}

var (
	lastVal  int64
	lastTime time.Time
	mu       sync.Mutex
)

// CountTokens analisa uma linha de log para extrair a contagem de tokens gerados.
// Possui deduplicação baseada em tempo para evitar dupla contagem se o mesmo engine
// disparar múltiplas linhas de log que casem com padrões diferentes para a mesma geração.
//
// ATENÇÃO (Viés de Subcontagem em Carga Alta):
// A deduplicação usa (valor numérico de tokens + intervalo de <500ms) como chave.
// Em cenários de produção com alta simultaneidade, se duas requests terminarem no
// mesmo intervalo e gerarem exatamente a mesma quantidade de tokens (ex: max_tokens=200),
// a segunda emissão será considerada duplicata e será ignorada.
// Para refinar isso, idealmente deve-se associar o evento ao Request ID do log em versões futuras.
func CountTokens(line string) (int64, bool) {
	for _, re := range regexes {
		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 {
			val, err := strconv.ParseInt(matches[1], 10, 64)
			if err == nil {
				mu.Lock()
				now := time.Now()
				// Dedup: se casou o exato mesmo valor num intervalo de <500ms, assume-se mesma geração.
				if val == lastVal && now.Sub(lastTime) < 500*time.Millisecond {
					mu.Unlock()
					return 0, false 
				}
				lastVal = val
				lastTime = now
				mu.Unlock()

				return val, true
			}
		}
	}
	return 0, false
}
