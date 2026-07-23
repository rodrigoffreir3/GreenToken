// This Source Code Form is subject to the terms of the Mozilla Public
// License, v. 2.0. If a copy of the MPL was not distributed with this
// file, You can obtain one at http://mozilla.org/MPL/2.0/.

package tokens

import (
	"io"
	"log"
	"os"
	"strings"
	"sync/atomic"
	"time"
)

// LogSnifferTokenSource é um fallback que conta tokens lendo o stdout de engines 
// que não suportam exposição por métricas (ex: modo verboso puro).
// Implementa a interface TokenSource retornando um acumulado.
type LogSnifferTokenSource struct {
	path              string
	accumulatedTokens int64
}

// NewLogSnifferTokenSource cria a fonte e inicia a goroutine de tail no arquivo de log.
func NewLogSnifferTokenSource(path string) *LogSnifferTokenSource {
	src := &LogSnifferTokenSource{
		path: path,
	}
	go src.startLogSniffer()
	return src
}

func (s *LogSnifferTokenSource) startLogSniffer() {
	log.Printf("[TOKEN] Iniciando sniffer de tokens no arquivo: %s", s.path)
	var file *os.File
	var err error

	for {
		file, err = os.Open(s.path)
		if err == nil {
			break
		}
		time.Sleep(2 * time.Second)
	}
	defer file.Close()

	// Posiciona no final do arquivo atual ao inicializar,
	// para não recontar tokens passados.
	_, _ = file.Seek(0, io.SeekEnd)
	buffer := make([]byte, 8192)
	var pending string

	for {
		n, err := file.Read(buffer)
		if n > 0 {
			pending += string(buffer[:n])
			lines := strings.Split(pending, "\n")
			// Guarda a última linha incompleta
			pending = lines[len(lines)-1]

			for i := 0; i < len(lines)-1; i++ {
				if t, matched := CountTokens(lines[i]); matched {
					// Adição cumulativa sem dedup, como especificado na GT-02
					atomic.AddInt64(&s.accumulatedTokens, t)
				}
			}
		}
		
		if err == io.EOF {
			time.Sleep(200 * time.Millisecond)
			continue
		}
		
		if err != nil {
			log.Printf("[TOKEN] Erro lendo log de tokens: %v", err)
			time.Sleep(2 * time.Second)
		}
	}
}

// CumulativeTokens retorna o número acumulado de tokens sniffados.
func (s *LogSnifferTokenSource) CumulativeTokens() (int64, error) {
	val := atomic.LoadInt64(&s.accumulatedTokens)
	return val, nil
}

// Name identifica a fonte.
func (s *LogSnifferTokenSource) Name() string {
	return "logsniffer:" + s.path
}
