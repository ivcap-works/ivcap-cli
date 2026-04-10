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

// Package nextflow contains reusable logic for working with Nextflow-based services:
//
//   - parsing ivcap.yaml / ivcap-tool.yaml from a pipeline archive
//   - generating the service description payload
//   - upload resume metadata helpers
//   - tar archive inspection helpers
//
// It intentionally contains no Cobra / CLI printing. (Note: it may still write
// small progress/status messages via the provided zap.Logger, consistent with
// other pkg/* helpers.)
package nextflow

import (
	"archive/tar"
	"bufio"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"

	yaml "gopkg.in/yaml.v2"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	log "go.uber.org/zap"
)

const ToolFileName = "ivcap-tool.yaml"

// Optional, simplified manifest. If present in the archive, this file is preferred over
// ivcap-tool.yaml and will be converted to the richer ivcap-tool structure internally.
const SimpleToolFileName = "ivcap.yaml"

const UploadMetaPrefix = ".ivcap-nextflow-"

const (
	ServiceSchema         = "urn:ivcap:schema.service.2"
	ServiceControllerURN  = "urn:ivcap:schema.service.nextflow.1"
	ServiceDefaultPolicy  = "urn:ivcap:policy:ivcap.open.service"
	ServiceMainScriptName = "main.nf"
)

// CreateOutput is a convenience struct for callers who want to return an object similar
// to `ivcap nextflow create`.
type CreateOutput struct {
	OK                    bool                `yaml:"ok" json:"ok"`
	ServiceID             string              `yaml:"service-id" json:"service-id"`
	PipelineArtifactURN   string              `yaml:"pipeline" json:"pipeline"`
	ServiceAspectRecordID string              `yaml:"service-aspect" json:"service-aspect"`
	ServiceDescription    *ServiceDescription `yaml:"service" json:"service"`
}

type ServiceDescription struct {
	Schema           string `yaml:"$schema" json:"$schema"`
	ID               string `yaml:"$id" json:"$id"`
	Name             string `yaml:"name" json:"name"`
	Description      string `yaml:"description" json:"description"`
	RequestSchema    any    `yaml:"request-schema" json:"request-schema"`
	Contact          any    `yaml:"contact" json:"contact"`
	Policy           string `yaml:"policy" json:"policy"`
	ControllerSchema string `yaml:"controller-schema" json:"controller-schema"`
	Controller       any    `yaml:"controller" json:"controller"`
}

type ToolHeader struct {
	Schema      string `yaml:"$schema" json:"$schema"`
	ID          string `yaml:"id" json:"id"`
	Name        string `yaml:"name" json:"name"`
	ServiceID   string `yaml:"service-id" json:"service-id"`
	Description string `yaml:"description" json:"description"`
	Contact     *struct {
		Name  string `yaml:"name" json:"name"`
		Email string `yaml:"email" json:"email"`
	} `yaml:"contact" json:"contact"`
	FnSchema map[string]any `yaml:"fn-schema" json:"fn-schema"`
}

// --- ivcap.yaml (simplified) parsing ------------------------------------------------

type SimpleToolHeader struct {
	Schema      string `yaml:"$schema" json:"$schema"`
	ID          string `yaml:"id" json:"id"`
	Name        string `yaml:"name" json:"name"`
	ServiceID   string `yaml:"service-id" json:"service-id"`
	Description string `yaml:"description" json:"description"`
	Contact     *struct {
		Name  string `yaml:"name" json:"name"`
		Email string `yaml:"email" json:"email"`
	} `yaml:"contact" json:"contact"`
	Properties []SimpleProperty `yaml:"properties" json:"properties"`
	Samples    []SimpleProperty `yaml:"samples" json:"samples"`
	Example    map[string]any   `yaml:"example" json:"example"`
}

type SimpleProperty struct {
	Name        string `yaml:"name" json:"name"`
	Description string `yaml:"description" json:"description"`
	Type        string `yaml:"type" json:"type"`
	Format      string `yaml:"format" json:"format"`
	Optional    bool   `yaml:"optional" json:"optional"`
}

