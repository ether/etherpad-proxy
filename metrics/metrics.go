package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	// RequestsTotal counts proxy requests by outcome:
	// proxied, no_backend, clash, resource_redirect.
	RequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Name: "etherpad_proxy_requests_total",
		Help: "Total proxy requests by outcome.",
	}, []string{"outcome"})

	// BackendsUp is the number of backends currently reachable.
	BackendsUp = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "etherpad_proxy_backends_up",
		Help: "Number of backends currently up.",
	})

	// BackendsAvailable is the number of up backends still under capacity.
	BackendsAvailable = promauto.NewGauge(prometheus.GaugeOpts{
		Name: "etherpad_proxy_backends_available",
		Help: "Number of backends currently available (up and under capacity).",
	})

	// PadAssignmentsTotal counts new pad-to-backend assignments.
	PadAssignmentsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "etherpad_proxy_pad_assignments_total",
		Help: "Total number of new pad assignments.",
	})

	// ClashesTotal counts recorded pad clashes.
	ClashesTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "etherpad_proxy_clashes_total",
		Help: "Total number of recorded pad clashes.",
	})

	// DBErrorsTotal counts database operation errors.
	DBErrorsTotal = promauto.NewCounter(prometheus.CounterOpts{
		Name: "etherpad_proxy_db_errors_total",
		Help: "Total number of database errors.",
	})
)
