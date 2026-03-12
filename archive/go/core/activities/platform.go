// Package activities defines the interfaces that platform adapters must implement.
package activities

import (
	"context"

	"github.com/junghan0611/durable-iot-migrate/core/models"
)

// SourcePlatform reads devices and automations from the migration source.
// Implementations must be idempotent — the same call should return the same result.
type SourcePlatform interface {
	// ListDevices returns all devices available for migration.
	ListDevices(ctx context.Context) ([]models.Device, error)

	// ListAutomations returns all automations available for migration.
	ListAutomations(ctx context.Context) ([]models.Automation, error)

	// UnbindDevice removes a device from the source platform.
	// Must be idempotent: calling twice for the same device should not error.
	UnbindDevice(ctx context.Context, deviceID string) error

	// RebindDevice restores a device to the source platform (compensation/rollback).
	RebindDevice(ctx context.Context, deviceID string) error
}

// TargetPlatform writes devices and automations to the migration target.
// Implementations must be idempotent.
type TargetPlatform interface {
	// BindDevice registers a device on the target platform.
	BindDevice(ctx context.Context, device models.Device) error

	// CreateAutomation creates an automation on the target platform.
	CreateAutomation(ctx context.Context, auto models.Automation) error

	// VerifyDevice checks that a migrated device is reachable and functional.
	VerifyDevice(ctx context.Context, deviceID string) (bool, error)

	// UnbindDevice removes a device from the target platform (compensation/rollback).
	UnbindDevice(ctx context.Context, deviceID string) error

	// DeleteAutomation removes an automation from the target platform (compensation/rollback).
	DeleteAutomation(ctx context.Context, autoID string) error
}
