package hue

import (
	"context"
	"fmt"
	"math"

	"github.com/amimof/huego"
	"go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/logging"
	"go.viam.com/rdk/resource"
)

var HueLightColor = family.WithModel("hue-light-color")

func init() {
	resource.RegisterComponent(toggleswitch.API, HueLightColor,
		resource.Registration[toggleswitch.Switch, *LightColorConfig]{
			Constructor: newHueLightColor,
		},
	)
}

// LightColorConfig embeds LightConfig and adds a per-channel selector.
// Validate for the shared fields is inherited; channel is validated here.
type LightColorConfig struct {
	LightConfig
	Channel string `json:"channel"` // "red", "green", or "blue"
}

func (cfg *LightColorConfig) Validate(path string) ([]string, []string, error) {
	if _, _, err := cfg.LightConfig.Validate(path); err != nil {
		return nil, nil, err
	}
	switch cfg.Channel {
	case "red", "green", "blue":
	default:
		return nil, nil, fmt.Errorf("channel must be \"red\", \"green\", or \"blue\", got %q", cfg.Channel)
	}
	return nil, nil, nil
}

type hueLightColor struct {
	resource.AlwaysRebuild
	resource.TriviallyCloseable

	name   resource.Name
	logger logging.Logger
	cfg    *LightColorConfig

	bridge *huego.Bridge
}

func newHueLightColor(ctx context.Context, deps resource.Dependencies, rawConf resource.Config, logger logging.Logger) (toggleswitch.Switch, error) {
	conf, err := resource.NativeConfig[*LightColorConfig](rawConf)
	if err != nil {
		return nil, err
	}

	bridge, _, err := connectToLight(&conf.LightConfig, logger)
	if err != nil {
		return nil, err
	}

	return &hueLightColor{
		name:   rawConf.ResourceName(),
		logger: logger,
		cfg:    conf,
		bridge: bridge,
	}, nil
}

func (s *hueLightColor) Name() resource.Name {
	return s.name
}

func (s *hueLightColor) DoCommand(ctx context.Context, cmd map[string]interface{}) (map[string]interface{}, error) {
	return map[string]interface{}{}, nil
}

// SetPosition sets the configured RGB channel to the given value.
// Position maps 1-to-1 to the channel value (0–255).
func (s *hueLightColor) SetPosition(ctx context.Context, position uint32, extra map[string]interface{}) error {
	if position > 255 {
		return fmt.Errorf("position must be 0–255, got %d", position)
	}

	light, err := s.bridge.GetLight(s.cfg.LightID)
	if err != nil {
		return fmt.Errorf("failed to get light state: %w", err)
	}

	r, g, b := xyBriToRGB(light.State.Xy, light.State.Bri)

	channelValue := uint8(position)
	switch s.cfg.Channel {
	case "red":
		r = channelValue
	case "green":
		g = channelValue
	case "blue":
		b = channelValue
	}

	x, y := rgbToXY(r, g, b)
	if err := light.XyContext(ctx, []float32{x, y}); err != nil {
		return fmt.Errorf("failed to set color: %w", err)
	}

	return nil
}

// GetPosition returns the current value of the configured RGB channel (0–255).
func (s *hueLightColor) GetPosition(ctx context.Context, extra map[string]interface{}) (uint32, error) {
	light, err := s.bridge.GetLight(s.cfg.LightID)
	if err != nil {
		return 0, fmt.Errorf("failed to get light state: %w", err)
	}

	if !light.State.On {
		return 0, nil
	}

	r, g, b := xyBriToRGB(light.State.Xy, light.State.Bri)

	var channelValue uint8
	switch s.cfg.Channel {
	case "red":
		channelValue = r
	case "green":
		channelValue = g
	case "blue":
		channelValue = b
	}

	return uint32(channelValue), nil
}

func (s *hueLightColor) GetNumberOfPositions(ctx context.Context, extra map[string]interface{}) (uint32, []string, error) {
	return 256, nil, nil
}

// rgbToXY converts sRGB values (0–255) to CIE xy chromaticity coordinates
// using the Philips Hue wide-gamut (D65) color matrix.
func rgbToXY(r, g, b uint8) (x, y float32) {
	rLin := srgbToLinear(float64(r) / 255.0)
	gLin := srgbToLinear(float64(g) / 255.0)
	bLin := srgbToLinear(float64(b) / 255.0)

	// Wide gamut D65 matrix.
	X := rLin*0.664511 + gLin*0.154324 + bLin*0.162028
	Y := rLin*0.283881 + gLin*0.668433 + bLin*0.047685
	Z := rLin*0.000088 + gLin*0.072310 + bLin*0.986039

	sum := X + Y + Z
	if sum == 0 {
		return 0, 0
	}
	return float32(X / sum), float32(Y / sum)
}

// xyBriToRGB converts CIE xy chromaticity + Hue brightness (1–254) to sRGB (0–255).
func xyBriToRGB(xy []float32, bri uint8) (r, g, b uint8) {
	if len(xy) < 2 {
		return 0, 0, 0
	}

	x := float64(xy[0])
	y := float64(xy[1])
	if y == 0 {
		return 0, 0, 0
	}

	Y := float64(bri) / 254.0
	X := (Y / y) * x
	Z := (Y / y) * (1 - x - y)

	// Wide gamut D65 inverse matrix.
	rLin := X*1.656492 - Y*0.354851 - Z*0.255038
	gLin := -X*0.707196 + Y*1.655397 + Z*0.036152
	bLin := X*0.051713 - Y*0.121364 + Z*1.011530

	rLin = math.Max(0, rLin)
	gLin = math.Max(0, gLin)
	bLin = math.Max(0, bLin)

	// Scale so the brightest channel is 1.0, preserving hue.
	scale := math.Max(rLin, math.Max(gLin, bLin))
	if scale > 1 {
		rLin /= scale
		gLin /= scale
		bLin /= scale
	}

	return linearToSRGB8(rLin), linearToSRGB8(gLin), linearToSRGB8(bLin)
}

func srgbToLinear(c float64) float64 {
	if c > 0.04045 {
		return math.Pow((c+0.055)/1.055, 2.4)
	}
	return c / 12.92
}

func linearToSRGB8(c float64) uint8 {
	var out float64
	if c > 0.0031308 {
		out = 1.055*math.Pow(c, 1/2.4) - 0.055
	} else {
		out = 12.92 * c
	}
	return uint8(math.Round(math.Min(1, math.Max(0, out)) * 255))
}
