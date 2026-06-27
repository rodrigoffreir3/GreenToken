package tokens

import (
	"regexp"
	"strconv"
)

var regexes = []*regexp.Regexp{
	// Padrão geral / vLLM: "generated 45 tokens"
	regexp.MustCompile(`(?i)generated\s+(\d+)\s+tokens`),
	// Padrão Ollama / vLLM: "eval count = 120"
	regexp.MustCompile(`(?i)eval\s+count\s*=\s*(\d+)`),
	// Padrão llama.cpp: "eval tokens = 80"
	regexp.MustCompile(`(?i)eval\s+tokens\s*=\s*(\d+)`),
	// Padrão genérico de sumário: "tokens: 256"
	regexp.MustCompile(`(?i)tokens\s*:\s*(\d+)`),
}

// CountTokens analisa uma linha de log para extrair a contagem de tokens gerados.
func CountTokens(line string) (int64, bool) {
	for _, re := range regexes {
		matches := re.FindStringSubmatch(line)
		if len(matches) > 1 {
			val, err := strconv.ParseInt(matches[1], 10, 64)
			if err == nil {
				return val, true
			}
		}
	}
	return 0, false
}
