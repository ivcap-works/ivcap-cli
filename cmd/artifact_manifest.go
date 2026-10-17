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
	"archive/tar"
	"archive/zip"
	"bufio"
	"compress/gzip"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime"
	"net/url"
	"os"
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	adpt "github.com/ivcap-works/ivcap-cli/pkg/adapter"
)

const (
	artifactArchiveManifestSchema = "urn:ivcap:schema:artifact.archive-manifest.1"
	roCrateContext                = "https://w3id.org/ro/crate/1.2/context"
	roCrateProfile                = "https://w3id.org/ro/crate/1.2"
)

type archiveManifestMetadata struct {
	Identifier  string
	Name        string
	Description string
	Published   string
}

type archiveManifestEntry struct {
	Name string
	Size uint64
}

type archiveManifestAspect struct {
	Schema  string          `json:"$schema"`
	Profile string          `json:"profile"`
	ROCrate roCrateDocument `json:"ro_crate"`
}

type roCrateDocument struct {
	Context string          `json:"@context"`
	Graph   []roCrateEntity `json:"@graph"`
}

type roCrateEntity struct {
	ID             string       `json:"@id"`
	Type           string       `json:"@type"`
	About          *roCrateRef  `json:"about,omitempty"`
	ConformsTo     *roCrateRef  `json:"conformsTo,omitempty"`
	Name           string       `json:"name,omitempty"`
	Description    string       `json:"description,omitempty"`
	Identifier     string       `json:"identifier,omitempty"`
	HasPart        []roCrateRef `json:"hasPart,omitempty"`
	DatePublished  string       `json:"datePublished,omitempty"`
	ContentSize    string       `json:"contentSize,omitempty"`
	EncodingFormat string       `json:"encodingFormat,omitempty"`
}

type roCrateRef struct {
	ID string `json:"@id"`
}

func isManifestArchive(fileName string) (bool, error) {
	cleanName := filepath.Clean(fileName)
	lowerName := strings.ToLower(cleanName)
	claimed := strings.HasSuffix(lowerName, ".zip") || strings.HasSuffix(lowerName, ".tar") ||
		strings.HasSuffix(lowerName, ".tar.gz") || strings.HasSuffix(lowerName, ".tgz")
	if claimed {
		// Let the full archive reader validate claimed archives. This also accepts a
		// valid empty TAR, whose first tar.Reader.Next call returns io.EOF.
		return true, nil
	}

	if zr, err := zip.OpenReader(cleanName); err == nil {
		_ = zr.Close()
		return true, nil
	}
	f, err := os.Open(cleanName)
	if err != nil {
		return false, fmt.Errorf("cannot inspect archive %q: %w", fileName, err)
	}
	defer f.Close()
	br := bufio.NewReader(f)
	var reader io.Reader = br
	magic, _ := br.Peek(2)
	if len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(br)
		if err != nil {
			return false, nil
		}
		defer gz.Close()
		reader = gz
	}
	_, tarErr := tar.NewReader(reader).Next()
	if tarErr == nil {
		return true, nil
	}
	return false, nil
}

func archiveManifestForFile(fileName, schema string, metadata archiveManifestMetadata) ([]byte, error) {
	if fileName == "-" {
		return nil, errors.New("manifest generation requires a local archive file; '-f -' is not supported")
	}
	if schema == "" {
		return nil, errors.New("manifest schema must not be empty")
	}
	info, err := os.Stat(filepath.Clean(fileName))
	if err != nil {
		return nil, fmt.Errorf("cannot inspect archive %q: %w", fileName, err)
	}
	if !info.Mode().IsRegular() {
		return nil, fmt.Errorf("archive %q is not a regular file", fileName)
	}
	entries, err := readArchiveManifest(fileName)
	if err != nil {
		return nil, err
	}
	if metadata.Name == "" {
		metadata.Name = filepath.Base(fileName)
	}
	if metadata.Identifier == "" {
		metadata.Identifier = metadata.Name
	}
	if metadata.Description == "" {
		metadata.Description = "Files contained in the uploaded archive."
	}
	if metadata.Published == "" {
		metadata.Published = info.ModTime().UTC().Format(time.DateOnly)
	}
	return json.Marshal(archiveManifestAspect{
		Schema: schema, Profile: roCrateProfile, ROCrate: buildROCrate(metadata, entries),
	})
}

