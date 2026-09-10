package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"time"

	"github.com/example/saas-dead-letter-worker/internal/infrai"
	"github.com/example/saas-dead-letter-worker/internal/jobs"
)

const (
	sourceQueue     = "failed-saas-jobs"
	deadLetterQueue = "failed-saas-jobs-dead-letter"
)

func main() {
	apiKey := os.Getenv("INFRAI_API_KEY")
	if apiKey == "" {
		log.Fatal("INFRAI_API_KEY is required")
	}
	client := infrai.New(apiKey)
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	messages, err := client.Consume(ctx, sourceQueue, 10, 60)
	if err != nil {
		log.Fatal(err)
	}
	for _, message := range messages {
		if err := handleFailure(ctx, client, message); err != nil {
			log.Printf("message %s: %v", message.MessageID, err)
		}
	}
}

func handleFailure(ctx context.Context, client *infrai.Client, message infrai.Message) error {
	var job jobs.Job
	if err := json.Unmarshal(message.Payload, &job); err != nil {
		return fmt.Errorf("decode job: %w", err)
	}
	if err := job.Validate(); err != nil {
		return err
	}
	if !jobs.ShouldDeadLetter(job) {
		log.Printf("retry pending: job=%s tenant=%s attempt=%d", job.ID, job.TenantID, job.Attempt)
		return nil
	}

	deadLetter := jobs.DeadLetter{
		OriginalJob: job,
		Reason:      "attempt limit reached",
		FailedAt:    time.Now().UTC().Format(time.RFC3339),
	}
	if err := client.Publish(ctx, deadLetterQueue, deadLetter, "dead-letter-"+job.ID); err != nil {
		return fmt.Errorf("publish dead letter: %w", err)
	}
	if err := client.Ack(ctx, sourceQueue, message.MessageID); err != nil {
		return fmt.Errorf("ack source message: %w", err)
	}
	log.Printf("dead-lettered: job=%s tenant=%s kind=%s", job.ID, job.TenantID, job.Kind)
	return nil
}
