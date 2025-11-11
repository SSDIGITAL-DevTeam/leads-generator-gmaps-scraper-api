package handler

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"google.golang.org/api/idtoken"
)

// WorkerClient posts JSON payloads to worker endpoints.
type WorkerPoster interface {
	PostJSON(ctx context.Context, path string, payload any, requestID string) (map[string]any, error)
}

type WorkerClient struct {
	client  *http.Client
	baseURL string
}

// NewWorkerClient builds a worker client, auto-configuring an ID token client when needed.
func NewWorkerClient(client *http.Client, workerBaseURL string) *WorkerClient {
	if workerBaseURL == "" {
		panic("workerBaseURL must not be empty")
	}
	workerBaseURL = strings.TrimRight(workerBaseURL, "/")
	if client == nil {
		idc, err := idtoken.NewClient(context.Background(), workerBaseURL)
		if err != nil {
			client = &http.Client{Timeout: 10 * time.Second}
		} else {
			client = idc
		}
	}
	return &WorkerClient{client: client, baseURL: workerBaseURL}
}

// PostJSON posts the payload to the worker and returns the "data" object.
func (c *WorkerClient) PostJSON(ctx context.Context, path string, payload any, requestID string) (map[string]any, error) {
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal payload: %w", err)
	}

	url := c.baseURL + path
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("failed to create worker request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	if requestID != "" {
		req.Header.Set("X-Request-ID", requestID)
	}

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("worker request failed: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		errMsg := extractWorkerError(resp.Body)
		return nil, fmt.Errorf("worker error: %s", errMsg)
	}

	var workerResp struct {
		Data  map[string]any `json:"data"`
		Error string         `json:"error"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&workerResp); err != nil && err != io.EOF {
		return nil, fmt.Errorf("could not decode worker response: %w", err)
	}
	if workerResp.Error != "" {
		return nil, fmt.Errorf("worker error: %s", workerResp.Error)
	}
	return workerResp.Data, nil
}

var _ WorkerPoster = (*WorkerClient)(nil)
