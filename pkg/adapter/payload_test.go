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

package adapter

import (
	"testing"

	log "go.uber.org/zap"
)

func TestEnsureSchemaField(t *testing.T) {
	logger := log.NewNop()

	tests := []struct {
		name        string
		input       map[string]any
		expectAdded bool
	}{
		{
			name: "payload without $schema",
			input: map[string]any{
				"foo": "bar",
				"baz": 123,
			},
			expectAdded: true,
		},
		{
			name: "payload with existing $schema",
			input: map[string]any{
				"$schema": "urn:ivcap:schema:job.input.1",
				"foo":     "bar",
			},
			expectAdded: false,
		},
		{
			name:        "empty payload",
			input:       map[string]any{},
			expectAdded: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create payload from input
			pyld, err := JsonPayloadFromAny(tt.input, logger)
			if err != nil {
				t.Fatalf("failed to create payload: %v", err)
			}

			// Apply EnsureSchemaField
			result, err := EnsureSchemaField(pyld)
			if err != nil {
				t.Fatalf("EnsureSchemaField failed: %v", err)
			}

			// Check result
			obj, err := result.AsObject()
			if err != nil {
				t.Fatalf("failed to convert result to object: %v", err)
			}

			// Verify $schema exists
			schema, hasSchema := obj["$schema"]
			if !hasSchema {
				t.Fatal("$schema field is missing in result")
			}

			// If we expected it to be added, check it's the default value
			if tt.expectAdded {
				if schema != "urn:unknown:unknown" {
					t.Errorf("expected $schema to be 'urn:unknown:unknown', got '%v'", schema)
				}
			} else {
				// If we didn't expect it to be added, verify original value preserved
				if schema != tt.input["$schema"] {
					t.Errorf("expected $schema to be '%v', got '%v'", tt.input["$schema"], schema)
				}
			}

			// Verify other fields are preserved
			// Note: JSON unmarshaling may convert numbers to float64
			for key, expectedValue := range tt.input {
				if key == "$schema" {
					continue
				}
				actualValue, exists := obj[key]
				if !exists {
					t.Errorf("field %s is missing", key)
					continue
				}

				// Handle numeric type conversions (int -> float64 in JSON)
				switch expected := expectedValue.(type) {
				case int:
					if actual, ok := actualValue.(float64); ok {
						if float64(expected) != actual {
							t.Errorf("field %s was modified: expected %v, got %v", key, expected, actual)
						}
					} else if actualValue != expectedValue {
						t.Errorf("field %s was modified: expected %v (%T), got %v (%T)", key, expectedValue, expectedValue, actualValue, actualValue)
					}
				default:
					if actualValue != expectedValue {
						t.Errorf("field %s was modified: expected %v, got %v", key, expectedValue, actualValue)
					}
				}
			}
		})
	}
}
