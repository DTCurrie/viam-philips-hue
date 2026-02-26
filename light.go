package hue

import (
	"fmt"

	"github.com/amimof/huego"
	"go.viam.com/rdk/logging"
)

// LightConfig holds the connection parameters common to all Hue light components.
type LightConfig struct {
	BridgeHost string `json:"bridge_host,omitempty"`
	Username   string `json:"username"`
	LightID    int    `json:"light_id"`
}

func (cfg *LightConfig) Validate(path string) ([]string, []string, error) {
	if cfg.Username == "" {
		return nil, nil, fmt.Errorf("need a username (API key) for the Hue bridge")
	}
	if cfg.LightID == 0 {
		return nil, nil, fmt.Errorf("need a light_id")
	}
	return nil, nil, nil
}

// connectToLight resolves the bridge host (discovering it if empty), connects to
// the bridge, and verifies the target light is reachable.
func connectToLight(cfg *LightConfig, logger logging.Logger) (*huego.Bridge, *huego.Light, error) {
	bridgeHost := cfg.BridgeHost
	if bridgeHost == "" {
		logger.Info("No bridge_host specified, discovering Hue bridge...")
		discovered, err := huego.Discover()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to discover Hue bridge: %w", err)
		}
		bridgeHost = discovered.Host
		logger.Infof("Discovered Hue bridge at %s", bridgeHost)
	}

	bridge := huego.New(bridgeHost, cfg.Username)
	light, err := bridge.GetLight(cfg.LightID)
	if err != nil {
		return nil, nil, fmt.Errorf("can't get light %d from Hue bridge @ (%s): %w", cfg.LightID, bridgeHost, err)
	}

	return bridge, light, nil
}
