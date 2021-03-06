package apmgometrics

import (
	"context"

	"github.com/rcrowley/go-metrics"

	"github.com/elastic/apm-agent-go"
)

// Wrap wraps r, a go-metrics Registry, so that it can be used
// as an elasticapm.MetricsGatherer.
func Wrap(r metrics.Registry) elasticapm.MetricsGatherer {
	return gatherer{r}
}

type gatherer struct {
	r metrics.Registry
}

// GatherMEtrics gathers metrics into m.
func (g gatherer) GatherMetrics(ctx context.Context, m *elasticapm.Metrics) error {
	g.r.Each(func(name string, v interface{}) {
		switch v := v.(type) {
		case metrics.Counter:
			// NOTE(axw) in go-metrics, counters can go up and down,
			// hence we use a gauge here. Should we provide config
			// to allow a user to specify that a counter is always
			// increasing, hence represent it as a counter type?
			m.AddGauge(name, "", nil, float64(v.Count()))
		case metrics.Gauge:
			m.AddGauge(name, "", nil, float64(v.Value()))
		case metrics.GaugeFloat64:
			m.AddGauge(name, "", nil, v.Value())
		case metrics.Histogram:
			stddev := v.StdDev()
			min := float64(v.Min())
			max := float64(v.Max())
			quantiles := map[float64]float64{0.5: 0, 0.9: 0, 0.99: 0}
			for q := range quantiles {
				quantiles[q] = v.Percentile(q)
			}
			m.AddSummary(name, "", nil, elasticapm.SummaryMetric{
				Count:     uint64(v.Count()),
				Sum:       float64(v.Sum()),
				Stddev:    &stddev,
				Min:       &min,
				Max:       &max,
				Quantiles: quantiles,
			})
		default:
			// TODO(axw) Meter, Timer, EWMA
		}
	})
	return nil
}
