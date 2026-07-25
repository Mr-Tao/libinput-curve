// SPDX-License-Identifier: Apache-2.0 OR MIT

package config

const (
	SchemaV1  = "io.github.mr-tao.libinput-curve/v1"
	MaxPoints = 64
	MaxValue  = 10000
)

type Config struct {
	Schema   string             `json:"schema"`
	Profiles map[string]Profile `json:"profiles"`
	Devices  []DeviceRule       `json:"devices"`
}

type Profile struct {
	Fallback *Curve `json:"fallback,omitempty"`
	Motion   *Curve `json:"motion,omitempty"`
	Scroll   *Curve `json:"scroll,omitempty"`
}

type Curve struct {
	Step   float64   `json:"step"`
	Points []float64 `json:"points"`
}

type DeviceRule struct {
	ID          string  `json:"id"`
	Match       Match   `json:"match"`
	Profile     string  `json:"profile"`
	MotionScale float64 `json:"motion_scale"`
	ScrollScale float64 `json:"scroll_scale"`
}

type Match struct {
	NameRegex       string `json:"name_regex,omitempty"`
	ProductContains string `json:"product_contains,omitempty"`
	VendorID        string `json:"vendor_id,omitempty"`
	ProductID       string `json:"product_id,omitempty"`
}

type HardwareID struct {
	VendorID  string `json:"vendor_id"`
	ProductID string `json:"product_id"`
}

func (id HardwareID) String() string {
	if id.VendorID == "" || id.ProductID == "" {
		return ""
	}
	return id.VendorID + ":" + id.ProductID
}

type ScaleOverrides struct {
	Motion *float64
	Scroll *float64
}
