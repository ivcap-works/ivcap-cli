// Copyright 2025 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
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
	neturl "net/url"
	"testing"
)

func TestParseURL(t *testing.T) {
	tests := []struct {
		name      string
		endpoint  string
		connCtxt  *ConnectionCtxt
		parsedURL *neturl.URL
		wantErr   bool
	}{
		{
			name:     "Valid URL defined in the endpoint",
			endpoint: "http://example.com/path",
			connCtxt: &ConnectionCtxt{URL: ""},
			parsedURL: &neturl.URL{
				Scheme: "http",
				Host:   "example.com",
				Path:   "/path",
			},
			wantErr: false,
		},
		{
			name:      "Invalid URL defined in the endpoint",
			endpoint:  "httpexample.com",
			connCtxt:  &ConnectionCtxt{URL: ""},
			parsedURL: nil,
			wantErr:   true,
		},
		{
			name:     "Undefined URL",
			endpoint: "invalid_url",
			connCtxt: &ConnectionCtxt{URL: ""},
			wantErr:  true,
		},
		{
			name:     "Valid URL with defined endpoint",
			endpoint: "/path",
			connCtxt: &ConnectionCtxt{URL: "http://example.com"},
			parsedURL: &neturl.URL{
				Scheme: "http",
				Host:   "example.com",
				Path:   "/path",
			},
			wantErr: false,
		},
		{
			name:     "Valid URL with undefined endpoint",
			endpoint: "",
			connCtxt: &ConnectionCtxt{URL: "http://example.com"},
			parsedURL: &neturl.URL{
				Scheme: "http",
				Host:   "example.com",
				Path:   "",
			},
			wantErr: false,
		},
		{
			name:      "Valid URL with invalid endpoint",
			endpoint:  "/%zz",
			connCtxt:  &ConnectionCtxt{URL: "http://example.com"},
			parsedURL: nil,
			wantErr:   true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			parsedURL, err := parseURL(tt.endpoint, tt.connCtxt)
			if tt.wantErr && err == nil {
				t.Errorf("parseURL() error = %v, wantErr %v", err, tt.wantErr)
			}
			if !tt.wantErr && err != nil {
				t.Errorf("parseURL() error = %v, wantErr %v", err, tt.wantErr)
			}
			if tt.parsedURL != nil && parsedURL == nil {
				t.Errorf("parseURL() parsedURL = %v, want %v", parsedURL, tt.parsedURL)
			}
			if tt.parsedURL != nil && parsedURL != nil && parsedURL.String() != tt.parsedURL.String() {
				t.Errorf("parseURL() parsedURL = %v, want %v", parsedURL, tt.parsedURL)
			}
		})
	}
}
