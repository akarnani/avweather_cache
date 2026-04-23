package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// Data pull metrics
	LastSuccessfulPullAge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "avweather_last_successful_pull_age_seconds",
		Help: "Age of the last successful data pull in seconds",
	})

	LastPullAttemptAge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "avweather_last_pull_attempt_age_seconds",
		Help: "Age of the last data pull attempt in seconds",
	})

	PullErrors = promauto.NewCounter(prometheus.CounterOpts{
		Name: "avweather_pull_errors_total",
		Help: "Total number of errors encountered while pulling data",
	})

	OldestMetarAge = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "avweather_oldest_metar_age_seconds",
		Help: "Age of the oldest METAR in the cache in seconds",
	})

	TotalStations = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "avweather_total_stations",
		Help: "Total number of stations in the cache",
	})

	StationsUnder1Hour = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "avweather_stations_under_1hour",
		Help: "Number of stations with METAR less than 1 hour old",
	})

	StationsUnder2Hours = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "avweather_stations_under_2hours",
		Help: "Number of stations with METAR less than 2 hours old",
	})

	// Query metrics
	QueryLatency = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "avweather_query_latency_seconds",
		Help:    "Latency of API queries in seconds",
		Buckets: prometheus.DefBuckets,
	})

	StationsFilteredByAge = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "avweather_stations_filtered_by_age_total",
		Help: "Stations queried but not included due to age filter",
	}, []string{"station"})

	StationsNotCached = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "avweather_stations_not_cached_total",
		Help: "Stations queried but not found in cache",
	}, []string{"station"})

	QueriesByStation = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "avweather_queries_by_station_total",
		Help: "Count of queries by station",
	}, []string{"station"})

	TotalQueries = promauto.NewCounter(prometheus.CounterOpts{
		Name: "avweather_queries_total",
		Help: "Total number of API queries",
	})

	NearestQueries = promauto.NewCounter(prometheus.CounterOpts{
		Name: "avweather_nearest_queries_total",
		Help: "Total number of /api/metar/nearest queries",
	})

	NearestNoMatch = promauto.NewCounter(prometheus.CounterOpts{
		Name: "avweather_nearest_no_match_total",
		Help: "Nearest queries that returned no matching station",
	})

	NearestDistanceMi = promauto.NewHistogram(prometheus.HistogramOpts{
		Name:    "avweather_nearest_distance_mi",
		Help:    "Distance in statute miles to the nearest matching station",
		Buckets: []float64{1, 5, 10, 25, 50, 100, 200, 500},
	})
)
