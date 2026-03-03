package hue

import (
	"context"
	"fmt"

	"github.com/amimof/huego"
	"go.viam.com/rdk/components/sensor"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var HueLightSensor = family.WithModel("hue-light-sensor")

func init() {
	resource.RegisterComponent(sensor.API, HueLightSensor,
		resource.Registration[sensor.Sensor, *LightSensorConfig]{
			Constructor: newHueLightSensor,
		},
	)
}

type LightSensorConfig struct {
	BridgeHost string `json:"bridge_host,omitempty"`
	Username   string `json:"username"`
	LightID    int    `json:"light_id"`
}

func (cfg *LightSensorConfig) Validate(path string) ([]string, []string, error) {
	if cfg.Username == "" {
		return nil, nil, fmt.Errorf("need a username (API key) for the Hue bridge")
	}
	if cfg.LightID == 0 {
		return nil, nil, fmt.Errorf("need a light_id")
	}
	return nil, nil, nil
}

type hueLightSensor struct {
	resource.AlwaysRebuild
	resource.TriviallyCloseable

	name   resource.Name
	logger logging.Logger
	cfg    *LightSensorConfig

	bridge *huego.Bridge
}

func newHueLightSensor(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (sensor.Sensor, error) {
	conf, err := resource.NativeConfig[*LightSensorConfig](rawConf)
	if err != nil {
		return nil, err
	}

	s := &hueLightSensor{
		name:   rawConf.ResourceName(),
		logger: logger,
		cfg:    conf,
	}

	s.bridge, _, err = connectToLight(conf.BridgeHost, conf.Username, conf.LightID, logger)
	if err != nil {
		return nil, err
	}

	return s, nil
}

func (s *hueLightSensor) Name() resource.Name {
	return s.name
}

func (s *hueLightSensor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

// Readings returns all available information about the light from the Hue bridge:
// static metadata, native state fields, and computed RGB/brightness values.
func (s *hueLightSensor) Readings(ctx context.Context, extra map[string]interface{}) (map[string]interface{}, error) {
	light, err := s.bridge.GetLight(s.cfg.LightID)
	if err != nil {
		return nil, fmt.Errorf("failed to get light state: %w", err)
	}

	var cieX, cieY float64
	if len(light.State.Xy) >= 2 {
		cieX = float64(light.State.Xy[0])
		cieY = float64(light.State.Xy[1])
	}

	r, g, b := xyBriToRGB(light.State.Xy, light.State.Bri)
	brightness := int(light.State.Bri) * 100 / 254

	return map[string]interface{}{
		// Light metadata
		"light_name":   light.Name,
		"light_type":   light.Type,
		"model_id":     light.ModelID,
		"manufacturer": light.ManufacturerName,
		"product_name": light.ProductName,
		"unique_id":    light.UniqueID,
		"sw_version":   light.SwVersion,

		// Native state
		"is_on":      light.State.On,
		"hue_bri":    int(light.State.Bri),
		"hue":        int(light.State.Hue),
		"saturation": int(light.State.Sat),
		"cie_x":      cieX,
		"cie_y":      cieY,
		"color_temp": int(light.State.Ct),
		"color_mode": light.State.ColorMode,
		"reachable":  light.State.Reachable,
		"effect":     light.State.Effect,
		"alert":      light.State.Alert,

		// Computed values
		"brightness": brightness,
		"red":        int(r),
		"green":      int(g),
		"blue":       int(b),
	}, nil
}
