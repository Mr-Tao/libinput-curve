// SPDX-License-Identifier: Apache-2.0 OR MIT

package xinput

import (
	"context"
	"errors"
	"reflect"
	"strings"
	"testing"
)

const sculptProperties = `Device 'Microsoft Microsoft 2.4GHz Transceiver v9.0 Mouse':
	Device Enabled (187):	1
	libinput Accel Profiles Available (340):	1, 1, 1
	libinput Accel Profile Enabled (341):	0, 0, 1
	libinput Accel Custom Motion Points (345):	0.000000, 0.070486, 0.159201
	libinput Accel Custom Motion Step (346):	0.142000
	libinput Accel Custom Scroll Points (347):	0.000000, 0.400000, 0.800000
	libinput Accel Custom Scroll Step (348):	1.000000
	Device Node (311):	"/dev/input/event6"
	Device Product ID (312):	1118, 1957
`

func TestParseProperties(t *testing.T) {
	device, err := ParseProperties(30, sculptProperties)
	if err != nil {
		t.Fatal(err)
	}
	if device.ID != 30 || device.Name != "Microsoft Microsoft 2.4GHz Transceiver v9.0 Mouse" {
		t.Fatalf("unexpected device: %#v", device)
	}
	vendor, product, ok := device.ProductID()
	if !ok || vendor != 0x045e || product != 0x07a5 {
		t.Fatalf("unexpected product ID: %04x:%04x, ok=%v", vendor, product, ok)
	}
	points, err := device.Properties["libinput Accel Custom Motion Points"].Floats()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(points, []float64{0, 0.070486, 0.159201}) {
		t.Fatalf("unexpected points: %#v", points)
	}
	if got := device.Properties["Device Node"].Values; !reflect.DeepEqual(got, []string{"/dev/input/event6"}) {
		t.Fatalf("unexpected device node: %#v", got)
	}
}

func TestParsePropertiesRejectsHeader(t *testing.T) {
	_, err := ParseProperties(1, "not a device\n")
	if err == nil {
		t.Fatal("expected an error")
	}
}

func TestPropertyConversionsRejectInvalidValues(t *testing.T) {
	if _, err := (Property{Name: "float", Values: []string{"x"}}).Floats(); err == nil {
		t.Fatal("expected float conversion error")
	}
	if _, err := (Property{Name: "integer", Values: []string{"1.5"}}).Integers(); err == nil {
		t.Fatal("expected integer conversion error")
	}
}

type call struct {
	command string
	args    []string
}

type fakeRunner struct {
	outputs map[string]string
	errs    map[string]error
	calls   []call
}

func (r *fakeRunner) Run(_ context.Context, command string, args ...string) ([]byte, error) {
	r.calls = append(r.calls, call{command: command, args: append([]string(nil), args...)})
	key := strings.Join(args, " ")
	return []byte(r.outputs[key]), r.errs[key]
}

func TestClientListDevicesAndSetProperty(t *testing.T) {
	runner := &fakeRunner{outputs: map[string]string{
		"list --short":  "Virtual core pointer id=2\n  Sculpt id=30\n",
		"list-props 2":  "Device 'Virtual core pointer':\n\tDevice Enabled (1): 1\n",
		"list-props 30": sculptProperties,
	}}
	client := Client{Command: "xinput-test", Runner: runner}

	devices, err := client.ListDevices(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if len(devices) != 2 || devices[0].ID != 2 || devices[1].ID != 30 {
		t.Fatalf("unexpected devices: %#v", devices)
	}
	if err := client.SetProperty(
		context.Background(),
		30,
		"libinput Accel Profile Enabled",
		[]string{"0", "0", "1"},
	); err != nil {
		t.Fatal(err)
	}
	last := runner.calls[len(runner.calls)-1]
	want := call{
		command: "xinput-test",
		args: []string{
			"set-prop", "30", "libinput Accel Profile Enabled", "0", "0", "1",
		},
	}
	if !reflect.DeepEqual(last, want) {
		t.Fatalf("unexpected call:\n got: %#v\nwant: %#v", last, want)
	}
}

func TestClientIncludesCommandFailureOutput(t *testing.T) {
	runner := &fakeRunner{
		outputs: map[string]string{"list --short": "unable to connect"},
		errs:    map[string]error{"list --short": errors.New("exit 1")},
	}
	client := Client{Runner: runner}
	_, err := client.ListDevices(context.Background())
	if err == nil || !strings.Contains(err.Error(), "unable to connect") {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestClientPreservesContextCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	runner := &fakeRunner{
		errs: map[string]error{"list --short": errors.New("signal: terminated")},
	}
	client := Client{Runner: runner}
	_, err := client.ListDevices(ctx)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestSetPropertyRejectsNoValues(t *testing.T) {
	client := Client{Runner: &fakeRunner{}}
	err := client.SetProperty(context.Background(), 1, "property", nil)
	if err == nil {
		t.Fatal("expected an error")
	}
}
