// SPDX-License-Identifier: Apache-2.0 OR MIT

package xinput

import (
	"context"
	"fmt"
	"os/exec"
	"regexp"
	"sort"
	"strconv"
	"strings"
	"time"
)

var (
	deviceIDPattern = regexp.MustCompile(`\bid=([0-9]+)\b`)
	propertyPattern = regexp.MustCompile(`^\s*(.+?) \([0-9]+\):\s*(.*)$`)
)

type Runner interface {
	Run(ctx context.Context, command string, args ...string) ([]byte, error)
}

type ExecRunner struct{}

func (ExecRunner) Run(ctx context.Context, command string, args ...string) ([]byte, error) {
	return exec.CommandContext(ctx, command, args...).CombinedOutput()
}

type Client struct {
	Command string
	Runner  Runner
}

func NewClient() Client {
	return Client{Command: "xinput", Runner: ExecRunner{}}
}

type Property struct {
	Name   string
	Values []string
}

func (p Property) Floats() ([]float64, error) {
	values := make([]float64, len(p.Values))
	for i, raw := range p.Values {
		value, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return nil, fmt.Errorf("%s value %q is not a float: %w", p.Name, raw, err)
		}
		values[i] = value
	}
	return values, nil
}

func (p Property) Integers() ([]int64, error) {
	values := make([]int64, len(p.Values))
	for i, raw := range p.Values {
		value, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return nil, fmt.Errorf("%s value %q is not an integer: %w", p.Name, raw, err)
		}
		values[i] = value
	}
	return values, nil
}

type Device struct {
	ID         int
	Name       string
	Properties map[string]Property
}

func (d Device) Property(name string) (Property, bool) {
	property, ok := d.Properties[name]
	return property, ok
}

func (d Device) ProductID() (vendor uint16, product uint16, ok bool) {
	property, exists := d.Property("Device Product ID")
	if !exists {
		return 0, 0, false
	}
	values, err := property.Integers()
	if err != nil || len(values) != 2 ||
		values[0] < 0 || values[0] > 0xffff ||
		values[1] < 0 || values[1] > 0xffff {
		return 0, 0, false
	}
	return uint16(values[0]), uint16(values[1]), true
}

func (c Client) ListDevices(ctx context.Context) ([]Device, error) {
	var lastErr error
	for attempt := 0; attempt < 3; attempt++ {
		devices, err := c.listDevicesOnce(ctx)
		if err == nil {
			return devices, nil
		}
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		lastErr = err
		if attempt < 2 {
			timer := time.NewTimer(50 * time.Millisecond)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
	}
	return nil, fmt.Errorf("xinput inventory did not stabilize after 3 attempts: %w", lastErr)
}

func (c Client) listDevicesOnce(ctx context.Context) ([]Device, error) {
	output, err := c.run(ctx, "list", "--short")
	if err != nil {
		return nil, err
	}

	seen := make(map[int]bool)
	var ids []int
	for _, match := range deviceIDPattern.FindAllStringSubmatch(string(output), -1) {
		id, conversionErr := strconv.Atoi(match[1])
		if conversionErr != nil || seen[id] {
			continue
		}
		seen[id] = true
		ids = append(ids, id)
	}
	sort.Ints(ids)

	devices := make([]Device, 0, len(ids))
	for _, id := range ids {
		device, getErr := c.GetDevice(ctx, id)
		if getErr != nil {
			return nil, getErr
		}
		devices = append(devices, device)
	}
	return devices, nil
}

func (c Client) GetDevice(ctx context.Context, id int) (Device, error) {
	output, err := c.run(ctx, "list-props", strconv.Itoa(id))
	if err != nil {
		return Device{}, err
	}
	device, err := ParseProperties(id, string(output))
	if err != nil {
		return Device{}, fmt.Errorf("parse xinput device %d: %w", id, err)
	}
	return device, nil
}

func (c Client) SetProperty(
	ctx context.Context,
	deviceID int,
	property string,
	values []string,
) error {
	if len(values) == 0 {
		return fmt.Errorf("refusing to set %q without values", property)
	}
	args := []string{"set-prop", strconv.Itoa(deviceID), property}
	args = append(args, values...)
	_, err := c.run(ctx, args...)
	return err
}

func (c Client) run(ctx context.Context, args ...string) ([]byte, error) {
	command := c.Command
	if command == "" {
		command = "xinput"
	}
	runner := c.Runner
	if runner == nil {
		runner = ExecRunner{}
	}
	output, err := runner.Run(ctx, command, args...)
	if err != nil {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}
		detail := strings.TrimSpace(string(output))
		if detail == "" {
			return nil, fmt.Errorf("%s %s: %w", command, strings.Join(args, " "), err)
		}
		return nil, fmt.Errorf("%s %s: %w: %s",
			command, strings.Join(args, " "), err, detail)
	}
	return output, nil
}

func ParseProperties(id int, output string) (Device, error) {
	lines := strings.Split(output, "\n")
	if len(lines) == 0 {
		return Device{}, fmt.Errorf("empty property output")
	}

	header := strings.TrimSpace(lines[0])
	if !strings.HasPrefix(header, "Device '") || !strings.HasSuffix(header, "':") {
		return Device{}, fmt.Errorf("unexpected header %q", header)
	}
	name := strings.TrimSuffix(strings.TrimPrefix(header, "Device '"), "':")
	if name == "" {
		return Device{}, fmt.Errorf("device name is empty")
	}

	device := Device{
		ID:         id,
		Name:       name,
		Properties: make(map[string]Property),
	}
	for _, line := range lines[1:] {
		if strings.TrimSpace(line) == "" {
			continue
		}
		match := propertyPattern.FindStringSubmatch(line)
		if match == nil {
			continue
		}
		propertyName := strings.TrimSpace(match[1])
		rawValues := strings.TrimSpace(match[2])
		var values []string
		if rawValues != "" && rawValues != "<no items>" {
			for _, raw := range strings.Split(rawValues, ",") {
				values = append(values, strings.Trim(strings.TrimSpace(raw), `"`))
			}
		}
		device.Properties[propertyName] = Property{
			Name:   propertyName,
			Values: values,
		}
	}
	return device, nil
}
