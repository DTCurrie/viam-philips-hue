package hue

import (
	"fmt"
	"github.com/amimof/huego"
	"go.viam.com/rdk/logging"
)

// connectToLight resolves the bridge host (discovering it if empty), connects to
// the bridge, and verifies the target light is reachable.
func connectToLight(bridgeHost, username string, lightID int, logger logging.Logger) (*huego.Bridge, *huego.Light, error) {
	if bridgeHost == "" {
		logger.Info("No bridge_host specified, discovering Hue bridge...")
		bridge, err := huego.Discover()
		if err != nil {
			return nil, nil, fmt.Errorf("failed to discover Hue bridge: %w", err)
		}
		bridgeHost = bridge.Host
		logger.Infof("Discovered Hue bridge at %s", bridgeHost)
	}

	bridge := huego.New(bridgeHost, username)
	light, err := bridge.GetLight(lightID)
	if err != nil {
		return nil, nil, fmt.Errorf("can't get light %d from Hue bridge @ (%s): %w", lightID, bridgeHost, err)
	}

	return bridge, light, nil
}
