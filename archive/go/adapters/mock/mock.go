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

	// FailUnbindIDs causes UnbindDevice to fail for these device IDs.
	FailUnbindIDs map[string]bool
	// FailRebindIDs causes RebindDevice to fail for these device IDs.
	FailRebindIDs map[string]bool
}

// NewSource creates a mock source with the given devices and automations.
func NewSource(devices []models.Device, autos []models.Automation) *Source {
	s := &Source{
		devices:       make(map[string]models.Device),
		autos:         make(map[string]models.Automation),
		unbound:       make(map[string]bool),
		FailUnbindIDs: make(map[string]bool),
		FailRebindIDs: make(map[string]bool),
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
	if s.FailUnbindIDs[deviceID] {
		return fmt.Errorf("simulated unbind failure for %s", deviceID)
	}
	s.unbound[deviceID] = true
	return nil
}

// IsUnbound returns true if the device has been unbound from source.
func (s *Source) IsUnbound(deviceID string) bool {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.unbound[deviceID]
}

// DeviceCount returns the total number of devices (bound + unbound).
func (s *Source) DeviceCount() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.devices)
}

func (s *Source) RebindDevice(_ context.Context, deviceID string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.FailRebindIDs[deviceID] {
		return fmt.Errorf("simulated rebind failure for %s", deviceID)
	}
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
	// FailVerifyIDs causes VerifyDevice to return false (device unreachable).
	FailVerifyIDs map[string]bool
	// FailAutoIDs causes CreateAutomation to fail for these automation IDs.
	FailAutoIDs map[string]bool
	// ErrorVerifyIDs causes VerifyDevice to return an error (not just false).
	ErrorVerifyIDs map[string]bool
	// FailRebindOnRollback causes RollbackDevice's source.RebindDevice to fail.
	// (Simulated via target: when UnbindDevice is called for rollback, it's best-effort)
}

// NewTarget creates an empty mock target.
func NewTarget() *Target {
	return &Target{
		devices:       make(map[string]models.Device),
		autos:         make(map[string]models.Automation),
		FailDeviceIDs: make(map[string]bool),
		FailVerifyIDs: make(map[string]bool),
		FailAutoIDs:   make(map[string]bool),
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
	if t.FailAutoIDs[auto.ID] {
		return fmt.Errorf("simulated automation creation failure for %s", auto.ID)
	}
	t.autos[auto.ID] = auto
	return nil
}

// ErrorVerifyIDs causes VerifyDevice to return an error (not just false).
func (t *Target) VerifyDevice(_ context.Context, deviceID string) (bool, error) {
	t.mu.Lock()
	defer t.mu.Unlock()
	if t.FailVerifyIDs[deviceID] {
		return false, nil
	}
	if t.ErrorVerifyIDs != nil && t.ErrorVerifyIDs[deviceID] {
		return false, fmt.Errorf("simulated verify error for %s", deviceID)
	}
	_, ok := t.devices[deviceID]
	return ok, nil
}

// ErrorVerifyIDs causes VerifyDevice to return an error (distinct from FailVerifyIDs which returns false, nil).
// Must be set directly on the struct.


// BoundAutomations returns all automations on the target.
func (t *Target) BoundAutomations() []models.Automation {
	t.mu.Lock()
	defer t.mu.Unlock()
	var result []models.Automation
	for _, a := range t.autos {
		result = append(result, a)
	}
	return result
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
