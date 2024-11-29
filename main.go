package main

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"strings"
	"time"

	"cloud.google.com/go/pubsub"
)

// Config holds the configuration parsed from command-line arguments
type Config struct {
	Project      string
	Subscription string
	URL          string
}

// PubSubMessage represents the transformed Pub/Sub message structure
type PubSubMessage struct {
	Message struct {
		Attributes  map[string]string `json:"attributes"`
		Data        string            `json:"data"`
		MessageID   string            `json:"messageId"`
		OrderingKey string            `json:"orderingKey,omitempty"`
		PublishTime string            `json:"publishTime"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

// parseFlags parses and validates comma`nd-line arguments
func parseFlags() (*Config, error) {
	project := flag.String("project", "", "GCP project ID (required)")
	subscription := flag.String("subscription", "", "Pub/Sub subscription ID (required)")
	url := flag.String("url", "http://localhost:8080", "URL to POST messages to (optional)")

	flag.Parse()

	if *project == "" {
		return nil, fmt.Errorf("missing required argument: --project")
	}
	if *subscription == "" {
		return nil, fmt.Errorf("missing required argument: --subscription")
	}

	return &Config{
		Project:      *project,
		Subscription: *subscription,
		URL:          *url,
	}, nil
}

// setupPubSubClient initializes the Pub/Sub client and subscription
func setupPubSubClient(ctx context.Context, cfg *Config) (*pubsub.Client, *pubsub.Subscription, error) {
	client, err := pubsub.NewClient(ctx, cfg.Project)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create Pub/Sub client: %w", err)
	}

	sub := client.Subscription(cfg.Subscription)
	exists, err := sub.Exists(ctx)
	if err != nil {
		client.Close()
		return nil, nil, fmt.Errorf("failed to verify subscription existence: %w", err)
	}
	if !exists {
		client.Close()
		return nil, nil, fmt.Errorf("subscription %s does not exist", cfg.Subscription)
	}

	log.Printf("Connected to Pub/Sub subscription: %s", cfg.Subscription)
	return client, sub, nil
}

// transformMessage converts a Pub/Sub message into the desired JSON structure
func transformMessage(msg *pubsub.Message, cfg *Config) *PubSubMessage {
	transformed := &PubSubMessage{}
	transformed.Message.Attributes = msg.Attributes
	transformed.Message.Data = base64.StdEncoding.EncodeToString(msg.Data)
	transformed.Message.MessageID = msg.ID
	transformed.Message.OrderingKey = msg.OrderingKey
	transformed.Message.PublishTime = msg.PublishTime.Format(time.RFC3339)
	transformed.Subscription = fmt.Sprintf("projects/%s/subscriptions/%s", cfg.Project, cfg.Subscription)
	return transformed
}

// sendPOST sends the transformed message to the specified URL via HTTP POST
func sendPOST(url string, payload *PubSubMessage) error {
	jsonData, err := json.Marshal(payload)
	if err != nil {
		return fmt.Errorf("failed to marshal JSON payload: %w", err)
	}

	req, err := http.NewRequest("POST", url, strings.NewReader(string(jsonData)))
	if err != nil {
		return fmt.Errorf("failed to create POST request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{
		Timeout: 10 * time.Second,
	}
	resp, err := client.Do(req)
	if err != nil {
		return fmt.Errorf("POST request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		log.Println("Message processed successfully.")
	} else {
		return fmt.Errorf("failed to process message. HTTP Status: %s", resp.Status)
	}

	return nil
}

// consumeMessages continuously receives and processes Pub/Sub messages
func consumeMessages(ctx context.Context, sub *pubsub.Subscription, cfg *Config) error {
	err := sub.Receive(ctx, func(ctx context.Context, msg *pubsub.Message) {
		transformed := transformMessage(msg, cfg)
		err := sendPOST(cfg.URL, transformed)
		if err != nil {
			log.Printf("Error processing message ID %s: %v", msg.ID, err)
			// Nack the message to allow redelivery
			msg.Nack()
			return
		}
		// Acknowledge the message upon successful processing
		msg.Ack()
	})

	if err != nil && err != context.Canceled {
		return fmt.Errorf("error receiving messages: %w", err)
	}

	return nil
}

// handleShutdown listens for interrupt signals and cancels the context for graceful shutdown
func handleShutdown(cancelFunc context.CancelFunc) {
	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, os.Interrupt)

	<-sigChan
	log.Println("Shutdown signal received. Initiating graceful shutdown...")
	cancelFunc()
}

func main() {
	// Parse command-line arguments
	cfg, err := parseFlags()
	if err != nil {
		log.Fatalf("Argument parsing error: %v", err)
	}

	log.Printf("Starting Pub/Sub Tester. Project: %s, Subscription: %s, POST URL: %s",
		cfg.Project, cfg.Subscription, cfg.URL)

	// Set up context with cancellation
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Handle graceful shutdown in a separate goroutine
	go handleShutdown(cancel)

	// Initialize Pub/Sub client and subscription
	client, sub, err := setupPubSubClient(ctx, cfg)
	if err != nil {
		log.Fatalf("Pub/Sub setup error: %v", err)
	}
	defer func() {
		if err := client.Close(); err != nil {
			log.Printf("Error closing Pub/Sub client: %v", err)
		}
	}()

	// Start consuming messages
	if err := consumeMessages(ctx, sub, cfg); err != nil {
		log.Fatalf("Message consumption error: %v", err)
	}

	log.Println("Graceful shutdown complete. Exiting application.")
}
