package hue

import (
	"context"
	"fmt"

	"github.com/amimof/huego"
	"go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var HueLightBrightness = family.WithModel("hue-light-brightness")

func init() {
	resource.RegisterComponent(toggleswitch.API, HueLightBrightness,
		resource.Registration[toggleswitch.Switch, *LightBrightnessConfig]{
			Constructor: newHueLightBrightness,
		},
	)
}

// LightBrightnessConfig embeds LightConfig — Validate is inherited.
type LightBrightnessConfig struct {
	LightConfig
}

type hueLightBrightness struct {
	resource.AlwaysRebuild
	resource.TriviallyCloseable

	name   resource.Name
	logger logging.Logger
	cfg    *LightBrightnessConfig

	bridge *huego.Bridge
	light  *huego.Light
}

func newHueLightBrightness(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (toggleswitch.Switch, error) {
	conf, err := resource.NativeConfig[*LightBrightnessConfig](rawConf)
	if err != nil {
		return nil, err
	}

	bridge, light, err := connectToLight(&conf.LightConfig, logger)
	if err != nil {
		return nil, err
	}

	return &hueLightBrightness{
		name:   rawConf.ResourceName(),
		logger: logger,
		cfg:    conf,
		bridge: bridge,
		light:  light,
	}, nil
}

func (s *hueLightBrightness) Name() resource.Name {
	return s.name
}

func (s *hueLightBrightness) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return nil, nil
}

// SetPosition controls on/off and brightness.
// 0 = off; position 1 is full brightness, higher values map to brightness levels.
func (s *hueLightBrightness) SetPosition(ctx context.Context, position uint32, extra map[string]interface{}) error {
	light, err := s.bridge.GetLight(s.cfg.LightID)
	if err != nil {
		return fmt.Errorf("failed to get light state: %w", err)
	}
	s.light = light

	if position == 0 {
		if err := s.light.Off(); err != nil {
			return fmt.Errorf("failed to turn off light: %w", err)
		}
		return nil
	}

	if err := s.light.On(); err != nil {
		return fmt.Errorf("failed to turn on light: %w", err)
	}

	if position <= 100 {
		// Hue brightness is 1–254.
		bri := uint8((float64(position) / 100.0) * 254)
		if bri < 1 {
			bri = 1
		}
		if err := s.light.Bri(bri); err != nil {
			return fmt.Errorf("failed to set brightness: %w", err)
		}
	}

	return nil
}

func (s *hueLightBrightness) GetPosition(ctx context.Context, extra map[string]interface{}) (uint32, error) {
	// Refresh light state
	light, err := s.bridge.GetLight(s.cfg.LightID)
	if err != nil {
		return 0, fmt.Errorf("failed to get light state: %w", err)
	}
	s.light = light

	if !s.light.State.On {
		return 0, nil
	}

	// Return brightness as position (1-100 scale)
	// Hue brightness is 1-254
	if s.light.State.Bri > 0 {
		pos := uint32((float64(s.light.State.Bri) / 254.0) * 100)
		if pos < 1 {
			pos = 1
		}
		return pos, nil
	}

	return 1, nil
}

func (s *hueLightBrightness) GetNumberOfPositions(ctx context.Context, extra map[string]interface{}) (uint32, []string, error) {
	// 0 = off, 1-100 = brightness levels
	return 101, nil, nil
}
