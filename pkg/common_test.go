// Copyright 2024 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
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

package client

import (
	"strings"
	"testing"
)

func TestValidateEntityURN(t *testing.T) {
	tests := []struct {
		name    string
		entity  string
		wantErr bool
		errMsg  string
	}{
		{
			name:    "valid service URN with UUID",
			entity:  "urn:ivcap:service:a98b81a8-9279-509f-9c0e-40d39e83058a",
			wantErr: false,
		},
		{
			name:    "valid artifact URN with UUID",
			entity:  "urn:ivcap:artifact:714e549b-ebab-5dd8-8ebd-2e4b0af76167",
			wantErr: false,
		},
		{
			name:    "valid job URN with UUID",
			entity:  "urn:ivcap:job:123e4567-e89b-12d3-a456-426614174000",
			wantErr: false,
		},
		{
			name:    "valid aspect URN with UUID",
			entity:  "urn:ivcap:aspect:550e8400-e29b-41d4-a716-446655440000",
			wantErr: false,
		},
		{
			name:    "valid queue URN with UUID",
			entity:  "urn:ivcap:queue:714e549b-ebab-5dd8-8ebd-2e4b0af76167",
			wantErr: false,
		},
		{
			name:    "valid UUID without hyphens",
			entity:  "urn:ivcap:service:a98b81a8927950f9c0e40d39e83058a",
			wantErr: false,
		},
		{
			name:    "invalid service URN - not a UUID",
			entity:  "urn:ivcap:service:invalid-uuid-string",
			wantErr: true,
			errMsg:  "does not end with a valid UUID",
		},
		{
			name:    "invalid artifact URN - incomplete UUID",
			entity:  "urn:ivcap:artifact:714e549b-ebab",
			wantErr: true,
			errMsg:  "does not end with a valid UUID",
		},
		{
			name:    "invalid job URN - wrong characters",
			entity:  "urn:ivcap:job:zzzzzzz-zzzz-zzzz-zzzz-zzzzzzzzzzzz",
			wantErr: true,
			errMsg:  "does not end with a valid UUID",
		},
		{
			name:    "invalid aspect URN - empty UUID",
			entity:  "urn:ivcap:aspect:",
			wantErr: true,
			errMsg:  "does not end with a valid UUID",
		},
		{
			name:    "non-IVCAP URN - should pass validation",
			entity:  "urn:other:service:anything",
			wantErr: false,
		},
		{
			name:    "IVCAP schema URN - should pass (not in validated list)",
			entity:  "urn:ivcap:schema:queue:message.1",
			wantErr: false,
		},
		{
			name:    "IVCAP collection URN - should pass (not in validated list)",
			entity:  "urn:ivcap:collection:123e4567-e89b-12d3-a456-426614174000",
			wantErr: false,
		},
		{
			name:    "IVCAP policy URN - should pass (not in validated list)",
			entity:  "urn:ivcap:policy:ivcap.open.service",
			wantErr: false,
		},
		{
			name:    "invalid format - too few parts",
			entity:  "urn:ivcap:service",
			wantErr: true,
			errMsg:  "invalid entity URN format",
		},
		{
			name:    "empty string - should pass",
			entity:  "",
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateEntityURN(tt.entity)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateEntityURN() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if err != nil && tt.errMsg != "" {
				if !strings.Contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateEntityURN() error = %v, should contain %v", err, tt.errMsg)
				}
			}
		})
	}
}
