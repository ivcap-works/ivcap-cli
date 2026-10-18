// Copyright 2026 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
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

package cmd

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	adpt "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	ncbigenome "github.com/ivcap-works/ivcap-ncbi-genome-download"
	log "go.uber.org/zap"
)

func TestValidateChunkSize(t *testing.T) {
	const known, unknown = int64(1000), int64(-1)
	tests := []struct {
		name        string
		chunk, size int64
		wantErr     string // empty = valid
	}{
		{"default", DEF_CHUNK_SIZE, known, ""},
		{"exactly the minimum", MIN_CHUNK_SIZE, known, ""},
		{"minimum on unknown size", MIN_CHUNK_SIZE, unknown, ""},
		{"no chunking on known size", -1, known, ""},
		{"no chunking on empty file", -1, 0, ""},
		{"one below the minimum", MIN_CHUNK_SIZE - 1, known, "at least 65536 bytes"},
		{"zero", 0, known, "invalid --chunk-size 0"},
		{"tiny", 1, known, "invalid --chunk-size 1"},
		{"other negative", -2, known, "invalid --chunk-size -2"},
		{"zero on unknown size", 0, unknown, "invalid --chunk-size 0"},
		{"no chunking on unknown size", -1, unknown, "unknown size"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := validateChunkSize(tt.chunk, tt.size)
			switch {
			case tt.wantErr == "" && err != nil:
				t.Errorf("want valid, got %v", err)
			case tt.wantErr != "" && err == nil:
				t.Errorf("want an error containing %q, got none", tt.wantErr)
			case tt.wantErr != "" && !strings.Contains(err.Error(), tt.wantErr):
				t.Errorf("error %q lacks %q", err, tt.wantErr)
			}
		})
	}
}

// A bad chunk size must be caught before the (possibly large) NCBI download starts.
func TestRunGenomeImportRejectsBadChunkSizeBeforeDownloading(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Errorf("no request expected, got %s %s", r.Method, r.URL.String())
		http.NotFound(w, r)
	}))
	t.Cleanup(srv.Close)
	adapter := adpt.RestAdapter(adpt.WithConnContext(&adpt.ConnectionCtxt{URL: srv.URL, TimeoutSec: 5}))
	client := ncbigenome.New(ncbigenome.Options{BaseURL: srv.URL, MaxRetries: -1})
	oldLogger := logger
	logger = log.NewNop()
	t.Cleanup(func() { logger = oldLogger })

	for _, chunk := range []int64{0, 1, MIN_CHUNK_SIZE - 1, -2} {
		err := runGenomeImport(context.Background(), client, genomeImportOpts{
			Accession:      "GCF_000001735.4",
			ChunkSize:      chunk,
			ReportSchema:   ncbiAssemblyReportSchema,
			ManifestSchema: artifactArchiveManifestSchema,
			Silent:         true,
		}, &adapter, io.Discard, io.Discard)
		if err == nil || !strings.Contains(err.Error(), "--chunk-size") {
			t.Errorf("chunk size %d: want a --chunk-size error, got %v", chunk, err)
		}
	}
}
