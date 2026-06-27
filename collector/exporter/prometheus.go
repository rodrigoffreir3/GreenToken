package exporter

import (
	"math"
	"net/http"
	"strconv"

	"greentoken/collector/aggregator"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

// GreenTokenCollector implementa a interface prometheus.Collector para expor as métricas do agregador.
type GreenTokenCollector struct {
	agg *aggregator.Aggregator

	// Descritores de métricas
	wattsCpuDesc         *prometheus.Desc
	wattsDramDesc        *prometheus.Desc
	wattsGpuDesc         *prometheus.Desc
	tokensTotalDesc      *prometheus.Desc
	costPerTokenDesc     *prometheus.Desc
	joulesPerRequestDesc *prometheus.Desc
}

// NewGreenTokenCollector inicializa e retorna uma instância do GreenTokenCollector.
func NewGreenTokenCollector(agg *aggregator.Aggregator) *GreenTokenCollector {
	labels := []string{"workload", "model", "pid", "gpu_index", "agent_id", "hostname"}

	return &GreenTokenCollector{
		agg: agg,
		wattsCpuDesc: prometheus.NewDesc(
			"greentoken_watts_cpu",
			"Consumo médio móvel de energia da CPU em Watts",
			labels,
			nil,
		),
		wattsDramDesc: prometheus.NewDesc(
			"greentoken_watts_dram",
			"Consumo médio móvel de energia da DRAM em Watts",
			labels,
			nil,
		),
		wattsGpuDesc: prometheus.NewDesc(
			"greentoken_watts_gpu",
			"Consumo médio móvel de energia da GPU em Watts",
			labels,
			nil,
		),
		tokensTotalDesc: prometheus.NewDesc(
			"greentoken_tokens_total",
			"Quantidade total acumulada de tokens gerados pelo workload",
			labels,
			nil,
		),
		costPerTokenDesc: prometheus.NewDesc(
			"greentoken_cost_per_token",
			"Custo financeiro estimado por token gerado baseado em GT_KWH_PRICE",
			labels,
			nil,
		),
		joulesPerRequestDesc: prometheus.NewDesc(
			"greentoken_joules_per_request",
			"Consumo médio móvel de energia da janela em Joules",
			labels,
			nil,
		),
	}
}

// Describe envia a descrição de cada métrica para o canal fornecido.
func (c *GreenTokenCollector) Describe(ch chan<- *prometheus.Desc) {
	ch <- c.wattsCpuDesc
	ch <- c.wattsDramDesc
	ch <- c.wattsGpuDesc
	ch <- c.tokensTotalDesc
	ch <- c.costPerTokenDesc
	ch <- c.joulesPerRequestDesc
}

// Collect extrai as medições consolidadas do agregador e as envia como métricas do Prometheus.
func (c *GreenTokenCollector) Collect(ch chan<- prometheus.Metric) {
	states := c.agg.GetStates()

	for _, state := range states {
		pidStr := strconv.Itoa(int(state.Pid))
		gpuIndexStr := strconv.Itoa(int(state.GpuIndex))
		labelValues := []string{
			state.Workload,
			state.Model,
			pidStr,
			gpuIndexStr,
			state.AgentID,
			state.Hostname,
		}

		// Arredonda valores para evitar ruído e manter legibilidade
		wattsCpu := round(state.WattsCpu, 4)
		wattsDram := round(state.WattsDram, 4)
		wattsGpu := round(state.WattsGpu, 4)
		costPerToken := round(state.CostPerToken, 8)
		joulesPerRequest := round(state.JoulesPerRequest, 4)

		ch <- prometheus.MustNewConstMetric(c.wattsCpuDesc, prometheus.GaugeValue, wattsCpu, labelValues...)
		ch <- prometheus.MustNewConstMetric(c.wattsDramDesc, prometheus.GaugeValue, wattsDram, labelValues...)
		ch <- prometheus.MustNewConstMetric(c.wattsGpuDesc, prometheus.GaugeValue, wattsGpu, labelValues...)
		ch <- prometheus.MustNewConstMetric(c.tokensTotalDesc, prometheus.CounterValue, float64(state.TokensTotal), labelValues...)
		ch <- prometheus.MustNewConstMetric(c.costPerTokenDesc, prometheus.GaugeValue, costPerToken, labelValues...)
		ch <- prometheus.MustNewConstMetric(c.joulesPerRequestDesc, prometheus.GaugeValue, joulesPerRequest, labelValues...)
	}
}

// round arredonda um float64 para uma precisão de casas decimais específica.
func round(val float64, precision int) float64 {
	p := math.Pow10(precision)
	return math.Round(val*p) / p
}

// GetMetricsHandler retorna uma requisição HTTP pronta contendo o endpoint /metrics.
func GetMetricsHandler(agg *aggregator.Aggregator) http.Handler {
	reg := prometheus.NewRegistry()
	c := NewGreenTokenCollector(agg)
	reg.MustRegister(c)

	// Registra coletores do Go runtime padrão para termos diagnósticos do collector
	reg.MustRegister(prometheus.NewGoCollector())
	reg.MustRegister(prometheus.NewProcessCollector(prometheus.ProcessCollectorOpts{}))

	return promhttp.HandlerFor(reg, promhttp.HandlerOpts{})
}
