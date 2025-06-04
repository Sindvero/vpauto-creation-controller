package metrics

import (
	"github.com/prometheus/client_golang/prometheus"
)

type Collectors struct {
	VPACreated *prometheus.CounterVec
	VPADeleted *prometheus.CounterVec
}

func NewCollectors() *Collectors {
	return &Collectors{
		VPACreated: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "vpactrl_created_vpa_total",
				Help: "Number of VPAs successfully created by the controller",
			},
			[]string{"kind", "namespace"},
		),
		VPADeleted: prometheus.NewCounterVec(
			prometheus.CounterOpts{
				Name: "vpactrl_deleted_orphaned_vpa_total",
				Help: "Number of orphaned VPAs deleted by the controller",
			},
			[]string{"namespace"},
		),
	}
}

func SetupMetrics() *Collectors {
	c := NewCollectors()
	prometheus.MustRegister(c.VPACreated, c.VPADeleted)
	return c
}
