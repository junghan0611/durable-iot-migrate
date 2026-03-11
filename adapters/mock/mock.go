// Package mock provides in-memory Source/Target platform adapters for testing.
package mock

import (
	"context"
	"fmt"
	"sync"

	"github.com/junghan0611/durable-iot-migrate/core/models"
)

// Source simulates a source IoT platform with pre-loaded devices.
type Source struct {
	mu      sync.Mutex
	devices map[string]models.Device
	autos   map[string]models.Automation
	unbound map[string]bool
}

// NewSource creates a mock source with the given devices and automations.
func NewSource(devices []models.Device, autos []models.Automation) *Source {
	s := &Source{
		devices: make(map[string]models.Device),
		autos:   make(map[string]models.Automation),
		unbound: make(map[string]bool),
	}
	for _, d := range devices {
		s.devices[d.ID] = d
	}
	for _, a := range autos {
		s.autos[a.ID] = a
	}
	return s
}

func (s *Source) ListDevices(_ context.Context) ([]models.Device, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []models.Device
	for _, d := range s.devices {
		if !s.unbound[d.ID] {
			result = append(result, d)
		}
	}
	return result, nil
}

func (s *Source) ListAutomations(_ context.Context) ([]models.Automation, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var result []models.Automation
	for _, a := range s.autos {
		result = append(result, a)
	}
	return result, nil
}

func (s *Source) UnbindDevice(_ context.Context, deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if _, ok := s.devices[deviceID]; !ok {
		return fmt.Errorf("device %s not found", deviceID)
	}
	s.unbound[deviceID] = true
	return nil
}

func (s *Source) RebindDevice(_ context.Context, deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	delete(s.unbound, deviceID)
	return nil
}

// Target simulates a target IoT platform.
type Target struct {
	mu      sync.Mutex
	devices map[string]models.Device
	autos   map[string]models.Automation

	// FailDeviceIDs causes BindDevice to fail for these device IDs (for testing).
	FailDeviceIDs map[string]bool
}

// NewTarget creates an empty mock target.
func NewTarget() *Target {
	return &Target{
		devices:       make(map[string]models.Device),
		autos:         make(map[string]models.Automation),
		FailDeviceIDs: make(map[string]bool),
	}
}

func (t *Target) BindDevice(_ context.Context, device models.Device) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.FailDeviceIDs[device.ID] {
		return fmt.Errorf("simulated bind failure for %s", device.ID)
	}
	t.devices[device.ID] = device
	return nil
}

func (t *Target) CreateAutomation(_ context.Context, auto models.Automation) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	t.autos[auto.ID] = auto
	return nil
}

func (t *Target) VerifyDevice(_ context.Context, deviceID string) (bool, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	_, ok := t.devices[deviceID]
	return ok, nil
}

func (t *Target) UnbindDevice(_ context.Context, deviceID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.devices, deviceID)
	return nil
}

func (t *Target) DeleteAutomation(_ context.Context, autoID string) error {
	t.mu.Lock()
	defer t.mu.Unlock()
	delete(t.autos, autoID)
	return nil
}

// BoundDevices returns all devices currently bound to the target.
func (t *Target) BoundDevices() []models.Device {
	t.mu.Lock()
	defer t.mu.Unlock()
	var result []models.Device
	for _, d := range t.devices {
		result = append(result, d)
	}
	return result
}
