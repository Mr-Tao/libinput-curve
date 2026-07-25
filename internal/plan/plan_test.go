// SPDX-License-Identifier: Apache-2.0 OR MIT

package plan

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"

	"github.com/Mr-Tao/libinput-curve/internal/config"
	"github.com/Mr-Tao/libinput-curve/internal/xinput"
)

func TestBuildInSyncAndDrift(t *testing.T) {
	cfg := testConfig()
	device := testDevice()

	inSync, err := Build(cfg, []xinput.Device{device}, config.ScaleOverrides{})
	if err != nil {
		t.Fatal(err)
	}
	if inSync.HasErrors() || inSync.OperationCount() != 0 || !inSync.Devices[0].InSync {
		t.Fatalf("unexpected in-sync plan: %#v", inSync)
	}

	device.Properties["libinput Accel Custom Motion Points"] = xinput.Property{
		Name:   "libinput Accel Custom Motion Points",
		Values: []string{"0.000000", "0.500000"},
	}
	device.Properties[propertyProfileEnabled] = xinput.Property{
		Name: propertyProfileEnabled, Values: []string{"1", "0", "0"},
	}
	drift, err := Build(cfg, []xinput.Device{device}, config.ScaleOverrides{})
	if err != nil {
		t.Fatal(err)
	}
	if drift.HasErrors() || drift.OperationCount() != 2 || drift.Devices[0].InSync {
		t.Fatalf("unexpected drift plan: %#v", drift)
	}
	if got := []string{
		drift.Devices[0].Operations[0].Property,
		drift.Devices[0].Operations[1].Property,
	}; !reflect.DeepEqual(got, []string{
		"libinput Accel Custom Motion Points",
		propertyProfileEnabled,
	}) {
		t.Fatalf("unexpected operation order: %#v", got)
	}
}

func TestBuildUsesScaleAndReportsUnmatchedRule(t *testing.T) {
	cfg := testConfig()
	scale := 2.0
	planned, err := Build(
		cfg,
		[]xinput.Device{testDevice()},
		config.ScaleOverrides{Motion: &scale},
	)
	if err != nil {
		t.Fatal(err)
	}
	if planned.OperationCount() != 1 {
		t.Fatalf("unexpected operation count: %#v", planned)
	}
	if got := planned.Devices[0].Operations[0].Desired; !reflect.DeepEqual(
		got,
		[]string{"0.000000", "2.000000", "6.000000"},
	) {
		t.Fatalf("unexpected scaled points: %#v", got)
	}

	cfg.Devices = append(cfg.Devices, config.DeviceRule{
		ID:          "absent",
		Match:       config.Match{NameRegex: "^Absent$"},
		Profile:     "motion",
		MotionScale: 1,
		ScrollScale: 1,
	})
	planned, err = Build(cfg, []xinput.Device{testDevice()}, config.ScaleOverrides{})
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(planned.UnmatchedRules, []string{"absent"}) {
		t.Fatalf("unexpected unmatched rules: %#v", planned.UnmatchedRules)
	}
}

func TestBuildRejectsAmbiguousAndUnsupportedDevice(t *testing.T) {
	cfg := testConfig()
	cfg.Devices = append(cfg.Devices, config.DeviceRule{
		ID:          "second",
		Match:       config.Match{VendorID: "045e", ProductID: "07a5"},
		Profile:     "motion",
		MotionScale: 1,
		ScrollScale: 1,
	})
	ambiguous, err := Build(cfg, []xinput.Device{testDevice()}, config.ScaleOverrides{})
	if err != nil {
		t.Fatal(err)
	}
	if !ambiguous.HasErrors() ||
		!strings.Contains(ambiguous.Devices[0].Errors[0], "multiple rules") {
		t.Fatalf("expected ambiguity: %#v", ambiguous)
	}

	cfg = testConfig()
	device := testDevice()
	delete(device.Properties, propertyProfilesAvailable)
	unsupported, err := Build(cfg, []xinput.Device{device}, config.ScaleOverrides{})
	if err != nil {
		t.Fatal(err)
	}
	if !unsupported.HasErrors() {
		t.Fatalf("expected unsupported device: %#v", unsupported)
	}
}

