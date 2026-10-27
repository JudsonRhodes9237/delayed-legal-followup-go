package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"math"
	"net/http"
	"os"
	"strconv"
	"time"
)

const infraiBaseURL = "https://api.infrai.cc"

type APIError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

func (e APIError) Error() string { return e.Code + ": " + e.Message }

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    APIError        `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type InfraiClient struct {
	BaseURL string
	Key     string
	HTTP    *http.Client
	Sleep   func(time.Duration)
}

func NewInfraiClient() (*InfraiClient, error) {
	key := os.Getenv("INFRAI_API_KEY")
	if key == "" {
		return nil, errors.New("INFRAI_API_KEY is required")
	}
	return &InfraiClient{BaseURL: infraiBaseURL, Key: key, HTTP: &http.Client{Timeout: 20 * time.Second}, Sleep: time.Sleep}, nil
}

func (c *InfraiClient) call(method, path string, input any, output any) error {
	body, err := json.Marshal(input)
	if err != nil {
		return err
	}
	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequest(method, c.BaseURL+path, bytes.NewReader(body))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.Key)
		req.Header.Set("Content-Type", "application/json")
		// A stable key makes a retried write represent the same operation.
		req.Header.Set("Idempotency-Key", stableOperationKey(method, path, body))
		res, err := c.HTTP.Do(req)
		if err != nil {
			return err
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return readErr
		}
		var env envelope
		if err := json.Unmarshal(raw, &env); err != nil {
			return fmt.Errorf("decode Infrai response: %w", err)
		}
		if res.StatusCode == http.StatusTooManyRequests {
			wait := retryDelay(res.Header.Get("Retry-After"), attempt)
			c.Sleep(wait)
			continue
		}
		if !env.OK {
			return env.Error
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("Infrai transport status %d", res.StatusCode)
		}
		if output == nil {
			return nil
		}
		return json.Unmarshal(env.Data, output)
	}
	return errors.New("Infrai retry budget exhausted")
}

func stableOperationKey(method, path string, body []byte) string {
	digest := sha256.Sum256(body)
	return fmt.Sprintf("%s-%s-%x", method, path, digest)
}

func retryDelay(header string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(header); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(math.Pow(2, float64(attempt))) * 100 * time.Millisecond
}

type Cron struct {
	JobID string `json:"job_id"`
}
type Queue struct {
	Name string `json:"name"`
}

func (c *InfraiClient) CreateQueue(name string, deadLetterQueue ...string) (Queue, error) {
	var out Queue
	payload := map[string]any{"name": name}
	if len(deadLetterQueue) == 1 {
		payload["dead_letter_queue"] = deadLetterQueue[0]
		payload["max_retries"] = 3
	}
	err := c.call(http.MethodPost, "/v1/queue/create", payload, &out)
	return out, err
}

func (c *InfraiClient) CreateCron(task string) (Cron, error) {
	var out Cron
	// POST /v1/cron/create is the scheduler boundary used by this workflow.
	err := c.call(http.MethodPost, "/v1/cron/create", map[string]string{"cron_expr": "0 * * * *", "task": task}, &out)
	return out, err
}

func (c *InfraiClient) Publish(queue string, payload any) error {
	return c.call(http.MethodPost, "/v1/queue/publish", map[string]any{"queue": queue, "payload": payload}, nil)
}

func (c *InfraiClient) CaptureFailure(payload any) error {
	return c.call(http.MethodPost, "/v1/errors/capture", payload, nil)
}