func readArchiveManifest(fileName string) ([]archiveManifestEntry, error) {
	if zr, err := zip.OpenReader(filepath.Clean(fileName)); err == nil {
		defer zr.Close()
		entries := make([]archiveManifestEntry, 0, len(zr.File))
		for _, f := range zr.File {
			if f.FileInfo().IsDir() {
				continue
			}
			name, err := safeArchivePath(f.Name)
			if err != nil {
				return nil, fmt.Errorf("invalid ZIP entry: %w", err)
			}
			entries = append(entries, archiveManifestEntry{Name: name, Size: f.UncompressedSize64})
		}
		return entries, nil
	}

	f, err := os.Open(filepath.Clean(fileName))
	if err != nil {
		return nil, fmt.Errorf("cannot open archive %q: %w", fileName, err)
	}
	defer f.Close()
	br := bufio.NewReader(f)
	var reader io.Reader = br
	magic, _ := br.Peek(2)
	if len(magic) == 2 && magic[0] == 0x1f && magic[1] == 0x8b {
		gz, err := gzip.NewReader(br)
		if err != nil {
			return nil, fmt.Errorf("cannot read gzip archive %q: %w", fileName, err)
		}
		defer gz.Close()
		reader = gz
	}
	tr := tar.NewReader(reader)
	var entries []archiveManifestEntry
	for {
		h, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, fmt.Errorf("%q is not a supported ZIP, TAR or TAR.GZ archive: %w", fileName, err)
		}
		if !h.FileInfo().Mode().IsRegular() {
			continue
		}
		name, err := safeArchivePath(h.Name)
		if err != nil {
			return nil, fmt.Errorf("invalid TAR entry: %w", err)
		}
		if h.Size < 0 {
			return nil, fmt.Errorf("invalid TAR entry %q: negative size", name)
		}
		entries = append(entries, archiveManifestEntry{Name: name, Size: uint64(h.Size)})
	}
	return entries, nil
}

func safeArchivePath(name string) (string, error) {
	name = strings.ReplaceAll(name, "\\", "/")
	clean := path.Clean(name)
	windowsAbsolute := len(clean) >= 3 && clean[1] == ':' && clean[2] == '/'
	if clean == "." || strings.HasPrefix(clean, "/") || windowsAbsolute || clean == ".." || strings.HasPrefix(clean, "../") {
		return "", fmt.Errorf("unsafe path %q", name)
	}
	return clean, nil
}

func buildROCrate(metadata archiveManifestMetadata, entries []archiveManifestEntry) roCrateDocument {
	parts := make([]roCrateRef, 0, len(entries))
	graph := make([]roCrateEntity, 0, len(entries)+2)
	graph = append(graph, roCrateEntity{
		ID: "ro-crate-metadata.json", Type: "CreativeWork",
		About: &roCrateRef{ID: "./"}, ConformsTo: &roCrateRef{ID: roCrateProfile},
	})
	graph = append(graph, roCrateEntity{
		ID: "./", Type: "Dataset", Identifier: metadata.Identifier,
		Name: metadata.Name, Description: metadata.Description, DatePublished: metadata.Published,
		HasPart: parts,
	})
	for _, entry := range entries {
		id := (&url.URL{Path: entry.Name}).EscapedPath()
		parts = append(parts, roCrateRef{ID: id})
		format := mime.TypeByExtension(strings.ToLower(filepath.Ext(entry.Name)))
		if format == "" {
			format = "application/octet-stream"
		}
		graph = append(graph, roCrateEntity{
			ID: id, Type: "File", Name: path.Base(entry.Name),
			ContentSize: strconv.FormatUint(entry.Size, 10), EncodingFormat: format,
		})
	}
	graph[1].HasPart = parts
	return roCrateDocument{Context: roCrateContext, Graph: graph}
}

func addAspectContent(
	ctxt context.Context,
	entity, schema, policy string,
	content []byte,
	adapter *adpt.Adapter,
) (string, error) {
	res, err := sdk.AddUpdateAspect(ctxt, true, entity, schema, policy, content, adapter, logger)
	if err != nil {
		return "", err
	}
	obj, err := res.AsObject()
	if err != nil {
		return "", fmt.Errorf("cannot parse aspect response: %w", err)
	}
	for _, key := range []string{"id", "record-id"} {
		if id, ok := obj[key].(string); ok && id != "" {
			return id, nil
		}
	}
	return "", errors.New("aspect response is missing its id")
}
