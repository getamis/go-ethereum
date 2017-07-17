// Copyright 2015 The go-ethereum Authors
// This file is part of the go-ethereum library.
//
// The go-ethereum library is free software: you can redistribute it and/or modify
// it under the terms of the GNU Lesser General Public License as published by
// the Free Software Foundation, either version 3 of the License, or
// (at your option) any later version.
//
// The go-ethereum library is distributed in the hope that it will be useful,
// but WITHOUT ANY WARRANTY; without even the implied warranty of
// MERCHANTABILITY or FITNESS FOR A PARTICULAR PURPOSE. See the
// GNU Lesser General Public License for more details.
//
// You should have received a copy of the GNU Lesser General Public License
// along with the go-ethereum library. If not, see <http://www.gnu.org/licenses/>.

package metrics

import (
	"errors"
	"fmt"
	"os"
	"strings"
	"time"

	"github.com/ethereum/go-ethereum/log"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/push"
	"github.com/rcrowley/go-metrics"
)

var (
	// format should be --promaddr 127.0.0.1:9091
	PrometheusAddrFlag = "promaddr"

	prometheusAddr = ""
	// enabled is the flag specifying if metrics are enable or not.
	prometheusEnabled = false
)

// Init enables or disables the metrics system. Since we need this to run before
// any other code gets to create meters and timers, we'll actually do an ugly hack
// and peek into the command line args for the metrics flag.
func init() {
	for i, arg := range os.Args {
		if strings.TrimLeft(arg, "-") == PrometheusAddrFlag {
			prometheusAddr = os.Args[i+1]
			prometheusEnabled = true
			log.Info("Enabling prometheus exporter", "addr", prometheusAddr)
		}
	}
	if prometheusEnabled {
		r := newPrometheusRegister(prometheusAddr, "geth", getSystemInfo(), metrics.DefaultRegistry)
		go r.export()
	}
}

type prometheusRegister struct {
	namespace      string
	url            string
	metricRegistry metrics.Registry
	registry       *prometheus.Registry
	labels         map[string]string
	gauges         map[string]prometheus.Gauge
	counters       map[string]prometheus.Counter
	exportDuration time.Duration
}

func newPrometheusRegister(url, namespace string, labels map[string]string, metricRegistry metrics.Registry) *prometheusRegister {
	return &prometheusRegister{
		namespace:      namespace,
		url:            url,
		metricRegistry: metricRegistry,
		registry:       prometheus.NewRegistry(),
		labels:         labels,
		gauges:         make(map[string]prometheus.Gauge),
		counters:       make(map[string]prometheus.Counter),
		exportDuration: 10 * time.Second,
	}
}

func (r *prometheusRegister) GetOrRegisterCounter(name string) prometheus.Counter {
	cnt, ok := r.counters[name]
	if !ok {
		cnt = prometheus.NewCounter(prometheus.CounterOpts{
			Namespace: flattenKey(r.namespace),
			Subsystem: "",
			Name:      flattenKey(name),
			Help:      name,
		})
		r.registry.MustRegister(cnt)
		r.counters[name] = cnt
	}
	return cnt
}

func (r *prometheusRegister) GetOrRegisterGauge(name string) prometheus.Gauge {
	gauge, ok := r.gauges[name]
	if !ok {
		gauge = prometheus.NewGauge(prometheus.GaugeOpts{
			Namespace: flattenKey(r.namespace),
			Subsystem: "",
			Name:      flattenKey(name),
			Help:      name,
		})
		r.registry.MustRegister(gauge)
		r.gauges[name] = gauge
	}
	return gauge
}

func (r *prometheusRegister) export() {
	for _ = range time.Tick(r.exportDuration) {
		r.UpdatePrometheusMetricsOnce()
		push.FromGatherer(r.namespace, r.labels, r.url, r.registry)
	}
}

func (r *prometheusRegister) gaugeFromNameAndValue(name string, val float64) {
	g := r.GetOrRegisterGauge(name)
	g.Set(val)
}

func (r *prometheusRegister) UpdatePrometheusMetricsOnce() error {
	if !Enabled {
		return errors.New("metric is not enabled.")
	}
	r.metricRegistry.Each(func(name string, i interface{}) {
		switch metric := i.(type) {
		case metrics.Counter:
			r.gaugeFromNameAndValue(name, float64(metric.Count()))
		case metrics.Gauge:
		case metrics.GaugeFloat64:
			r.gaugeFromNameAndValue(name, float64(metric.Value()))
		case metrics.Meter:
			snap := metric.Snapshot()
			r.gaugeFromNameAndValue(name, snap.Rate1())
			name5 := fmt.Sprintf("%s.rate5", name)
			r.gaugeFromNameAndValue(name5, snap.Rate5())
			name15 := fmt.Sprintf("%s.rate15", name)
			r.gaugeFromNameAndValue(name15, snap.Rate15())
			nameMean := fmt.Sprintf("%s.ratemean", name)
			r.gaugeFromNameAndValue(nameMean, snap.RateMean())
		case metrics.Timer:
			snap := metric.Snapshot()
			r.gaugeFromNameAndValue(name, snap.Mean())
			name50 := fmt.Sprintf("%s.p50", name)
			r.gaugeFromNameAndValue(name50, snap.Percentile(float64(0.5)))
			name90 := fmt.Sprintf("%s.p90", name)
			r.gaugeFromNameAndValue(name90, snap.Percentile(float64(0.9)))
			name95 := fmt.Sprintf("%s.p95", name)
			r.gaugeFromNameAndValue(name95, snap.Percentile(float64(0.95)))
		}
	})
	return nil
}

func flattenKey(key string) string {
	key = strings.Replace(key, " ", "_", -1)
	key = strings.Replace(key, ".", "_", -1)
	key = strings.Replace(key, "-", "_", -1)
	key = strings.Replace(key, "=", "_", -1)
	key = strings.Replace(key, "/", "_", -1)
	return key
}

func getSystemInfo() map[string]string {
	labels := map[string]string{}
	if host, err := os.Hostname(); err == nil {
		labels["instance"] = host
	}
	return labels
}
