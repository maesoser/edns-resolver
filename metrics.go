package main

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// DNSMetrics contains all Prometheus metrics for the DNS resolver
type DNSMetrics struct {
	// Request metrics
	RequestsTotal  prometheus.CounterVec
	RequestLatency prometheus.HistogramVec

	// Cache metrics
	CacheHits   prometheus.Counter
	CacheMisses prometheus.Counter
	CacheSize   prometheus.Gauge

	// REST service metrics
	RestLatency prometheus.Histogram
	RestErrors  prometheus.Counter

	// Upstream resolver metrics
	UpstreamLatency prometheus.Histogram
	UpstreamErrors  prometheus.Counter

	// Response metrics
	ResponseCodes    prometheus.CounterVec
	EdnsOptionsAdded prometheus.Counter

	// Category metrics
	CategoriesReceived prometheus.CounterVec
}

// initializeMetrics creates and registers all Prometheus metrics
func initializeMetrics() *DNSMetrics {
	return &DNSMetrics{
		RequestsTotal: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "dns_requests_total",
				Help: "Total number of DNS requests received",
			},
			[]string{"qtype", "qclass", "domain_suffix"},
		),
		RequestLatency: *promauto.NewHistogramVec(
			prometheus.HistogramOpts{
				Name:    "dns_request_latency_seconds",
				Help:    "Latency of DNS request processing in seconds",
				Buckets: prometheus.DefBuckets,
			},
			[]string{"status"},
		),
		CacheHits: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "dns_cache_hits_total",
				Help: "Total number of cache hits",
			},
		),
		CacheMisses: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "dns_cache_misses_total",
				Help: "Total number of cache misses",
			},
		),
		CacheSize: promauto.NewGauge(
			prometheus.GaugeOpts{
				Name: "dns_cache_size",
				Help: "Current number of entries in the cache",
			},
		),
		RestLatency: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "dns_rest_latency_seconds",
				Help:    "Latency of REST service queries in seconds",
				Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
			},
		),
		RestErrors: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "dns_rest_errors_total",
				Help: "Total number of REST service errors",
			},
		),
		UpstreamLatency: promauto.NewHistogram(
			prometheus.HistogramOpts{
				Name:    "dns_upstream_latency_seconds",
				Help:    "Latency of upstream DNS resolver queries in seconds",
				Buckets: []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
			},
		),
		UpstreamErrors: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "dns_upstream_errors_total",
				Help: "Total number of upstream DNS resolver errors",
			},
		),
		ResponseCodes: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "dns_response_codes_total",
				Help: "Total number of DNS responses by response code",
			},
			[]string{"rcode"},
		),
		EdnsOptionsAdded: promauto.NewCounter(
			prometheus.CounterOpts{
				Name: "dns_edns_options_added_total",
				Help: "Total number of EDNS options added to queries",
			},
		),
		CategoriesReceived: *promauto.NewCounterVec(
			prometheus.CounterOpts{
				Name: "dns_categories_received_total",
				Help: "Categories received from REST service",
			},
			[]string{"category"},
		),
	}
}
