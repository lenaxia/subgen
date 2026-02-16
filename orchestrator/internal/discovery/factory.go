package discovery

import (
	"fmt"

	"github.com/mccloud/subgen/orchestrator/internal/config"
	"github.com/sirupsen/logrus"
)

// NewDiscovery creates WorkerDiscovery based on configuration
func NewDiscovery(cfg *config.Config, log *logrus.Logger) (WorkerDiscovery, error) {
	switch cfg.Worker.Discovery {
	case "localhost":
		return NewLocalhostDiscovery(cfg.Worker.Address, log), nil

	case "kubernetes":
		return NewKubernetesDiscovery(
			cfg.Worker.Namespace,
			cfg.Worker.ServiceName,
			cfg.Worker.Port,
			log,
		)

	default:
		return nil, fmt.Errorf("unknown worker discovery: %s", cfg.Worker.Discovery)
	}
}
