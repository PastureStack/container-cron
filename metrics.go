package main

import (
	"net/http"
	"os"
	"time"

	"github.com/PastureStack/container-cron/events"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/sirupsen/logrus"
)

var (
	activeJobGauge *prometheus.GaugeVec
)

func initMetrics() {
	hostname, _ := os.Hostname()
	activeJobGauge = prometheus.NewGaugeVec(
		prometheus.GaugeOpts{
			Name:        "pasturestack_container_cron_jobs",
			Help:        "Current number of container cron jobs",
			ConstLabels: prometheus.Labels{"hostname": hostname},
		}, []string{"state"})
	prometheus.MustRegister(activeJobGauge)
}

func collectMetrics(handler *events.DockerHandler) {
	for {
		handler.GetJobStats(activeJobGauge)
		time.Sleep(5 * time.Second)
	}
}

func MetricsServer(handler *events.DockerHandler) {
	initMetrics()

	go collectMetrics(handler)

	http.Handle("/metrics", promhttp.Handler())
	logrus.Fatal(http.ListenAndServe(":9191", nil))
}
