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
	lastLine string
	lastTime time.Time
	mu       sync.Mutex
)

// CountTokens analisa uma linha de log para extrair a contagem de tokens gerados.
// Possui deduplicação baseada em tempo para evitar dupla contagem se o mesmo engine
// disparar múltiplas linhas de log duplicadas em caso de retransmissão de stdio.
func CountTokens(line string) (int64, bool) {
	for _, re := range regexes {
		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 {
			val, err := strconv.ParseInt(matches[1], 10, 64)
			if err == nil {
				mu.Lock()
				now := time.Now()
				// Dedup: se a exata mesma linha de log foi processada em <500ms, assume-se duplicação.
				if line == lastLine && now.Sub(lastTime) < 500*time.Millisecond {
					mu.Unlock()
					return 0, false 
				}
				lastLine = line
				lastTime = now
				mu.Unlock()

				return val, true
			}
		}
	}
	return 0, false
}
