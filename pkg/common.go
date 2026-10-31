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
	"fmt"
	"regexp"
	"strings"
	"time"
)

type ListRequest struct {
	Limit     int
	Page      *string
	Search    *string
	Filter    *string
	OrderBy   *string
	OrderDesc bool
	AtTime    *time.Time
}

// ValidateEntityURN validates that entities starting with specific IVCAP system entity prefixes
// end with a valid UUID. Any entity not matching these prefixes is allowed without validation.
// Returns an error if validation fails.
func ValidateEntityURN(entity string) error {
	// Check if entity starts with one of the validated IVCAP system entity types
	validatedPrefixes := []string{
		"urn:ivcap:service:",
		"urn:ivcap:artifact:",
		"urn:ivcap:job:",
		"urn:ivcap:aspect:",
		"urn:ivcap:queue:",
	}

	matchesPrefix := false
	for _, prefix := range validatedPrefixes {
		// Match either a full URN (entity starts with prefix) or a bare type name (entity + ":" equals prefix)
		if strings.HasPrefix(entity, prefix) || entity+":"+"" == prefix {
			matchesPrefix = true
			break
		}
	}

	// If it doesn't match any validated prefix, no validation needed
	if !matchesPrefix {
		return nil
	}

	// Extract the UUID part (everything after the last ':')
	parts := strings.Split(entity, ":")
	if len(parts) < 4 {
		return fmt.Errorf("invalid entity URN format: %s", entity)
	}

	uuid := parts[len(parts)-1]

	// UUID v4 regex pattern (standard format: 8-4-4-4-12 hex digits)
	// Also accepts UUID without hyphens
	uuidPattern := `^[0-9a-fA-F]{8}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{4}-?[0-9a-fA-F]{12}$`
	matched, err := regexp.MatchString(uuidPattern, uuid)
	if err != nil {
		return fmt.Errorf("error validating UUID: %w", err)
	}

	if !matched {
		return fmt.Errorf("invalid entity URN: %s does not end with a valid UUID (got: %s)", entity, uuid)
	}

	return nil
}