func LoadToolHeaderFromArchivePath(archivePath string) (*ToolHeader, string, error) {
	// Prefer simplified ivcap.yaml if present.
	if b, foundPath, err := ExtractFileFromTarPath(archivePath, SimpleToolFileName); err != nil {
		return nil, "", err
	} else if b != nil {
		var simple SimpleToolHeader
		dec := yaml.NewDecoder(bytes.NewReader(b))
		if err := dec.Decode(&simple); err != nil {
			return nil, "", fmt.Errorf("while parsing %q extracted from %q: %w", foundPath, archivePath, err)
		}
		tool, err := ConvertSimpleToolToToolHeader(&simple)
		if err != nil {
			return nil, "", fmt.Errorf("invalid %q extracted from %q: %w", foundPath, archivePath, err)
		}
		return tool, foundPath, nil
	}

	// Fallback to ivcap-tool.yaml.
	b, foundPath, err := ExtractFileFromTarPath(archivePath, ToolFileName)
	if err != nil {
		return nil, "", err
	}
	if b == nil {
		return nil, "", nil
	}
	var tool ToolHeader
	dec := yaml.NewDecoder(bytes.NewReader(b))
	if err := dec.Decode(&tool); err != nil {
		return nil, "", fmt.Errorf("while parsing %q extracted from %q: %w", foundPath, archivePath, err)
	}
	if err := ValidateFnSchema(tool.FnSchema); err != nil {
		return nil, "", fmt.Errorf("invalid fn-schema in %q extracted from %q: %w", foundPath, archivePath, err)
	}
	return &tool, foundPath, nil
}

// LoadToolHeaderFromArchiveBytes reads ivcap.yaml or ivcap-tool.yaml from a tar/tar.gz
// archive provided as bytes.
//
// Returns:
//   - tool header (or nil if not found)
//   - found path in archive
func LoadToolHeaderFromArchiveBytes(archive []byte) (*ToolHeader, string, error) {
	// Prefer simplified ivcap.yaml.
	if b, foundPath, err := ExtractFileFromTarBytes(archive, SimpleToolFileName); err != nil {
		return nil, "", err
	} else if b != nil {
		var simple SimpleToolHeader
		dec := yaml.NewDecoder(bytes.NewReader(b))
		if err := dec.Decode(&simple); err != nil {
			return nil, "", fmt.Errorf("while parsing %s extracted from %s: %w", foundPath, "archive-bytes", err)
		}
		tool, err := ConvertSimpleToolToToolHeader(&simple)
		if err != nil {
			return nil, "", fmt.Errorf("invalid %s extracted from %s: %w", foundPath, "archive-bytes", err)
		}
		return tool, foundPath, nil
	}

	// Fallback to ivcap-tool.yaml.
	if toolYAML, foundPath, err := ExtractFileFromTarBytes(archive, ToolFileName); err != nil {
		return nil, "", err
	} else if toolYAML != nil {
		var toolHdr ToolHeader
		dec := yaml.NewDecoder(bytes.NewReader(toolYAML))
		if err := dec.Decode(&toolHdr); err != nil {
			return nil, "", fmt.Errorf("while parsing %s extracted from %s: %w", foundPath, "archive-bytes", err)
		}
		if err := ValidateFnSchema(toolHdr.FnSchema); err != nil {
			return nil, "", fmt.Errorf("invalid fn-schema in %s: %w", foundPath, err)
		}
		return &toolHdr, foundPath, nil
	}

	return nil, "", nil
}

func ConvertSimpleToolToToolHeader(simple *SimpleToolHeader) (*ToolHeader, error) {
	if simple == nil {
		return nil, fmt.Errorf("missing tool")
	}
	if strings.TrimSpace(simple.Name) == "" {
		return nil, fmt.Errorf("missing name")
	}

	fn, err := buildFnSchemaFromSimplifiedTool(simple)
	if err != nil {
		return nil, err
	}
	if err := ValidateFnSchema(fn); err != nil {
		// ensure we generated something that passes existing validation.
		return nil, fmt.Errorf("generated fn-schema invalid: %w", err)
	}

	return &ToolHeader{
		Schema:      simple.Schema,
		ID:          simple.ID,
		Name:        simple.Name,
		ServiceID:   simple.ServiceID,
		Description: simple.Description,
		Contact:     simple.Contact,
		FnSchema:    fn,
	}, nil
}