func TestBuildReportsMissingCurveProperty(t *testing.T) {
	cfg := testConfig()
	cfg.Profiles["motion"] = config.Profile{
		Scroll: &config.Curve{Step: 1, Points: []float64{0, 1}},
	}
	device := testDevice()
	planned, err := Build(cfg, []xinput.Device{device}, config.ScaleOverrides{})
	if err != nil {
		t.Fatal(err)
	}
	if !planned.HasErrors() ||
		!strings.Contains(strings.Join(planned.Devices[0].Errors, " "), "Scroll Points") {
		t.Fatalf("expected missing scroll property: %#v", planned)
	}
}

type recordingSetter struct {
	operations []Operation
	err        error
}

func (s *recordingSetter) SetProperty(
	_ context.Context,
	deviceID int,
	property string,
	values []string,
) error {
	s.operations = append(s.operations, Operation{
		DeviceID: deviceID,
		Property: property,
		Desired:  append([]string(nil), values...),
	})
	return s.err
}

func TestApplyPreflightsAndStopsOnFailure(t *testing.T) {
	invalid := Plan{Devices: []Device{{Errors: []string{"unsupported"}}}}
	setter := &recordingSetter{}
	if err := Apply(context.Background(), setter, invalid); err == nil {
		t.Fatal("expected preflight error")
	}
	if len(setter.operations) != 0 {
		t.Fatal("preflight changed a property")
	}

	valid := Plan{Devices: []Device{{Operations: []Operation{{
		DeviceID: 1, Property: "one", Desired: []string{"1"},
	}, {
		DeviceID: 1, Property: "two", Desired: []string{"2"},
	}}}}}
	setter.err = errors.New("failed")
	if err := Apply(context.Background(), setter, valid); err == nil {
		t.Fatal("expected apply error")
	}
	if len(setter.operations) != 1 {
		t.Fatalf("expected stop after first failure: %#v", setter.operations)
	}
}

func TestFloatComparisonAllowsXInputPrintRounding(t *testing.T) {
	if !floatValuesEqual(
		[]string{"16.095118", "18.435738"},
		[]string{"16.095117", "18.435737"},
	) {
		t.Fatal("six-decimal XInput rounding should be treated as equal")
	}
	if floatValuesEqual([]string{"1.000000"}, []string{"1.000010"}) {
		t.Fatal("meaningful drift was treated as rounding")
	}
}

func testConfig() *config.Config {
	return &config.Config{
		Schema: config.SchemaV1,
		Profiles: map[string]config.Profile{
			"motion": {
				Motion: &config.Curve{Step: 0.5, Points: []float64{0, 1, 3}},
			},
		},
		Devices: []config.DeviceRule{{
			ID: "sculpt",
			Match: config.Match{
				NameRegex: "Mouse$",
				VendorID:  "045e",
				ProductID: "07a5",
			},
			Profile:     "motion",
			MotionScale: 1,
			ScrollScale: 1,
		}},
	}
}

func testDevice() xinput.Device {
	return xinput.Device{
		ID:   30,
		Name: "Example Mouse",
		Properties: map[string]xinput.Property{
			"Device Product ID": {
				Name: "Device Product ID", Values: []string{"1118", "1957"},
			},
			propertyProfilesAvailable: {
				Name: propertyProfilesAvailable, Values: []string{"1", "1", "1"},
			},
			propertyProfileEnabled: {
				Name: propertyProfileEnabled, Values: []string{"0", "0", "1"},
			},
			"libinput Accel Custom Motion Points": {
				Name:   "libinput Accel Custom Motion Points",
				Values: []string{"0.000000", "1.000000", "3.000000"},
			},
			"libinput Accel Custom Motion Step": {
				Name: "libinput Accel Custom Motion Step", Values: []string{"0.500000"},
			},
		},
	}
}
