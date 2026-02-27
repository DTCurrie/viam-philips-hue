package hue

import (
	"context"
	"fmt"
	"math"

	"github.com/amimof/huego"
	toggleswitch "go.viam.com/rdk/components/switch"
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

type LightColorConfig struct {
	BridgeHost string `json:"bridge_host,omitempty"`
	Username   string `json:"username"`
	LightID    int    `json:"light_id"`
	Channel    string `json:"channel"` // "red", "green", or "blue"
}

func (cfg *LightColorConfig) Validate(path string) ([]string, []string, error) {
	if cfg.Username == "" {
		return nil, nil, fmt.Errorf("need a username (API key) for the Hue bridge")
	}
	if cfg.LightID == 0 {
		return nil, nil, fmt.Errorf("need a light_id")
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

	s := &hueLightColor{
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

	maxChan := maxUint8(r, g, b)
	if maxChan == 0 {
		if err := light.SetState(huego.State{On: false}); err != nil {
			return fmt.Errorf("failed to turn off light: %w", err)
		}
		return nil
	}

	bri := maxChan
	if bri > 254 {
		bri = 254
	}

	x, y := rgbToXY(r, g, b)
	if err := light.SetState(huego.State{
		On:  true,
		Xy:  []float32{x, y},
		Bri: bri,
	}); err != nil {
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

// xyBriToRGB converts CIE xy chromaticity + brightness to sRGB (0–255).
//
// Bri is treated as max(r, g, b) — the encoding SetPosition writes. The
// color direction is computed at Y=1 (full luminance), the brightest linear
// channel is normalized to 1.0, gamma is applied to get 8-bit sRGB at full
// brightness, and then every channel is scaled by Bri/255 so that the
// brightest channel equals Bri. This matches SetPosition's Bri=max(r,g,b)
// encoding and makes the round-trip lossless.
func xyBriToRGB(xy []float32, bri uint8) (r, g, b uint8) {
	if len(xy) < 2 {
		return 0, 0, 0
	}

	x := float64(xy[0])
	y := float64(xy[1])
	if y == 0 {
		return 0, 0, 0
	}

	// Use Y=1 to extract the pure color direction regardless of stored luminance.
	X := x / y
	Z := (1 - x - y) / y

	// Wide gamut D65 inverse matrix.
	rLin := X*1.656492 - 0.354851 - Z*0.255038
	gLin := -X*0.707196 + 1.655397 + Z*0.036152
	bLin := X*0.051713 - 0.121364 + Z*1.011530

	rLin = math.Max(0, rLin)
	gLin = math.Max(0, gLin)
	bLin = math.Max(0, bLin)

	// Normalize so the brightest linear channel = 1.0, preserving hue.
	scale := math.Max(rLin, math.Max(gLin, bLin))
	if scale > 0 {
		rLin /= scale
		gLin /= scale
		bLin /= scale
	}

	// Convert to 8-bit sRGB at full brightness (max channel = 255).
	rFull := float64(linearToSRGB8(rLin))
	gFull := float64(linearToSRGB8(gLin))
	bFull := float64(linearToSRGB8(bLin))

	// Scale by Bri/255 so that max(r,g,b) == Bri, matching SetPosition's encoding.
	briF := float64(bri) / 255.0
	return uint8(math.Round(rFull * briF)),
		uint8(math.Round(gFull * briF)),
		uint8(math.Round(bFull * briF))
}

func maxUint8(a, b, c uint8) uint8 {
	if a >= b && a >= c {
		return a
	}
	if b >= c {
		return b
	}
	return c
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
