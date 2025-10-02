// Copyright 2025 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//     http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package adapter

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

// SseEvent represents a single Server-Sent SseEvent.
type SseEvent struct {
	// Event is the event type name (defaults to "message" when not provided).
	Event string
	// Data is the payload (can contain newlines if multiple data lines were sent).
	Data string
	// ID is the event id (used for Last-Event-ID on reconnects).
	ID string
}

type SeeOptions struct {
	// InitialReconnectDelay is used as the base delay before reconnects
	// unless overridden by a "retry" field from the server (in milliseconds).
	InitialReconnectDelay time.Duration
	// MaxReconnectDelay caps the exponential backoff delay.
	MaxReconnectDelay time.Duration

	// Handlers
	OnOpen  func(*http.Response)
	OnEvent func(SseEvent)
	OnError func(error)
}

// SeeClient is a minimal SSE client with automatic reconnection and Last-Event-ID support.
type SeeClient struct {
	SeeOptions
	URL         string
	Header      http.Header
	HTTPClient  *http.Client
	LastEventID string

	// // InitialReconnectDelay is used as the base delay before reconnects
	// // unless overridden by a "retry" field from the server (in milliseconds).
	// InitialReconnectDelay time.Duration
	// // MaxReconnectDelay caps the exponential backoff delay.
	// MaxReconnectDelay time.Duration

	// // Handlers
	// OnOpen  func(*http.Response)
	// OnEvent func(SseEvent)
	// OnError func(error)
}

func NewSeeClient(url string, opts SeeOptions) *SeeClient {
	return &SeeClient{
		URL:         url,
		Header:      make(http.Header),
		HTTPClient:  &http.Client{},
		SeeOptions:  opts,
		LastEventID: "",
	}
}

// Run connects to the SSE endpoint and continuously reads events,
// automatically reconnecting with backoff on transient errors.
func (c *SeeClient) Run(ctx context.Context, lastEventID *string) error {
	if lastEventID != nil {
		c.LastEventID = *lastEventID
	}
	if c.HTTPClient == nil {
		// No global timeout for streaming. The default client has no Timeout.
		c.HTTPClient = &http.Client{}
	}
	if c.InitialReconnectDelay <= 0 {
		c.InitialReconnectDelay = 1 * time.Second
	}
	if c.MaxReconnectDelay <= 0 {
		c.MaxReconnectDelay = 30 * time.Second
	}
	backoff := c.InitialReconnectDelay

	for {
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.URL, nil)
		if err != nil {
			c.emitError(fmt.Errorf("build request: %w", err))
			if !c.sleepWithContext(ctx, backoff) {
				return ctx.Err()
			}
			backoff = nextBackoff(backoff, c.MaxReconnectDelay)
			continue
		}

		// Mandatory/typical headers for SSE
		if c.Header != nil {
			req.Header = c.Header.Clone()
		}
		req.Header.Set("Accept", "text/event-stream")
		req.Header.Set("Cache-Control", "no-cache")
		req.Header.Set("Connection", "keep-alive")
		if c.LastEventID != "" {
			req.Header.Set("Last-Event-ID", c.LastEventID)
		}
		// Helpful UA
		if req.Header.Get("User-Agent") == "" {
			req.Header.Set("User-Agent", "go-sse-client/1.0")
		}

		resp, err := c.HTTPClient.Do(req)
		if err != nil {
			c.emitError(fmt.Errorf("connect: %w", err))
			if !c.sleepWithContext(ctx, backoff) {
				return ctx.Err()
			}
			backoff = nextBackoff(backoff, c.MaxReconnectDelay)
			continue
		}

		// Ensure body closed on exit from this iteration.
		func() {
			defer resp.Body.Close()

			// Expect HTTP 200 + content-type beginning with text/event-stream
			ct := strings.ToLower(resp.Header.Get("Content-Type"))
			valid_ct := strings.HasPrefix(ct, "text/event-stream") || strings.HasPrefix(ct, "application/json") // some servers use this for SSE
			if resp.StatusCode != http.StatusOK || !valid_ct {
				// For some endpoints, there may be a redirect or auth page; surface a clear error.
				bodyPreview := limitedRead(resp.Body, 1024)
				c.emitError(fmt.Errorf("unexpected response: status=%d content-type=%q body-preview=%q", resp.StatusCode, ct, bodyPreview))
				return
			}

			if c.OnOpen != nil {
				c.OnOpen(resp)
			}

			// Read and parse the event stream.
			if err := c.readStream(ctx, resp.Body, &backoff); err != nil && err != context.Canceled {
				// readStream only returns non-nil error on hard failures (not normal EOF/reconnect).
				c.emitError(err)
			}
		}()

		// On normal disconnection or after finishing readStream, attempt reconnect unless context canceled.
		select {
		case <-ctx.Done():
			return ctx.Err()
		default:
		}

		// sleep according to backoff (already possibly adjusted by "retry" field via readStream)
		if !c.sleepWithContext(ctx, backoff) {
			return ctx.Err()
		}
		backoff = nextBackoff(backoff, c.MaxReconnectDelay)
	}
}

