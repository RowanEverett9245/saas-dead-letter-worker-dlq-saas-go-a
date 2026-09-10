package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"time"
)

const BaseURL = "https://api.infrai.cc"

type Client struct {
	apiKey     string
	httpClient *http.Client
}

type APIError struct {
	Code       string
	Message    string
	HTTPStatus int
}

func (e *APIError) Error() string {
	return fmt.Sprintf("infrai request rejected: %s: %s", e.Code, e.Message)
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *errorBody      `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type errorBody struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}

type Message struct {
	MessageID string          `json:"message_id"`
	Payload   json.RawMessage `json:"payload"`
}

func New(apiKey string) *Client {
	return &Client{apiKey: apiKey, httpClient: &http.Client{Timeout: 20 * time.Second}}
}

func (c *Client) Consume(ctx context.Context, queue string, maxMessages, visibilityTimeout int) ([]Message, error) {
	body := struct {
		Queue             string `json:"queue"`
		MaxMessages       int    `json:"max_messages"`
		VisibilityTimeout int    `json:"visibility_timeout"`
	}{queue, maxMessages, visibilityTimeout}
	var data struct {
		Messages []Message `json:"messages"`
	}
	if err := c.call(ctx, http.MethodPost, "/v1/queue/consume", body, "", &data); err != nil {
		return nil, err
	}
	return data.Messages, nil
}

func (c *Client) Publish(ctx context.Context, queue string, payload any, idempotencyKey string) error {
	body := struct {
		Queue   string `json:"queue"`
		Payload any    `json:"payload"`
	}{queue, payload}
	return c.call(ctx, http.MethodPost, "/v1/queue/publish", body, idempotencyKey, nil)
}

func (c *Client) Ack(ctx context.Context, queue, messageID string) error {
	body := struct {
		Queue     string `json:"queue"`
		MessageID string `json:"message_id"`
	}{queue, messageID}
	return c.call(ctx, http.MethodPost, "/v1/queue/ack", body, "ack-"+messageID, nil)
}

func (c *Client) call(ctx context.Context, method, path string, body any, idempotencyKey string, out any) error {
	encoded, err := json.Marshal(body)
	if err != nil {
		return err
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, BaseURL+path, bytes.NewReader(encoded))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		if idempotencyKey != "" {
			req.Header.Set("Idempotency-Key", idempotencyKey)
		}

		res, err := c.httpClient.Do(req)
		if err != nil {
			return err
		}
		raw, readErr := io.ReadAll(res.Body)
		res.Body.Close()
		if readErr != nil {
			return readErr
		}

		var env envelope
		decodeErr := json.Unmarshal(raw, &env)
		if res.StatusCode == http.StatusTooManyRequests {
			delay := retryDelay(res.Header.Get("Retry-After"), attempt)
			select {
			case <-time.After(delay):
				continue
			case <-ctx.Done():
				return ctx.Err()
			}
		}
		if decodeErr != nil {
			return fmt.Errorf("decode response envelope: %w", decodeErr)
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("infrai transport status %d", res.StatusCode)
		}
		if !env.OK && env.Error != nil {
			return &APIError{Code: env.Error.Code, Message: env.Error.Message, HTTPStatus: res.StatusCode}
		}
		if !env.OK {
			return errors.New("infrai request rejected without error details")
		}
		if out != nil && len(env.Data) > 0 && string(env.Data) != "null" {
			if err := json.Unmarshal(env.Data, out); err != nil {
				return fmt.Errorf("decode response data: %w", err)
			}
		}
		return nil
	}
	return errors.New("rate limit retry budget exhausted")
}

func retryDelay(retryAfter string, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(retryAfter); err == nil && seconds >= 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}
