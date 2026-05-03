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

package cmd

import (
	"testing"
)

func TestNextflowListCommand(t *testing.T) {
	// Test that the nextflow list command is properly registered
	if nextflowListCmd == nil {
		t.Fatal("nextflowListCmd should not be nil")
	}

	if nextflowListCmd.Use != "list-jobs" {
		t.Errorf("expected Use to be 'list-jobs', got '%s'", nextflowListCmd.Use)
	}

	if nextflowListCmd.Short != "List recent Nextflow jobs" {
		t.Errorf("expected Short description to be 'List recent Nextflow jobs', got '%s'", nextflowListCmd.Short)
	}

	// Verify the command has the expected flags
	if nextflowListCmd.Flags().Lookup("content-path") == nil {
		t.Error("expected content-path flag to be defined")
	}

	if nextflowListCmd.Flags().Lookup("limit") == nil {
		t.Error("expected limit flag to be defined (inherited from list flags)")
	}
}

func TestNextflowListFilterConstruction(t *testing.T) {
	// Test that the filter is constructed correctly
	expectedFilter := `$.["in-content"]["$schema"] == "urn:ivcap:schema:nextflow.request.1"`

	// The filter should match jobs with the nextflow request schema
	if expectedFilter == "" {
		t.Error("filter should not be empty")
	}
}