// readStream parses an SSE stream from r according to the WHATWG EventSource spec.
// It adjusts the provided backoff if a "retry" field is received from the server.
func (c *SeeClient) readStream(ctx context.Context, r io.Reader, backoff *time.Duration) error {
	reader := bufio.NewReader(r)

	var (
		eventName   string
		dataLines   []string
		eventID     string
		retryMillis *int // may be set by "retry" field
	)

	dispatch := func() {
		if len(dataLines) == 0 && eventName == "" && eventID == "" {
			return
		}
		ev := SseEvent{
			Event: "message",
			Data:  strings.Join(dataLines, "\n"),
			ID:    c.LastEventID, // default to last known id unless overridden by current event's id field
		}
		if eventName != "" {
			ev.Event = eventName
		}
		// If the event had its own id field, update both event and client's last id.
		if eventID != "" {
			ev.ID = eventID
			c.LastEventID = eventID
		}

		// Deliver
		if c.OnEvent != nil {
			c.OnEvent(ev)
		} else {
			// Default behavior: print to stdout
			log.Printf("event=%q id=%q data=%s\n", ev.Event, ev.ID, ev.Data)
		}

		// Reset per-event fields (id field does not persist across events)
		eventName = ""
		dataLines = dataLines[:0]
		eventID = ""
	}

	for {
		select {
		case <-ctx.Done():
			return context.Canceled
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			// Normal disconnect (EOF) is not an error; we'll reconnect upstream.
			if err == io.EOF || isNetTemporary(err) {
				return nil
			}
			return fmt.Errorf("read stream: %w", err)
		}

		// Trim CRLF
		line = strings.TrimRight(line, "\r\n")

		// Empty line indicates dispatch
		if line == "" {
			dispatch()
			// If server sent a retry directive, apply it to reconnection backoff base
			if retryMillis != nil {
				d := time.Duration(*retryMillis) * time.Millisecond
				if d <= 0 {
					d = c.InitialReconnectDelay
				}
				*backoff = d
				// Reset so it applies only once per receipt per spec
				retryMillis = nil
			}
			continue
		}

		// Comments (lines starting with ":") should be ignored
		if strings.HasPrefix(line, ":") {
			continue
		}

		// Parse "field: value" form
		var field, value string
		if idx := strings.IndexByte(line, ':'); idx == -1 {
			field = line
			value = ""
		} else {
			field = line[:idx]
			value = line[idx+1:]
			if strings.HasPrefix(value, " ") {
				value = value[1:]
			}
		}

		switch field {
		case "event":
			eventName = value
		case "data":
			dataLines = append(dataLines, value)
		case "id":
			// If the value is empty, the event id should be reset to empty string
			// per spec. We'll update on dispatch.
			eventID = value
		case "retry":
			// retry is in milliseconds
			if n, perr := parseInt(value); perr == nil && n >= 0 {
				retryMillis = &n
			}
			// ignore invalid retry values
		default:
			// unknown fields are ignored
		}
	}
}

func (c *SeeClient) emitError(err error) {
	if c.OnError != nil {
		c.OnError(err)
	} else {
		log.Printf("sse error: %v", err)
	}
}

func (c *SeeClient) sleepWithContext(ctx context.Context, d time.Duration) bool {
	if d <= 0 {
		return true
	}
	timer := time.NewTimer(d)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return false
	case <-timer.C:
		return true
	}
}

func nextBackoff(current, max time.Duration) time.Duration {
	// Exponential backoff with jitter
	next := current * 2
	if next > max {
		next = max
	}
	// add small jitter (+/-10%)
	jitter := time.Duration(int64(next) / 10)
	return next - jitter + time.Duration(randInt63n(int64(2*jitter+1)))
}

func isNetTemporary(err error) bool {
	// Heuristic; you can expand this as needed.
	type temporary interface{ Temporary() bool }
	if te, ok := err.(temporary); ok {
		return te.Temporary()
	}
	return false
}

func parseInt(s string) (int, error) {
	var n int
	for _, r := range s {
		if r < '0' || r > '9' {
			return 0, fmt.Errorf("invalid integer")
		}
		n = n*10 + int(r-'0')
	}
	return n, nil
}

// Simple xorshift-like PRNG for jitter to avoid importing math/rand
// (we just need a bit of variability; crypto-strength randomness not required).
var rngState uint64 = uint64(time.Now().UnixNano())

func randInt63n(n int64) int64 {
	if n <= 0 {
		return 0
	}
	// xorshift64*
	x := rngState + 0x9E3779B97F4A7C15
	x = (x ^ (x >> 30)) * 0xBF58476D1CE4E5B9
	x = (x ^ (x >> 27)) * 0x94D049BB133111EB
	x = x ^ (x >> 31)
	rngState = x
	// Convert to positive int63
	u := int64(x & ((1 << 63) - 1))
	return u % n
}

// limitedRead reads up to limit bytes and returns them as a string.
// Useful for previewing error bodies without blocking on streams.
func limitedRead(r io.Reader, limit int64) string {
	if limit <= 0 {
		return ""
	}
	lr := io.LimitReader(r, limit)
	b, _ := io.ReadAll(lr)
	return string(b)
}
