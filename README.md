[![License](https://img.shields.io/badge/license-MIT-blue)](https://opensource.org/licenses/MIT) [![Work In Progress](https://img.shields.io/badge/Status-Work%20In%20Progress-yellow)](https://guide.unitvectorylabs.com/bestpractices/status/#work-in-progress) [![Go Report Card](https://goreportcard.com/badge/github.com/UnitVectorY-Labs/pubsubmsgrestforwarder)](https://goreportcard.com/report/github.com/UnitVectorY-Labs/pubsubmsgrestforwarder)

# pubsubmsgrestforwarder

A Go command-line application for local testing, simulating the Cloud Run Push use case by consuming Pub/Sub messages and forwarding them as RESTful HTTP POST requests.

## Overview

This tool is intended for local testing and debugging of Pub/Sub integrations. It connects to a given Pub/Sub subscription, processes the messages, converts them into the format required by Cloud Run push subscriptions, and sends them to a specified RESTful HTTP endpoint.

The primary use case is to enable developers to simulate the behavior of Cloud Run push subscriptions locally with real Pub/Sub data.

## Configuration

This application is run as a command-line tool and supports the following configuration options:

### Command-Line Arguments

- `--project` (string, required): The GCP project ID associated with the Pub/Sub subscription.
- `--subscription` (string, required): The Pub/Sub subscription ID to consume messages from.
- `--url` (string, optional): The URL to which the transformed messages will be POSTed. (default: `http://localhost:8080`)

### Example Usage

```bash
./pubsubmsgrestforwarder --project=my-gcp-project --subscription=my-subscription-id --url=http://localhost:9090/webhook
```

This command consumes messages from the specified Pub/Sub subscription and forwards them to `http://localhost:9090/webhook`.

## Key Design

The application continuously consumes messages from the specified Pub/Sub subscription, transforms them into the following JSON format, and sends them to the configured URL:

```json
{
  "message": {
    "attributes": { "key1": "value1", "key2": "value2" },
    "data": "Base64EncodedData",
    "messageId": "1234567890",
    "orderingKey": "order-key",
    "publishTime": "2024-01-01T00:00:00Z"
  },
  "subscription": "projects/my-project/subscriptions/my-subscription"
}
```

## Limitations

- This application does not handle message retries on POST failures. Messages are Nacked and may be redelivered by Pub/Sub based on the subscription configuration.
- Only a single message is processed at a time. The application does not support batch processing or high-throughput scenarios.
- The tool is designed for local testing and does not include production-level security features.
