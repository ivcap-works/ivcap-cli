// Copyright 2026 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
//
// Unless required by applicable law or agreed to in writing, software
// distributed under the License is distributed on an "AS IS" BASIS,
// WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
// See the License for the specific language governing permissions and
// limitations under the License.

package mcp

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"sync"
	"time"
)

// mcpLogger handles JSON-RPC request/response logging to a file.
type mcpLogger struct {
	file *os.File
	mu   sync.Mutex
}

// newMCPLogger creates a new MCP logger that writes to a timestamped file in the specified directory.
func newMCPLogger(logDir string) (*mcpLogger, error) {
	// Ensure log directory exists
	if err := os.MkdirAll(logDir, 0750); err != nil {
		return nil, fmt.Errorf("failed to create log directory: %w", err)
	}

	// Create log file with timestamp
	timestamp := time.Now().Format("2006-01-02_15-04-05")
	logPath := filepath.Join(logDir, fmt.Sprintf("mcp-logging-%s.txt", timestamp))

	// #nosec G304 - logPath is constructed from user-specified log-dir flag and timestamp, which is acceptable for logging
	file, err := os.Create(logPath)
	if err != nil {
		return nil, fmt.Errorf("failed to create log file: %w", err)
	}

	logger := &mcpLogger{
		file: file,
	}

	// Write initial header
	logger.writeHeader()

	return logger, nil
}

// writeHeader writes the initial log file header.
func (l *mcpLogger) writeHeader() {
	l.mu.Lock()
	defer l.mu.Unlock()

	header := fmt.Sprintf("MCP JSON-RPC Logging Session\nStarted: %s\n%s\n\n",
		time.Now().Format("2006-01-02 15:04:05"),
		"================================================================================")
	_, _ = l.file.WriteString(header)
	_ = l.file.Sync()
}

// logRequest logs an incoming JSON-RPC request.
func (l *mcpLogger) logRequest(data []byte) {
	l.logMessage("REQUEST", data)
}

// logResponse logs an outgoing JSON-RPC response.
func (l *mcpLogger) logResponse(data []byte) {
	l.logMessage("RESPONSE", data)
}

// logMessage logs a JSON-RPC message with human-readable formatting.
func (l *mcpLogger) logMessage(messageType string, data []byte) {
	l.mu.Lock()
	defer l.mu.Unlock()

	// Write timestamp separator
	timestamp := time.Now().Format("2006-01-02 15:04:05")
	separator := fmt.Sprintf("\n%s ------\n", timestamp)
	_, _ = l.file.WriteString(separator)

	// Write message type header
	header := fmt.Sprintf("%s:\n", messageType)
	_, _ = l.file.WriteString(header)

	// Try to pretty-print JSON
	var jsonData interface{}
	if err := json.Unmarshal(data, &jsonData); err == nil {
		prettyJSON, err := json.MarshalIndent(jsonData, "", "  ")
		if err == nil {
			_, _ = l.file.Write(prettyJSON)
			_, _ = l.file.WriteString("\n")
		} else {
			// Fallback to raw data if pretty-print fails
			_, _ = l.file.Write(data)
			_, _ = l.file.WriteString("\n")
		}
	} else {
		// If not valid JSON, write raw data
		_, _ = l.file.Write(data)
		_, _ = l.file.WriteString("\n")
	}

	// Ensure data is written to disk
	_ = l.file.Sync()
}

// close closes the log file.
func (l *mcpLogger) close() error {
	l.mu.Lock()
	defer l.mu.Unlock()

	if l.file != nil {
		// Write closing footer
		footer := fmt.Sprintf("\n%s\nSession ended: %s\n",
			"================================================================================",
			time.Now().Format("2006-01-02 15:04:05"))
		_, _ = l.file.WriteString(footer)
		_ = l.file.Sync()
		return l.file.Close()
	}
	return nil
}

// loggingReader wraps an io.Reader to log all data read.
type loggingReader struct {
	r      io.Reader
	logger *mcpLogger
}

func (lr *loggingReader) Read(p []byte) (n int, err error) {
	n, err = lr.r.Read(p)
	if n > 0 && lr.logger != nil {
		lr.logger.logRequest(p[:n])
	}
	return n, err
}

// loggingWriter wraps an io.Writer to log all data written.
type loggingWriter struct {
	w      io.Writer
	logger *mcpLogger
}

func (lw *loggingWriter) Write(p []byte) (n int, err error) {
	if len(p) > 0 && lw.logger != nil {
		lw.logger.logResponse(p)
	}
	return lw.w.Write(p)
}
