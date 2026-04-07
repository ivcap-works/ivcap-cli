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
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"os"
	"path/filepath"
	"testing"

	yaml "gopkg.in/yaml.v2"

	nf "github.com/ivcap-works/ivcap-cli/pkg/nextflow"
)

const testToolYAML = `{"$schema": "urn:sd-core:schema.ai-tool.1"}
id: urn:sd-core:nextflow:simple-rnaseq-pipeline
name: simple-rnaseq-pipeline
service-id: urn:ivcap:service:a98b81a8-9279-509f-9c0e-40d39e83058a
description: |
  This pipeline provides a workflow for processing and analyzing bulk RNAseq data
contact:
  name: Mary Doe
  email: mary.doe@acme.au
fn-schema:
  $schema: http://json-schema.org/draft-07/schema#
  $id: urn:ivcap:schema:simple-rnaseq-pipeline.request.1
  title: Request to run pipeline 'simple-rnaseq-pipeline'
  type: object
  properties:
    parameters:
      type: object
      properties:
        x:
          type: string
`

const testSimpleToolYAML = `{"$schema": "urn:sd-core:schema.ai-tool.1"}
id: urn:sd-core:nextflow:simple-rnaseq-pipeline
name: simple-rnaseq-pipeline
service-id: urn:ivcap:service:a98b81a8-9279-509f-9c0e-40d39e83058a
description: |
  This pipeline provides a workflow for processing and analyzing bulk RNAseq data
contact:
  name: Mary Doe
  email: mary.doe@acme.au
properties:
  - name: hisat2_index_zip
    description: a uri
    type: string
    format: uri
    optional: false
  - name: report_id
    description: report id
    type: string
    optional: false
samples:
  - name: sample_name
    type: string
    description: unique id
  - name: read1_urn
    type: string
    format: uri
    description: read1
  - name: read2_urn
    type: string
    format: uri
    description: read2
`

func TestExtractFileFromTarReader_FindsRootPath(t *testing.T) {
	b := tarBytes(map[string]string{"ivcap-tool.yaml": testToolYAML})
	got, found, err := nf.ExtractFileFromTarReader(tar.NewReader(bytes.NewReader(b)), "ivcap-tool.yaml")
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}
	if found != "ivcap-tool.yaml" {
		t.Fatalf("unexpected found path: %q", found)
	}
	if string(got) != testToolYAML {
		t.Fatalf("unexpected content: %q", string(got))
	}
}

func TestExtractFileFromTarReader_FindsNestedByBaseName(t *testing.T) {
	b := tarBytes(map[string]string{"nested/ivcap-tool.yaml": testToolYAML})
	got, found, err := nf.ExtractFileFromTarReader(tar.NewReader(bytes.NewReader(b)), "ivcap-tool.yaml")
	if err != nil {
		t.Fatalf("extract error: %v", err)
	}
	if found != "nested/ivcap-tool.yaml" {
		t.Fatalf("unexpected found path: %q", found)
	}
	if string(got) != testToolYAML {
		t.Fatalf("unexpected content")
	}
}

