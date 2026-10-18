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
	"regexp"
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

	// licenseUndefined is the default of --license: the license of the content is not known.
	licenseUndefined = "undefined"
)

// roCrateLicense is the contextual entity the root dataset of a manifest points at.
type roCrateLicense struct {
	ID          string
	Name        string
	Description string
}

var spdxIdentifier = regexp.MustCompile(`^[A-Za-z0-9][A-Za-z0-9.+-]*$`)

// resolveLicense turns a --license value into the license entity of the manifest. The value is
// 'undefined' (or empty), an absolute http(s) URL, or an SPDX license identifier. Identifiers
// are not checked against the SPDX list, so a misspelt one yields a link that does not resolve.
func resolveLicense(value string) (roCrateLicense, error) {
	v := strings.TrimSpace(value)
	switch {
	case v == "" || strings.EqualFold(v, licenseUndefined):
		return roCrateLicense{
			ID:          "#license-undefined",
			Name:        "License undefined",
			Description: "No license was specified for the content of this archive, so its terms of use are unknown.",
		}, nil
	case strings.HasPrefix(v, "http://") || strings.HasPrefix(v, "https://"):
		u, err := url.Parse(v)
		if err != nil || u.Host == "" {
			return roCrateLicense{}, invalidLicense(value)
		}
		return roCrateLicense{ID: v, Name: v, Description: "License given as a URL: " + v}, nil
	case spdxIdentifier.MatchString(v):
		return roCrateLicense{
			ID: "https://spdx.org/licenses/" + v, Name: v, Description: "SPDX license identifier " + v + ".",
		}, nil
	}
	return roCrateLicense{}, invalidLicense(value)
}

func invalidLicense(value string) error {
	return fmt.Errorf("invalid --license %q: use an SPDX license identifier such as CC-BY-4.0, "+
		"a license URL starting with http:// or https://, or 'undefined'", value)
}

type archiveManifestMetadata struct {
	Identifier  string
	Name        string
	Description string
	Published   string
	License     roCrateLicense // zero value: license undefined
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
	License        *roCrateRef  `json:"license,omitempty"`
	DatePublished  string       `json:"datePublished,omitempty"`
	ContentSize    string       `json:"contentSize,omitempty"`
	EncodingFormat string       `json:"encodingFormat,omitempty"`
}

type roCrateRef struct {
	ID string `json:"@id"`
}

// isManifestArchive reports whether the file gets a manifest: a regular file whose name ends
// in .zip, .tar, .tar.gz or .tgz. Only the name decides. The content is deliberately not
// sniffed, so ZIP-based formats such as .docx or .jar are not indexed and pipes are never read.
func isManifestArchive(fileName string) bool {
	cleanName := filepath.Clean(fileName)
	info, err := os.Stat(cleanName)
	if err != nil || !info.Mode().IsRegular() {
		return false // a missing or unreadable file is reported by the upload itself
	}
	lowerName := strings.ToLower(cleanName)
	for _, suffix := range []string{".zip", ".tar", ".tar.gz", ".tgz"} {
		if strings.HasSuffix(lowerName, suffix) {
			return true
		}
	}
	return false
}

// manifestForUpload prepares the archive manifest for an upload. The manifest is a
// convenience, so a file that cannot be indexed is reported as a warning and uploaded
// without one.
func manifestForUpload(fileName string, warn io.Writer) []byte {
	content, err := prepareArtifactManifest(fileName)
	if err != nil {
		fmt.Fprintf(warn, "Warning: not adding a manifest for '%s': %v\n", fileName, err)
		return nil
	}
	return content
}

// warnManifestNotAttached reports that the artifact is complete but its manifest aspect could
// not be added. Running the same command again retries only the manifest.
func warnManifestNotAttached(warn io.Writer, artifactID string, err error) {
	fmt.Fprintf(warn, "Warning: artifact '%s' is uploaded, but its manifest aspect could not be added: %v\n"+
		"Run the same command again to retry the manifest.\n", artifactID, err)
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
	lic := metadata.License
	if lic.ID == "" {
		lic, _ = resolveLicense(licenseUndefined)
	}

	// An archive can hold the same path twice (extracting lets the later entry win), but every
	// @id must be unique: keep one entity per path, with the size of the last entry.
	unique := make([]archiveManifestEntry, 0, len(entries))
	position := make(map[string]int, len(entries))
	for _, entry := range entries {
		if i, seen := position[entry.Name]; seen {
			unique[i].Size = entry.Size
			continue
		}
		position[entry.Name] = len(unique)
		unique = append(unique, entry)
	}

	parts := make([]roCrateRef, 0, len(unique))
	graph := make([]roCrateEntity, 0, len(unique)+3)
	graph = append(graph, roCrateEntity{
		ID: "ro-crate-metadata.json", Type: "CreativeWork",
		About: &roCrateRef{ID: "./"}, ConformsTo: &roCrateRef{ID: roCrateProfile},
	})
	graph = append(graph, roCrateEntity{
		ID: "./", Type: "Dataset", Identifier: metadata.Identifier,
		Name: metadata.Name, Description: metadata.Description, DatePublished: metadata.Published,
		License: &roCrateRef{ID: lic.ID}, HasPart: parts,
	})
	for _, entry := range unique {
		id := roCratePathID(entry.Name)
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
	graph = append(graph, roCrateEntity{
		ID: lic.ID, Type: "CreativeWork", Name: lic.Name, Description: lic.Description,
	})
	graph[1].HasPart = parts
	return roCrateDocument{Context: roCrateContext, Graph: graph}
}

// roCratePathID turns an archive path into an RO-Crate @id, which is a relative URI reference.
// A first path segment that contains a colon would read as a URI scheme (RFC 3986, section 4.2),
// so such a path gets a "./" prefix.
func roCratePathID(name string) string {
	id := (&url.URL{Path: name}).EscapedPath()
	if first, _, _ := strings.Cut(id, "/"); strings.Contains(first, ":") {
		return "./" + id
	}
	return id
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

// findAspectID returns the ID of an existing aspect with exactly this schema on the entity,
// or "" when there is none. The schema filter of the API matches by prefix, so the result is
// compared exactly.
func findAspectID(ctxt context.Context, entity, schema string, adapter *adpt.Adapter) (string, error) {
	list, _, err := sdk.ListAspect(ctxt, sdk.AspectSelector{Entity: entity, SchemaPrefix: schema}, adapter, logger)
	if err != nil {
		return "", fmt.Errorf("cannot check for an existing %s aspect: %w", schema, err)
	}
	for _, item := range list.Items {
		if item.ID != nil && item.Schema != nil && *item.Schema == schema {
			return *item.ID, nil
		}
	}
	return "", nil
}
