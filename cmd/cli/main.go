// Package main provides a CLI to trigger and query migration workflows.
package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/junghan0611/durable-iot-migrate/core/models"
	"github.com/junghan0611/durable-iot-migrate/core/workflows"
	"go.temporal.io/sdk/client"
)

const taskQueue = "iot-migration"

func main() {
	if len(os.Args) < 2 {
		fmt.Println("Usage: migrate <start|status> [args]")
		fmt.Println("  start [batch-id]    Start a migration batch")
		fmt.Println("  status <workflow-id> Query migration status")
		os.Exit(1)
	}

	hostPort := os.Getenv("TEMPORAL_HOST")
	if hostPort == "" {
		hostPort = "localhost:7233"
	}

	c, err := client.Dial(client.Options{HostPort: hostPort})
	if err != nil {
		log.Fatalf("Unable to create Temporal client: %v", err)
	}
	defer c.Close()

	switch os.Args[1] {
	case "start":
		batchID := fmt.Sprintf("batch-%d", time.Now().Unix())
		if len(os.Args) > 2 {
			batchID = os.Args[2]
		}
		startMigration(c, batchID)
	case "status":
		if len(os.Args) < 3 {
			log.Fatal("Usage: migrate status <workflow-id>")
		}
		queryStatus(c, os.Args[2])
	default:
		log.Fatalf("Unknown command: %s", os.Args[1])
	}
}

func startMigration(c client.Client, batchID string) {
	config := models.BatchConfig{
		BatchID:          batchID,
		SourcePlatform:   "mock",
		TargetPlatform:   "mock",
		MigrateAutos:     true,
		Concurrency:      5,
		SuccessThreshold: 0.95,
	}

	workflowID := fmt.Sprintf("migration-%s", batchID)
	opts := client.StartWorkflowOptions{
		ID:        workflowID,
		TaskQueue: taskQueue,
	}

	we, err := c.ExecuteWorkflow(context.Background(), opts, workflows.DeviceMigrationWorkflow, config)
	if err != nil {
		log.Fatalf("Failed to start workflow: %v", err)
	}

	fmt.Printf("Migration started!\n")
	fmt.Printf("  Workflow ID: %s\n", we.GetID())
	fmt.Printf("  Run ID:      %s\n", we.GetRunID())
	fmt.Printf("  Batch ID:    %s\n", batchID)
	fmt.Printf("\nTrack progress:\n")
	fmt.Printf("  temporal workflow show -w %s\n", workflowID)
	fmt.Printf("  migrate status %s\n", workflowID)

	// Wait for result
	fmt.Printf("\nWaiting for completion...\n")
	var result models.BatchResult
	if err := we.Get(context.Background(), &result); err != nil {
		log.Fatalf("Workflow failed: %v", err)
	}

	out, _ := json.MarshalIndent(result, "", "  ")
	fmt.Printf("\nResult:\n%s\n", out)
	fmt.Printf("\nSuccess rate: %.1f%%\n", result.SuccessRate()*100)
}

func queryStatus(c client.Client, workflowID string) {
	resp, err := c.DescribeWorkflowExecution(context.Background(), workflowID, "")
	if err != nil {
		log.Fatalf("Failed to query workflow: %v", err)
	}

	info := resp.WorkflowExecutionInfo
	fmt.Printf("Workflow: %s\n", info.Execution.WorkflowId)
	fmt.Printf("Run ID:   %s\n", info.Execution.RunId)
	fmt.Printf("Status:   %s\n", info.Status)
	fmt.Printf("Started:  %s\n", info.StartTime)
	if info.CloseTime != nil {
		fmt.Printf("Closed:   %s\n", info.CloseTime)
	}
}
