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
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"reflect"
	"sync"
	"testing"

	adpt "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	asapi "github.com/ivcap-works/ivcap-core-api/http/aspect"
	log "go.uber.org/zap"
)

const testManifestEntity = "urn:ivcap:artifact:11111111-2222-3333-4444-555555555555"

// aspectListReply builds an aspect list reply with the generated API type itself, so the
// test does not depend on guessed JSON field names. Each item is {id, entity, schema}.
func aspectListReply(items [][3]string) ([]byte, error) {
	var list asapi.ListResponseBody
	slice := reflect.ValueOf(&list).Elem().FieldByName("Items")
	elem := slice.Type().Elem()
	isPtr := elem.Kind() == reflect.Ptr
	for _, it := range items {
		var item reflect.Value
		if isPtr {
			item = reflect.New(elem.Elem())
		} else {
			item = reflect.New(elem)
		}
		for i, field := range []string{"ID", "Entity", "Schema"} {
			value := it[i]
			item.Elem().FieldByName(field).Set(reflect.ValueOf(&value))
		}
		if isPtr {
			slice.Set(reflect.Append(slice, item))
		} else {
			slice.Set(reflect.Append(slice, item.Elem()))
		}
	}
	return json.Marshal(list)
}

// aspectTestServer is a minimal aspect store: GET lists everything stored, POST adds one.
type aspectTestServer struct {
	mu      sync.Mutex
	stored  [][3]string // id, entity, schema
	posts   int
	queries []url.Values
}

func (s *aspectTestServer) handler(t *testing.T) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		s.mu.Lock()
		defer s.mu.Unlock()
		w.Header().Set("Content-Type", "application/json")
		switch {
		case r.Method == http.MethodGet && r.URL.Path == "/1/aspects":
			s.queries = append(s.queries, r.URL.Query())
			body, err := aspectListReply(s.stored)
			if err != nil {
				t.Errorf("cannot build the aspect list reply: %v", err)
				http.Error(w, err.Error(), http.StatusInternalServerError)
				return
			}
			_, _ = w.Write(body)
		case r.Method == http.MethodPost && r.URL.Path == "/1/aspects":
			s.posts++
			id := fmt.Sprintf("urn:ivcap:aspect:%08d-1111-1111-1111-111111111111", s.posts)
			s.stored = append(s.stored, [3]string{id, r.URL.Query().Get("entity"), r.URL.Query().Get("schema")})
			_, _ = fmt.Fprintf(w, `{"id":%q}`, id)
		default:
			t.Errorf("unexpected request: %s %s", r.Method, r.URL.String())
			http.NotFound(w, r)
		}
	}
}

func (s *aspectTestServer) counts() (posts, lists int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.posts, len(s.queries)
}

// useAspectTestServer starts the server and points the package-level state that
// attachArtifactManifest reads at it for the duration of the test.
func useAspectTestServer(t *testing.T, s *aspectTestServer) adpt.Adapter {
	t.Helper()
	srv := httptest.NewServer(s.handler(t))
	t.Cleanup(srv.Close)
	oldLogger, oldSilent, oldFormat, oldPolicy, oldSchema := logger, silent, outputFormat, policy, artifactManifestSchema
	logger = log.NewNop()
	silent, outputFormat, policy, artifactManifestSchema = true, "", "", artifactArchiveManifestSchema
	t.Cleanup(func() {
		logger, silent, outputFormat, policy, artifactManifestSchema = oldLogger, oldSilent, oldFormat, oldPolicy, oldSchema
	})
	return adpt.RestAdapter(adpt.WithConnContext(&adpt.ConnectionCtxt{URL: srv.URL, TimeoutSec: 5}))
}

func TestAttachArtifactManifestIsIdempotent(t *testing.T) {
	s := &aspectTestServer{}
	adapter := useAspectTestServer(t, s)
	content := []byte(`{"$schema":"` + artifactArchiveManifestSchema + `"}`)

	for i := 1; i <= 3; i++ {
		if err := attachArtifactManifest(context.Background(), testManifestEntity, content, &adapter); err != nil {
			t.Fatalf("attach #%d: %v", i, err)
		}
	}
	posts, lists := s.counts()
	if posts != 1 {
		t.Errorf("the manifest aspect was added %d times, want exactly once", posts)
	}
	if lists != 3 {
		t.Errorf("the existing aspects were looked up %d times, want once per attach (3)", lists)
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	q := s.queries[0]
	if q.Get("entity") != testManifestEntity || q.Get("schema") != artifactArchiveManifestSchema {
		t.Errorf("lookup was not scoped to the artifact and schema: %v", q)
	}
}

func TestAttachArtifactManifestIgnoresOtherSchemas(t *testing.T) {
	// The API filters schemas by prefix, so a longer schema name can come back too.
	s := &aspectTestServer{stored: [][3]string{
		{"urn:ivcap:aspect:00000000-1111-1111-1111-111111111111", testManifestEntity, artifactArchiveManifestSchema + "0"},
	}}
	adapter := useAspectTestServer(t, s)

	if err := attachArtifactManifest(context.Background(), testManifestEntity, []byte(`{}`), &adapter); err != nil {
		t.Fatal(err)
	}
	if posts, _ := s.counts(); posts != 1 {
		t.Errorf("an aspect with a different schema must not count as the manifest; posts = %d, want 1", posts)
	}
}

func TestAttachArtifactManifestWithoutContentDoesNothing(t *testing.T) {
	s := &aspectTestServer{}
	adapter := useAspectTestServer(t, s)
	if err := attachArtifactManifest(context.Background(), testManifestEntity, nil, &adapter); err != nil {
		t.Fatal(err)
	}
	if posts, lists := s.counts(); posts != 0 || lists != 0 {
		t.Errorf("no content must mean no requests, got %d posts and %d lookups", posts, lists)
	}
}