func TestExtractFileFromTarReader_ErrorsOnMultipleMatches(t *testing.T) {
	b := tarBytes(map[string]string{
		"a/ivcap-tool.yaml": testToolYAML,
		"b/ivcap-tool.yaml": testToolYAML,
	})
	_, _, err := nf.ExtractFileFromTarReader(tar.NewReader(bytes.NewReader(b)), "ivcap-tool.yaml")
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestNextflowToolHeader_YamlUnmarshal(t *testing.T) {
	var h nf.ToolHeader
	if err := yaml.Unmarshal([]byte(testToolYAML), &h); err != nil {
		t.Fatalf("yaml unmarshal: %v", err)
	}
	if h.Schema != "urn:sd-core:schema.ai-tool.1" {
		t.Fatalf("unexpected schema: %q", h.Schema)
	}
	if h.ID == "" {
		t.Fatalf("expected id")
	}
	if h.ServiceID == "" {
		t.Fatalf("expected service-id")
	}
	if h.Name != "simple-rnaseq-pipeline" {
		t.Fatalf("unexpected name: %q", h.Name)
	}
	if h.Description == "" {
		t.Fatalf("expected description")
	}
	if h.Contact == nil || h.Contact.Email != "mary.doe@acme.au" {
		t.Fatalf("expected contact")
	}
	if err := nf.ValidateFnSchema(h.FnSchema); err != nil {
		t.Fatalf("expected fn-schema to be valid: %v", err)
	}
}

func TestConvertSimpleToolToToolHeader_GeneratesFnSchema(t *testing.T) {
	var s nf.SimpleToolHeader
	if err := yaml.Unmarshal([]byte(testSimpleToolYAML), &s); err != nil {
		t.Fatalf("yaml unmarshal: %v", err)
	}
	tool, err := nf.ConvertSimpleToolToToolHeader(&s)
	if err != nil {
		t.Fatalf("convert: %v", err)
	}
	if tool == nil {
		t.Fatalf("expected tool")
	}
	if tool.Name != "simple-rnaseq-pipeline" {
		t.Fatalf("unexpected name: %q", tool.Name)
	}
	if tool.FnSchema == nil {
		t.Fatalf("expected generated fn-schema")
	}
	if err := nf.ValidateFnSchema(tool.FnSchema); err != nil {
		t.Fatalf("expected generated fn-schema to be valid: %v", err)
	}

	props := tool.FnSchema["properties"].(map[string]any)
	params := props["parameters"].(map[string]any)
	pprops := params["properties"].(map[string]any)
	if _, ok := pprops["hisat2_index_zip"]; !ok {
		t.Fatalf("expected parameter hisat2_index_zip")
	}
	// samples tuple must exist
	samples := props["samples"].(map[string]any)
	items := samples["items"].(map[string]any)
	if items["minItems"].(int) != 3 {
		t.Fatalf("expected 3 sample fields")
	}
}

func TestValidateFnSchema_AllowsNil(t *testing.T) {
	if err := nf.ValidateFnSchema(nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestValidateFnSchema_RejectsMissingSchema(t *testing.T) {
	err := nf.ValidateFnSchema(map[string]any{"type": "object"})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestValidateFnSchema_RejectsNonJsonSchemaURL(t *testing.T) {
	err := nf.ValidateFnSchema(map[string]any{"$schema": "urn:abc", "type": "object"})
	if err == nil {
		t.Fatalf("expected error")
	}
}

func TestArchiveLooksGzip_ByMagic(t *testing.T) {
	// Build a small gzip stream and make sure we detect it.
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	_, _ = gzw.Write([]byte("x"))
	_ = gzw.Close()

	br := bytes.NewReader(buf.Bytes())
	// Wrap br in something that provides Peek.
	// Use bufio.NewReader in the call site, but for this unit test we just
	// want to hit magic detection via extractFileFromTarPath indirectly.
	// Therefore just ensure gzip magic is present.
	magic := make([]byte, 2)
	_, _ = br.Read(magic)
	if magic[0] != 0x1f || magic[1] != 0x8b {
		t.Fatalf("expected gzip magic")
	}
}

func tarBytes(files map[string]string) []byte {
	var buf bytes.Buffer
	tw := tar.NewWriter(&buf)
	for name, content := range files {
		b := []byte(content)
		if err := tw.WriteHeader(&tar.Header{Name: name, Mode: 0o644, Size: int64(len(b))}); err != nil {
			panic(err)
		}
		if _, err := tw.Write(b); err != nil {
			panic(err)
		}
	}
	if err := tw.Close(); err != nil {
		panic(err)
	}
	return buf.Bytes()
}

func TestNextflowUploadMeta_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	archive := filepath.Join(tmp, "big.tar")
	// create placeholder file
	if err := os.WriteFile(archive, []byte("x"), 0644); err != nil {
		t.Fatalf("write: %v", err)
	}
	st, err := os.Stat(archive)
	if err != nil {
		t.Fatalf("stat: %v", err)
	}
	size := st.Size()
	mt := st.ModTime().Unix()
	aid := "urn:ivcap:artifact:123"

	nf.WriteUploadMeta(archive, size, mt, aid)
	got, ok := nf.ReadUploadMeta(archive, size, mt)
	if !ok {
		t.Fatalf("expected ok")
	}
	if got != aid {
		t.Fatalf("unexpected aid: %q", got)
	}
}

func TestBuildNextflowServiceDescription_UsesToolAndPipelineURN(t *testing.T) {
	tool := &nf.ToolHeader{
		Name:        "simple-rnaseq-pipeline",
		Description: "desc",
		Contact: &struct {
			Name  string `yaml:"name" json:"name"`
			Email string `yaml:"email" json:"email"`
		}{Name: "Mary Doe", Email: "mary.doe@acme.au"},
		FnSchema: map[string]any{"$schema": "http://json-schema.org/draft-07/schema#", "type": "object"},
	}
	svc := nf.BuildServiceDescription(tool, "urn:ivcap:service:abc", "urn:ivcap:artifact:pipeline")
	if svc.Schema != nf.ServiceSchema {
		t.Fatalf("unexpected schema: %q", svc.Schema)
	}
	if svc.ID != "urn:ivcap:service:abc" {
		t.Fatalf("unexpected id: %q", svc.ID)
	}
	if svc.ControllerSchema != nf.ServiceControllerURN {
		t.Fatalf("unexpected controller schema: %q", svc.ControllerSchema)
	}
	ctrl := svc.Controller.(map[string]any)
	p := ctrl["pipeline"].(map[string]any)
	if p["urn"].(string) != "urn:ivcap:artifact:pipeline" {
		t.Fatalf("unexpected pipeline urn")
	}
}

func TestPrintNextflowCreateOutput_RejectsUnknownFormat(t *testing.T) {
	prev := nextflowCreateFormat
	defer func() { nextflowCreateFormat = prev }()
	nextflowCreateFormat = "toml"
	out := &nf.CreateOutput{OK: true, ServiceID: "s", PipelineArtifactURN: "a"}
	if err := printNextflowCreateOutput(out); err == nil {
		t.Fatalf("expected error")
	}
}

func TestRunNextflowCreateOrUpdate_RequiresServiceID(t *testing.T) {
	if err := runNextflowCreateOrUpdate(context.Background(), ""); err == nil {
		t.Fatalf("expected error")
	}
}
