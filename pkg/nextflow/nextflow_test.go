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

package nextflow

import (
	"testing"
)

func TestResolveServiceID_PrefersProvidedServiceID(t *testing.T) {
	tool := &ToolHeader{
		ServiceID: "urn:ivcap:service:from-tool",
		Name:      "test-pipeline",
	}

	serviceID, err := ResolveServiceID("urn:ivcap:service:provided-id", tool)
	if err != nil {
		t.Errorf("ResolveServiceID with valid provided ID should not error: %v", err)
	}

	if serviceID != "urn:ivcap:service:provided-id" {
		t.Errorf("ResolveServiceID with provided ID should use provided ID, got %q", serviceID)
	}
}

func TestResolveServiceID_ExtractsFromToolHeader(t *testing.T) {
	tool := &ToolHeader{
		ServiceID: "urn:ivcap:service:from-tool-header",
		Name:      "test-pipeline",
	}

	serviceID, err := ResolveServiceID("", tool)
	if err != nil {
		t.Errorf("ResolveServiceID with valid tool header should not error: %v", err)
	}

	if serviceID != "urn:ivcap:service:from-tool-header" {
		t.Errorf("ResolveServiceID should extract from tool header, got %q", serviceID)
	}
}

func TestResolveServiceID_RejectsInvalidProvidedID(t *testing.T) {
	tool := &ToolHeader{
		ServiceID: "urn:ivcap:service:from-tool",
		Name:      "test-pipeline",
	}

	_, err := ResolveServiceID("invalid-id", tool)
	if err == nil {
		t.Error("ResolveServiceID with invalid provided ID should error")
	}

	if err.Error() == "" {
		t.Error("Error message should not be empty")
	}
}

func TestResolveServiceID_RejectsInvalidToolServiceID(t *testing.T) {
	tool := &ToolHeader{
		ServiceID: "invalid-from-tool",
		Name:      "test-pipeline",
	}

	_, err := ResolveServiceID("", tool)
	if err == nil {
		t.Error("ResolveServiceID with invalid tool service-id should error")
	}

	if err.Error() == "" {
		t.Error("Error message should not be empty")
	}
}

func TestResolveServiceID_ErrorWhenBothMissing(t *testing.T) {
	tool := &ToolHeader{
		Name: "test-pipeline",
	}

	_, err := ResolveServiceID("", tool)
	if err == nil {
		t.Error("ResolveServiceID with missing both IDs should error")
	}
}

func TestResolveServiceID_ErrorWhenNilToolHeader(t *testing.T) {
	_, err := ResolveServiceID("", nil)
	if err == nil {
		t.Error("ResolveServiceID with nil tool header should error")
	}
}

func TestResolveServiceID_TrimsWhitespace(t *testing.T) {
	tool := &ToolHeader{
		ServiceID: "  urn:ivcap:service:with-whitespace  ",
		Name:      "test-pipeline",
	}

	serviceID, err := ResolveServiceID("", tool)
	if err != nil {
		t.Errorf("ResolveServiceID should trim whitespace and not error: %v", err)
	}

	if serviceID != "urn:ivcap:service:with-whitespace" {
		t.Errorf("ResolveServiceID should trim whitespace, got %q", serviceID)
	}
}
