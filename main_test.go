package main

import (
	"encoding/json/v2"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"cloud.google.com/go/pubsub"
)

func TestSendPOSTMessage(t *testing.T) {
	for _, tc := range []struct {
		name       string
		attributes map[string]string
		ordering   string
	}{
		{name: "nil attributes"},
		{name: "attributes and ordering key", attributes: map[string]string{"key": "value"}, ordering: "ordered"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPost || r.Header.Get("Content-Type") != "application/json" {
					t.Errorf("unexpected request: %s, content type %q", r.Method, r.Header.Get("Content-Type"))
				}
				body, err := io.ReadAll(r.Body)
				if err != nil {
					t.Error(err)
					return
				}
				var got struct {
					Message struct {
						Attributes  map[string]string `json:"attributes"`
						Data        string            `json:"data"`
						MessageID   string            `json:"messageId"`
						OrderingKey *string           `json:"orderingKey"`
						PublishTime string            `json:"publishTime"`
					} `json:"message"`
					Subscription string `json:"subscription"`
				}
				if err := json.Unmarshal(body, &got); err != nil {
					t.Fatalf("invalid JSON: %v", err)
				}
				if got.Message.Attributes == nil || len(got.Message.Attributes) != len(tc.attributes) || got.Message.Attributes["key"] != tc.attributes["key"] {
					t.Errorf("attributes = %#v, want object containing %#v", got.Message.Attributes, tc.attributes)
				}
				if got.Message.Data != "aGVsbG8=" || got.Message.MessageID != "123" || got.Message.PublishTime != "2024-01-02T03:04:05Z" || got.Subscription != "projects/project/subscriptions/sub" {
					t.Errorf("unexpected payload: %s", body)
				}
				if tc.ordering == "" && got.Message.OrderingKey != nil || tc.ordering != "" && (got.Message.OrderingKey == nil || *got.Message.OrderingKey != tc.ordering) {
					t.Errorf("ordering key = %v, want %q", got.Message.OrderingKey, tc.ordering)
				}
			}))
			defer server.Close()
			msg := &pubsub.Message{Data: []byte("hello"), ID: "123", Attributes: tc.attributes, OrderingKey: tc.ordering, PublishTime: time.Date(2024, 1, 2, 3, 4, 5, 0, time.UTC)}
			if err := sendPOST(server.URL, transformMessage(msg, &Config{Project: "project", Subscription: "sub"})); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestSendPOSTRejectsInvalidUTF8(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Error("invalid payload was sent")
	}))
	defer server.Close()
	payload := &PubSubMessage{}
	payload.Message.Attributes = map[string]string{"key": string([]byte{0xff})}
	if err := sendPOST(server.URL, payload); err == nil || !strings.Contains(err.Error(), "failed to marshal JSON payload") {
		t.Fatalf("sendPOST error = %v, want marshaling error", err)
	}
}