func buildFnSchemaFromSimplifiedTool(simple *SimpleToolHeader) (map[string]any, error) {
	name := strings.TrimSpace(simple.Name)
	if name == "" {
		return nil, fmt.Errorf("missing name")
	}

	paramProps := map[string]any{}
	requiredParams := []any{}
	for _, p := range simple.Properties {
		pn := strings.TrimSpace(p.Name)
		if pn == "" {
			return nil, fmt.Errorf("properties entry missing name")
		}
		prop := map[string]any{}
		if strings.TrimSpace(p.Type) != "" {
			prop["type"] = strings.TrimSpace(p.Type)
		} else {
			prop["type"] = "string"
		}
		if strings.TrimSpace(p.Description) != "" {
			prop["description"] = p.Description
		}
		if strings.TrimSpace(p.Format) != "" {
			prop["format"] = strings.TrimSpace(p.Format)
		}
		paramProps[pn] = prop
		if !p.Optional {
			requiredParams = append(requiredParams, pn)
		}
	}

	paramsObj := map[string]any{
		"type":       "object",
		"properties": paramProps,
	}
	if len(requiredParams) > 0 {
		paramsObj["required"] = requiredParams
	}

	sampleItemSchemas := []any{}
	for _, s := range simple.Samples {
		sn := strings.TrimSpace(s.Name)
		if sn == "" {
			return nil, fmt.Errorf("samples entry missing name")
		}
		it := map[string]any{}
		if strings.TrimSpace(s.Type) != "" {
			it["type"] = strings.TrimSpace(s.Type)
		} else {
			it["type"] = "string"
		}
		if strings.TrimSpace(s.Description) != "" {
			it["description"] = s.Description
		}
		if strings.TrimSpace(s.Format) != "" {
			it["format"] = strings.TrimSpace(s.Format)
		}
		sampleItemSchemas = append(sampleItemSchemas, it)
	}

	samplesSchema := map[string]any{
		"type": "array",
	}
	if len(sampleItemSchemas) > 0 {
		samplesSchema["items"] = map[string]any{
			"type":     "array",
			"minItems": len(sampleItemSchemas),
			"maxItems": len(sampleItemSchemas),
			"items":    sampleItemSchemas,
		}
	}

	requiredTop := []any{"parameters"}
	if len(sampleItemSchemas) > 0 {
		requiredTop = append(requiredTop, "samples")
	}

	id := fmt.Sprintf("urn:ivcap:schema:%s.request.1", name)

	fn := map[string]any{
		"$schema": "http://json-schema.org/draft-07/schema#",
		"$id":     id,
		"title":   fmt.Sprintf("Request to run pipeline '%s'", name),
		"type":    "object",
		"properties": map[string]any{
			"parameters": paramsObj,
			"samples":    samplesSchema,
		},
		"required": requiredTop,
	}

	return fn, nil
}

func BuildServiceDescription(tool *ToolHeader, serviceID string, pipelineArtifactURN string) *ServiceDescription {
	name := tool.Name
	desc := tool.Description
	contact := any(nil)
	if tool.Contact != nil {
		contact = map[string]any{"name": tool.Contact.Name, "email": tool.Contact.Email}
	}

	// Per instruction: parameters should contain fn-schema.
	params := any([]any{})
	if tool.FnSchema != nil {
		params = tool.FnSchema
	}

	controller := map[string]any{
		"$schema": ServiceControllerURN,
		"pipeline": map[string]any{
			"urn":         pipelineArtifactURN,
			"main_script": ServiceMainScriptName,
		},
		"resources": map[string]any{
			"limits":   map[string]any{"cpu": "500m", "memory": "1Gi"},
			"requests": map[string]any{"cpu": "500m", "memory": "1Gi"},
		},
	}

	return &ServiceDescription{
		Schema:           ServiceSchema,
		ID:               serviceID,
		Name:             name,
		Description:      desc,
		RequestSchema:    params,
		Contact:          contact,
		Policy:           ServiceDefaultPolicy,
		ControllerSchema: ServiceControllerURN,
		Controller:       controller,
	}
}

