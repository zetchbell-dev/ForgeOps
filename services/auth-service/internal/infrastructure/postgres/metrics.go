package postgres

import (
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
)

// RegisterPoolMetrics registers GaugeFunc collectors that read
// pool.Stat() on every Prometheus scrape (M5 Phase 1: "Database
// metrics"). GaugeFunc, rather than a periodic background goroutine that
// polls and caches the value, because pgxpool.Stat() is cheap and safe
// to call concurrently — this mirrors pool.go's own NewPool, which
// doesn't start a background goroutine either.
//
// Two of the six (new_conns and empty_acquire) are cumulative counts,
// not point-in-time gauges, but pgxpool.Stat() only exposes them as
// running totals with no reset hook this package could invoke — they're
// registered as gauges (not counters) so the exposed type honestly
// reflects that this package cannot guarantee monotonicity across a
// process restart, and their metric names deliberately avoid the
// Prometheus "_total" suffix convention (which specifically implies
// counter semantics) for the same reason.
func RegisterPoolMetrics(reg *prometheus.Registry, pool *pgxpool.Pool) error {
	collectorList := []prometheus.Collector{
		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "postgres_pool_acquired_conns",
			Help: "Number of connections currently checked out from the pool.",
		}, func() float64 { return float64(pool.Stat().AcquiredConns()) }),

		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "postgres_pool_idle_conns",
			Help: "Number of idle connections currently held by the pool.",
		}, func() float64 { return float64(pool.Stat().IdleConns()) }),

		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "postgres_pool_total_conns",
			Help: "Total connections (acquired + idle + constructing) currently tracked by the pool.",
		}, func() float64 { return float64(pool.Stat().TotalConns()) }),

		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "postgres_pool_max_conns",
			Help: "Configured maximum pool size.",
		}, func() float64 { return float64(pool.Stat().MaxConns()) }),

		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "postgres_pool_new_conns_count",
			Help: "Cumulative count of new connections opened by the pool since process start.",
		}, func() float64 { return float64(pool.Stat().NewConnsCount()) }),

		prometheus.NewGaugeFunc(prometheus.GaugeOpts{
			Name: "postgres_pool_empty_acquire_count",
			Help: "Cumulative count of Acquire calls that had to wait because no idle connection was immediately available.",
		}, func() float64 { return float64(pool.Stat().EmptyAcquireCount()) }),
	}

	for _, c := range collectorList {
		if err := reg.Register(c); err != nil {
			return err
		}
	}
	return nil
}
