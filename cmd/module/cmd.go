package main

import (
	"go.viam.com/rdk/components/sensor"
	toggleswitch "go.viam.com/rdk/components/switch"
	"go.viam.com/rdk/module"
	"go.viam.com/rdk/resource"
	"go.viam.com/rdk/services/discovery"

	hue "github.com/erh/hue"
)

func main() {
	module.ModularMain(
		resource.APIModel{toggleswitch.API, hue.HueLightBrightness},
		resource.APIModel{toggleswitch.API, hue.HueLightColor},
		resource.APIModel{toggleswitch.API, hue.HueLightMode},
		resource.APIModel{discovery.API, hue.HueDiscovery},
		resource.APIModel{sensor.API, hue.HueLightSensor},
	)
}
