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

//go:build !windows

package cmd

import (
	"path/filepath"
	"syscall"
	"testing"
	"time"
)

// Sniffing a pipe would read bytes that the upload never sees, so anything that is not a
// regular file must be skipped without being opened, whatever its name says.
func TestManifestSkipsNonRegularFiles(t *testing.T) {
	dir := t.TempDir()
	for _, name := range []string{"piped-data", "piped.zip"} {
		fifo := filepath.Join(dir, name)
		if err := syscall.Mkfifo(fifo, 0o600); err != nil {
			t.Skipf("cannot create a FIFO here: %v", err)
		}
		type result struct {
			archive bool
			content []byte
			err     error
		}
		done := make(chan result, 1)
		go func() {
			var r result
			r.archive = isManifestArchive(fifo)
			r.content, r.err = prepareArtifactManifest(fifo)
			done <- r
		}()
		select {
		case r := <-done:
			if r.archive {
				t.Errorf("%s: isManifestArchive = true, want false", name)
			}
			if r.content != nil || r.err != nil {
				t.Errorf("%s: prepareArtifactManifest = (%q, %v), want (nil, nil)", name, r.content, r.err)
			}
		case <-time.After(5 * time.Second):
			t.Fatalf("%s: blocked while reading a FIFO; non-regular files must not be opened", name)
		}
	}
}
