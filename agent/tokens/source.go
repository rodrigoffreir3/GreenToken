package tokens

// TokenSource fornece a contagem de tokens gerados acumulada desde o início do processo.
// Implementações devem ser thread-safe e retornar um contador MONOTÔNICO (sempre crescente),
// permitindo ao chamador calcular o delta entre duas janelas de medição.
type TokenSource interface {
	// CumulativeTokens retorna o total de tokens gerados desde o início da observação.
	// O valor é monotônico. Retorna erro se a fonte está indisponível; nesse caso
	// o chamador deve assumir 0 tokens na janela e prosseguir (degradação graciosa).
	CumulativeTokens() (int64, error)

	// Name identifica a fonte para logging e métricas.
	Name() string
}

// NullTokenSource é uma fonte de tokens que sempre retorna 0.
// Usada quando a contagem de tokens está desativada.
type NullTokenSource struct{}

func (n *NullTokenSource) CumulativeTokens() (int64, error) {
	return 0, nil
}

func (n *NullTokenSource) Name() string {
	return "none"
}
