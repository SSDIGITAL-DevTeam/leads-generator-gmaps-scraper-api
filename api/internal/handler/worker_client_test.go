package handler

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestWorkerClient_PostJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("X-Request-ID") != "req-1" {
			w.WriteHeader(http.StatusBadRequest)
			return
		}
		json.NewEncoder(w).Encode(map[string]any{"data": map[string]any{"status": "queued"}})
	}))
	defer server.Close()

	client := NewWorkerClient(server.Client(), server.URL)
	data, err := client.PostJSON(context.Background(), "/test", map[string]string{"foo": "bar"}, "req-1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if data["status"] != "queued" {
		t.Fatalf("expected queued, got %v", data)
	}
}
