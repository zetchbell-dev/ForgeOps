package redis

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

// RegisterPoolMetrics registers GaugeFunc collectors that read
// client.PoolStats() on every Prometheus scrape (M5 Phase 1: "Redis
// metrics") — the same GaugeFunc-not-goroutine shape as
// postgres.RegisterPoolMetrics, for the same reason (PoolStats() is a
// cheap, concurrency-safe read with no need for a cached background
// poller).
//
// Scoped to connection-pool health only, not per-command latency:
// go-redis v9's per-command instrumentation goes through its Hook
// interface, which this change deliberately doesn't wire up in this
// phase — the pool-level signal below is what M5 §3 actually asks for
// ("Redis metrics" as one bullet, no per-command breakdown named), and
// getting a custom Hook wrong in a way this sandbox has no way to
// compile-check felt like a worse trade than shipping the well-understood
// PoolStats() surface now and adding command-level histograms as a
// separate, reviewable follow-up.
func RegisterPoolMetrics(reg *prometheus.Registry, client *redis.Client) error {
	collectorList := []prometheus.Collector{
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "redis_pool_hits_count",
			Help: "Cumulative count of connection pool hits (a free connection was found immediately).",
		}, func() float64 { return float64(client.PoolStats().Hits) }),

		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "redis_pool_misses_count",
			Help: "Cumulative count of connection pool misses (a new connection had to be created).",
		}, func() float64 { return float64(client.PoolStats().Misses) }),

		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "redis_pool_timeouts_count",
			Help: "Cumulative count of connection pool wait timeouts.",
		}, func() float64 { return float64(client.PoolStats().Timeouts) }),

		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "redis_pool_total_conns",
			Help: "Total connections currently tracked by the pool.",
		}, func() float64 { return float64(client.PoolStats().TotalConns) }),

		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "redis_pool_idle_conns",
			Help: "Idle connections currently held by the pool.",
		}, func() float64 { return float64(client.PoolStats().IdleConns) }),

		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "redis_pool_stale_conns",
			Help: "Cumulative count of stale connections removed from the pool.",
		}, func() float64 { return float64(client.PoolStats().StaleConns) }),
	}

	for _, c := range collectorList {
		if err := reg.Register(c); err != nil {
			return err
		}
	}
	return nil
}