// UpsertServiceDescriptionAspect publishes a service description as a Data Fabric aspect.
//
// Entity: service ID
// Schema: svc.Schema
func UpsertServiceDescriptionAspect(ctxt context.Context, entityServiceID string, svc *ServiceDescription, adapter *a.Adapter, logger *log.Logger) (aspectID string, err error) {
	if entityServiceID == "" {
		return "", fmt.Errorf("missing entity service id")
	}
	if svc == nil {
		return "", fmt.Errorf("missing service description")
	}
	if svc.Schema == "" {
		return "", fmt.Errorf("service description missing $schema")
	}

	b, err := json.Marshal(svc)
	if err != nil {
		return "", err
	}

	// Try update (PUT) first.
	res, err := sdk.AddUpdateAspect(ctxt, false, entityServiceID, svc.Schema, "", b, adapter, logger)
	if err != nil {
		var rnfe *a.ResourceNotFoundError
		if errors.As(err, &rnfe) {
			// Create (POST).
			res, err = sdk.AddUpdateAspect(ctxt, true, entityServiceID, svc.Schema, "", b, adapter, logger)
			if err != nil {
				return "", err
			}
		} else {
			return "", err
		}
	}
	if m, err := res.AsObject(); err == nil {
		if id, ok := m["id"]; ok {
			aspectID = fmt.Sprintf("%v", id)
		}
	}
	return aspectID, nil
}

// --- Upload resume metadata helpers --------------------------------------------------

type UploadMeta struct {
	Size       int64
	MTimeUnix  int64
	ArtifactID string
}

func UploadMetaPath(archivePath string) string {
	base := filepath.Base(archivePath)
	dir := filepath.Dir(archivePath)
	return filepath.Join(dir, UploadMetaPrefix+base+".txt")
}

func ReadUploadMeta(archivePath string, size int64, mtimeUnix int64) (artifactID string, ok bool) {
	if archivePath == "-" {
		return "", false
	}
	mp := UploadMetaPath(archivePath)
	b, err := os.ReadFile(filepath.Clean(mp))
	if err != nil {
		return "", false
	}
	var m UploadMeta
	if err := json.Unmarshal(b, &m); err != nil {
		return "", false
	}
	if m.ArtifactID == "" {
		return "", false
	}
	if m.Size != size || m.MTimeUnix != mtimeUnix {
		return "", false
	}
	return m.ArtifactID, true
}

func WriteUploadMeta(archivePath string, size int64, mtimeUnix int64, artifactID string) {
	if archivePath == "-" {
		return
	}
	mp := UploadMetaPath(archivePath)
	m := UploadMeta{Size: size, MTimeUnix: mtimeUnix, ArtifactID: artifactID}
	b, err := json.Marshal(m)
	if err != nil {
		return
	}
	// best-effort; ignore errors.
	_ = os.WriteFile(filepath.Clean(mp), b, 0644) // #nosec G306 -- stores only artifact id + basic file info
}

func GuessArchiveContentType(p string) string {
	pl := strings.ToLower(p)
	if strings.HasSuffix(pl, ".tgz") || strings.HasSuffix(pl, ".tar.gz") || strings.HasSuffix(pl, ".gz") {
		return "application/gzip"
	}
	if strings.HasSuffix(pl, ".tar") {
		return "application/x-tar"
	}
	f, err := os.Open(filepath.Clean(p))
	if err != nil {
		return "application/octet-stream"
	}
	defer f.Close()
	buf := make([]byte, 512)
	_, _ = f.Read(buf)
	ct := http.DetectContentType(buf)
	if ct == "application/octet-stream" {
		return "application/x-tar"
	}
	return ct
}

// --- JSON schema validation ----------------------------------------------------------

