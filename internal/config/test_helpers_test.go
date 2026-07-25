// SPDX-License-Identifier: Apache-2.0 OR MIT

package config

func validConfig() *Config {
	return &Config{
		Schema: SchemaV1,
		Profiles: map[string]Profile{
			"default": {
				Fallback: &Curve{
					Step:   0.5,
					Points: []float64{0, 1},
				},
			},
		},
		Devices: []DeviceRule{
			{
				ID:          "mouse",
				Match:       Match{NameRegex: `Mouse`},
				Profile:     "default",
				MotionScale: 1,
				ScrollScale: 1,
			},
		},
	}
}

func floatPointer(value float64) *float64 {
	return &value
}
