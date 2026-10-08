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
	"strings"
	"testing"

	"github.com/ivcap-works/ivcap-cli/pkg/accountsapi"
)

func TestValidateCapabilities(t *testing.T) {
	valid := []accountsapi.Capability{
		{Name: "read"}, {Name: "write"}, {Name: "delete"},
	}
	t.Run("all valid", func(t *testing.T) {
		if err := validateCapabilities([]string{"read", "write"}, valid); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("empty is allowed", func(t *testing.T) {
		if err := validateCapabilities(nil, valid); err != nil {
			t.Errorf("unexpected error: %v", err)
		}
	})
	t.Run("unknown reported with valid list", func(t *testing.T) {
		err := validateCapabilities([]string{"read", "bogus"}, valid)
		if err == nil {
			t.Fatal("expected an error for an unknown capability")
		}
		msg := err.Error()
		if !strings.Contains(msg, "bogus") {
			t.Errorf("error should name the bad capability: %q", msg)
		}
		for _, v := range []string{"read", "write", "delete"} {
			if !strings.Contains(msg, v) {
				t.Errorf("error should list valid value %q: %q", v, msg)
			}
		}
	})
}