func ValidateFnSchema(m map[string]any) error {
	if m == nil {
		return nil
	}
	if _, err := json.Marshal(m); err != nil {
		return fmt.Errorf("not JSON representable: %w", err)
	}

	vs, ok := m["$schema"]
	if !ok {
		return fmt.Errorf("missing $schema")
	}
	schema, ok := vs.(string)
	if !ok || strings.TrimSpace(schema) == "" {
		return fmt.Errorf("$schema must be a non-empty string")
	}
	if !strings.Contains(schema, "json-schema.org") {
		return fmt.Errorf("$schema does not look like a JSON Schema meta-schema URL: %q", schema)
	}

	keywords := []string{"type", "properties", "$ref", "allOf", "anyOf", "oneOf"}
	found := false
	for _, k := range keywords {
		if _, ok := m[k]; ok {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("does not contain any of the expected JSON Schema keywords: %s", strings.Join(keywords, ", "))
	}

	if tv, ok := m["type"]; ok {
		if ts, ok := tv.(string); ok {
			switch ts {
			case "null", "boolean", "object", "array", "number", "integer", "string":
			default:
				return fmt.Errorf("invalid type %q", ts)
			}
		}
	}

	if pv, ok := m["properties"]; ok {
		if _, ok := pv.(map[string]any); !ok {
			if _, ok := pv.(map[any]any); ok {
				return fmt.Errorf("properties must be a map with string keys")
			}
			return fmt.Errorf("properties must be an object")
		}
	}
	return nil
}

// --- TAR helpers --------------------------------------------------------------------

func ExtractFileFromTarPath(archivePath string, fileName string) (content []byte, foundPath string, err error) {
	f, err := os.Open(filepath.Clean(archivePath))
	if err != nil {
		return nil, "", err
	}
	defer f.Close()

	br := bufio.NewReader(f)
	isGzip := archiveLooksGzip(archivePath, br)

	var r io.Reader = br
	var gz *gzip.Reader
	if isGzip {
		gz, err = gzip.NewReader(br)
		if err != nil {
			return nil, "", err
		}
		defer gz.Close()
		r = gz
	}
	return ExtractFileFromTarReader(tar.NewReader(r), fileName)
}

// ExtractFileFromTarBytes extracts a file from a tar or tar.gz archive represented as bytes.
func ExtractFileFromTarBytes(archive []byte, fileName string) ([]byte, string, error) {
	fileName = path.Clean(strings.TrimPrefix(fileName, "/"))
	if fileName == "." || fileName == "" {
		return nil, "", fmt.Errorf("invalid target name: %q", fileName)
	}

	// Try gzip first (magic bytes).
	if len(archive) > 2 && archive[0] == 0x1f && archive[1] == 0x8b {
		gzr, err := gzip.NewReader(bytes.NewReader(archive))
		if err != nil {
			return nil, "", err
		}
		defer gzr.Close()
		return ExtractFileFromTarReader(tar.NewReader(gzr), fileName)
	}
	return ExtractFileFromTarReader(tar.NewReader(bytes.NewReader(archive)), fileName)
}

func archiveLooksGzip(p string, br *bufio.Reader) bool {
	if b, err := br.Peek(2); err == nil {
		if len(b) == 2 && b[0] == 0x1f && b[1] == 0x8b {
			return true
		}
	}
	ext := strings.ToLower(filepath.Ext(p))
	if ext == ".gz" || ext == ".tgz" {
		return true
	}
	return false
}

func ExtractFileFromTarReader(tr *tar.Reader, targetName string) (content []byte, foundPath string, err error) {
	targetName = path.Clean(strings.TrimPrefix(targetName, "/"))
	if targetName == "." || targetName == "" {
		return nil, "", fmt.Errorf("invalid target name: %q", targetName)
	}

	var byBase [][]byte
	var byBasePaths []string
	for {
		h, err := tr.Next()
		if err != nil {
			if err == io.EOF {
				break
			}
			return nil, "", err
		}
		name := path.Clean(strings.TrimPrefix(h.Name, "/"))
		if name == "." {
			continue
		}
		if name == targetName {
			b, err := io.ReadAll(io.LimitReader(tr, 2*1024*1024))
			if err != nil {
				return nil, "", err
			}
			extra := make([]byte, 1)
			if n, _ := tr.Read(extra); n > 0 {
				return nil, "", fmt.Errorf("%q in archive is too large", targetName)
			}
			return b, name, nil
		}
		if path.Base(name) == targetName {
			b, err := io.ReadAll(io.LimitReader(tr, 2*1024*1024))
			if err != nil {
				return nil, "", err
			}
			byBase = append(byBase, b)
			byBasePaths = append(byBasePaths, name)
		}
	}

	if len(byBase) == 1 {
		return bytes.Clone(byBase[0]), byBasePaths[0], nil
	}
	if len(byBase) > 1 {
		return nil, "", fmt.Errorf("multiple %q files found in archive: %s", targetName, strings.Join(byBasePaths, ", "))
	}
	return nil, "", nil
}

// --- Uploading a local archive to an artifact with TUS resume -----------------------

// UploadArchiveAsArtifact uploads a local archive file as an IVCAP artifact and supports
// resuming interrupted uploads via local UploadMeta.
//
// Notes:
//   - archivePath must be a local path ("-" is not supported)
//   - chunkSize should typically be cmd.DEF_CHUNK_SIZE
func UploadArchiveAsArtifact(
	ctxt context.Context,
	toolName string,
	archivePath string,
	chunkSize int64,
	adapter *a.Adapter,
	silent bool,
	logger *log.Logger,
) (artifactID string, err error) {
	if toolName == "" {
		toolName = filepath.Base(archivePath)
	}
	ct := GuessArchiveContentType(archivePath)
	st, err := os.Stat(filepath.Clean(archivePath))
	if err != nil {
		return "", err
	}
	size := st.Size()
	mtimeUnix := st.ModTime().Unix()

	if mid, ok := ReadUploadMeta(archivePath, size, mtimeUnix); ok {
		artifactID = mid
		readResp, err := sdk.ReadArtifact(ctxt, &sdk.ReadArtifactRequest{Id: artifactID}, adapter, logger)
		if err != nil {
			return artifactID, fmt.Errorf("while reading artifact %s for resume: %w", artifactID, err)
		}
		if readResp == nil || readResp.DataHref == nil {
			return artifactID, fmt.Errorf("artifact %s has no data upload URL", artifactID)
		}
		p, err := (*adapter).GetPath(*readResp.DataHref)
		if err != nil {
			return artifactID, err
		}
		if err := tusUploadWithResume(ctxt, archivePath, size, p, chunkSize, adapter, silent, logger); err != nil {
			return artifactID, err
		}
		return artifactID, nil
	}

	req := &sdk.CreateArtifactRequest{Name: toolName, Size: size}
	resp, err := sdk.CreateArtifact(ctxt, req, ct, size, nil, adapter, logger)
	if err != nil {
		return "", err
	}
	if resp == nil || resp.ID == nil || resp.DataHref == nil {
		return "", fmt.Errorf("unexpected create artifact response")
	}
	artifactID = *resp.ID
	WriteUploadMeta(archivePath, size, mtimeUnix, artifactID)

	p, err := (*adapter).GetPath(*resp.DataHref)
	if err != nil {
		return "", err
	}

	if err := tusUploadWithResume(ctxt, archivePath, size, p, chunkSize, adapter, silent, logger); err != nil {
		return artifactID, err
	}
	return artifactID, nil
}

func tusUploadWithResume(
	ctxt context.Context,
	archivePath string,
	size int64,
	uploadPath string,
	chunkSize int64,
	adapter *a.Adapter,
	silent bool,
	logger *log.Logger,
) error {
	const maxAttempts = 20
	var offset int64 = 0

	if off, err := getTusOffset(ctxt, uploadPath, adapter, logger); err == nil {
		offset = off
		if size > 0 && offset >= size {
			return nil
		}
	}

	sameOffsetRetries := 0
	for attempt := 1; attempt <= maxAttempts; attempt++ {
		f, err := os.Open(filepath.Clean(archivePath))
		if err != nil {
			return err
		}
		err = sdk.UploadArtifact(ctxt, f, size, offset, chunkSize, uploadPath, adapter, silent, logger)
		_ = f.Close()
		if err == nil {
			return nil
		}

		noff, herr := getTusOffset(ctxt, uploadPath, adapter, logger)
		if herr != nil {
			return fmt.Errorf("upload failed (%v) and cannot query offset (%v)", err, herr)
		}
		if noff == offset {
			sameOffsetRetries++
			if sameOffsetRetries >= 5 {
				return fmt.Errorf("upload failed and did not progress (offset=%d): %w", offset, err)
			}
		} else {
			sameOffsetRetries = 0
		}
		offset = noff
	}
	return fmt.Errorf("upload failed after %d attempts", maxAttempts)
}

func getTusOffset(ctxt context.Context, uploadPath string, adapter *a.Adapter, logger *log.Logger) (int64, error) {
	headers := map[string]string{"Tus-Resumable": "1.0.0"}
	pyld, herr := (*adapter).Head(ctxt, uploadPath, &headers, logger)
	if herr != nil {
		return 0, herr
	}
	offH := pyld.Header("Upload-Offset")
	noff, perr := strconv.ParseInt(offH, 10, 64)
	if perr != nil {
		return 0, fmt.Errorf("cannot parse Upload-Offset %q: %w", offH, perr)
	}
	return noff, nil
}
