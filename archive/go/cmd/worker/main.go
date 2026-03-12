// Package main starts a Temporal worker for IoT migration.
package main

import (
	"log"
	"os"

	"github.com/junghan0611/durable-iot-migrate/adapters/mock"
	"github.com/junghan0611/durable-iot-migrate/core/activities"
	"github.com/junghan0611/durable-iot-migrate/core/models"
	"github.com/junghan0611/durable-iot-migrate/core/workflows"
	"go.temporal.io/sdk/client"
	"go.temporal.io/sdk/worker"
)

const taskQueue = "iot-migration"

func main() {
	hostPort := os.Getenv("TEMPORAL_HOST")
	if hostPort == "" {
		hostPort = "localhost:7233"
	}

	c, err := client.Dial(client.Options{HostPort: hostPort})
	if err != nil {
		log.Fatalf("Unable to create Temporal client: %v", err)
	}
	defer c.Close()

	// For development: use mock adapters with sample devices
	source := mock.NewSource(sampleDevices(), nil)
	target := mock.NewTarget()

	acts := &activities.MigrationActivities{
		Source: source,
		Target: target,
	}

	w := worker.New(c, taskQueue, worker.Options{})

	// Register workflows
	w.RegisterWorkflow(workflows.DeviceMigrationWorkflow)

	// Register activities
	w.RegisterActivity(acts.FetchDevices)
	w.RegisterActivity(acts.FetchAutomations)
	w.RegisterActivity(acts.MigrateDevice)
	w.RegisterActivity(acts.RollbackDevice)

	log.Printf("Starting worker on task queue: %s", taskQueue)
	log.Printf("Mock source: %d devices loaded", len(sampleDevices()))

	if err := w.Run(worker.InterruptCh()); err != nil {
		log.Fatalf("Worker failed: %v", err)
	}
}

func sampleDevices() []models.Device {
	return []models.Device{
		{ID: "dev-001", Name: "Living Room Light", Category: "light", Protocol: "zigbee", Online: true},
		{ID: "dev-002", Name: "Front Door Sensor", Category: "sensor", Protocol: "zigbee", Online: true},
		{ID: "dev-003", Name: "Kitchen Smart Plug", Category: "switch", Protocol: "wifi", Online: true},
		{ID: "dev-004", Name: "Bedroom Thermostat", Category: "thermostat", Protocol: "thread", Online: true},
		{ID: "dev-005", Name: "Garage Door Lock", Category: "lock", Protocol: "matter", Online: true},
	}
}
