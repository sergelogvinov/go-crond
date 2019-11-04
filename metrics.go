package main

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type MetricsExporter struct {
	r               *Runner
	CronJobStatus   *prometheus.Desc
	CronJobDuration *prometheus.Desc
}

func NewMetricsExporter(r *Runner) *MetricsExporter {
	return &MetricsExporter{
		r: r,
		CronJobStatus: prometheus.NewDesc("cronjob_execute_error",
			"Last cronjob run error",
			[]string{"jobname", "id"},
			nil,
		),
		CronJobDuration: prometheus.NewDesc("cronjob_execute_duration",
			"Last cronjob run duration seconds",
			[]string{"jobname", "id"},
			nil,
		),
	}
}

func (collector *MetricsExporter) Describe(ch chan<- *prometheus.Desc) {
	ch <- collector.CronJobStatus
	ch <- collector.CronJobDuration
}

func (collector *MetricsExporter) Collect(ch chan<- prometheus.Metric) {
	jobs := collector.r.GetJobs()

	for _, e := range jobs {
		if e.Updated {
			if e.Status != nil {
				ch <- prometheus.MustNewConstMetric(collector.CronJobStatus, prometheus.CounterValue, 1, e.Name, fmt.Sprintf("%d", e.Id))
			} else {
				ch <- prometheus.MustNewConstMetric(collector.CronJobStatus, prometheus.CounterValue, 0, e.Name, fmt.Sprintf("%d", e.Id))
			}
			ch <- prometheus.MustNewConstMetric(collector.CronJobDuration, prometheus.CounterValue, float64(e.Elapsed/time.Second), e.Name, fmt.Sprintf("%d", e.Id))
		}
	}
}
