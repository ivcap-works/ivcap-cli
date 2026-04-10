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

package nextflow

import (
	"archive/tar"
	"bytes"
	"compress/gzip"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"path"
	"strings"
	"time"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"
	log "go.uber.org/zap"
)

// Source describes a file which should become a file at `Path` inside an assembled
// Nextflow pipeline tar.gz.
type Source struct {
	// Path inside the assembled archive.
	Path string `json:"path"`
	// One of: text, base64, url, artifact
	Type string `json:"type"`
	// For text
	Text string `json:"text,omitempty"`
	// For base64
	Base64 string `json:"base64,omitempty"`
	// For url
	URL string `json:"url,omitempty"`
	// For artifact
	ArtifactID   string `json:"artifact_id,omitempty"`
	ArtifactPath string `json:"artifact_path,omitempty"`
	// Optional mime type for manifest.
	MediaType string `json:"media_type,omitempty"`
}

// TarGzFromSources assembles a tar.gz from the provided sources and also returns a
// compact JSON manifest string (suitable for storing in artifact meta).
//
// `fetchURLBytes` and `downloadArtifactBytes` are injected to keep this package
// independent of cmd/* MCP helpers.
func TarGzFromSources(
	ctx context.Context,
	sources []Source,
	adpt *a.Adapter,
	logger *log.Logger,
	fetchURLBytes func(context.Context, string) ([]byte, string, error),
	downloadArtifactBytes func(context.Context, string, *a.Adapter) ([]byte, error),
) ([]byte, string, error) {
	var buf bytes.Buffer
	gzw := gzip.NewWriter(&buf)
	tw := tar.NewWriter(gzw)
	defer func() {
		_ = tw.Close()
		_ = gzw.Close()
	}()

	used := map[string]bool{}
	manifest := make([]manifestEntry, 0, len(sources))

	for i, src := range sources {
		if strings.TrimSpace(src.Path) == "" {
			return nil, "", fmt.Errorf("sources[%d].path is required", i)
		}
		tarPath, err := sanitizeTarPath(src.Path)
		if err != nil {
			return nil, "", fmt.Errorf("invalid sources[%d].path: %w", i, err)
		}
		if tarPath == "MANIFEST.json" {
			return nil, "", fmt.Errorf("sources[%d].path cannot be MANIFEST.json", i)
		}
		if used[tarPath] {
			return nil, "", fmt.Errorf("duplicate path %q", tarPath)
		}
		used[tarPath] = true

		data, mt, sourceDesc, err := sourceToBytes(ctx, src, adpt, logger, fetchURLBytes, downloadArtifactBytes)
		if err != nil {
			return nil, "", err
		}
		if mt == "" {
			mt = src.MediaType
		}

		hdr := &tar.Header{Name: tarPath, Mode: 0o644, Size: int64(len(data)), ModTime: time.Now()}
		if err := tw.WriteHeader(hdr); err != nil {
			return nil, "", err
		}
		if _, err := tw.Write(data); err != nil {
			return nil, "", err
		}

		manifest = append(manifest, manifestEntry{Path: tarPath, Type: src.Type, MediaType: mt, Source: sourceDesc, Size: len(data)})
	}

	mb, err := json.MarshalIndent(map[string]any{"files": manifest}, "", "  ")
	if err != nil {
		return nil, "", err
	}
	if err := tw.WriteHeader(&tar.Header{Name: "MANIFEST.json", Mode: 0o644, Size: int64(len(mb)), ModTime: time.Now()}); err != nil {
		return nil, "", err
	}
	if _, err := tw.Write(mb); err != nil {
		return nil, "", err
	}

	if err := tw.Close(); err != nil {
		return nil, "", err
	}
	if err := gzw.Close(); err != nil {
		return nil, "", err
	}

	compact, _ := json.Marshal(map[string]any{"files": manifest})
	return buf.Bytes(), string(compact), nil
}

type manifestEntry struct {
	Path      string `json:"path"`
	Type      string `json:"type"`
	MediaType string `json:"media_type,omitempty"`
	Source    string `json:"source,omitempty"`
	Size      int    `json:"size"`
}

func sourceToBytes(
	ctx context.Context,
	src Source,
	adpt *a.Adapter,
	logger *log.Logger,
	fetchURLBytes func(context.Context, string) ([]byte, string, error),
	downloadArtifactBytes func(context.Context, string, *a.Adapter) ([]byte, error),
) ([]byte, string, string, error) {
	switch src.Type {
	case "text":
		return []byte(src.Text), "text/plain; charset=utf-8", "text", nil
	case "base64":
		decoded, err := base64.StdEncoding.DecodeString(src.Base64)
		if err != nil {
			return nil, "", "", fmt.Errorf("invalid base64 for %q: %w", src.Path, err)
		}
		mt := src.MediaType
		if mt == "" {
			mt = "application/octet-stream"
		}
		return decoded, mt, "base64", nil
	case "url":
		if src.URL == "" {
			return nil, "", "", fmt.Errorf("url source for %q missing url", src.Path)
		}
		if fetchURLBytes == nil {
			return nil, "", "", fmt.Errorf("url sources not supported (fetchURLBytes is nil)")
		}
		b, mt, err := fetchURLBytes(ctx, src.URL)
		if err != nil {
			return nil, "", "", err
		}
		if src.MediaType != "" {
			mt = src.MediaType
		}
		if mt == "" {
			mt = "application/octet-stream"
		}
		return b, mt, "url:" + src.URL, nil
	case "artifact":
		if src.ArtifactID == "" {
			return nil, "", "", fmt.Errorf("artifact source for %q missing artifact_id", src.Path)
		}
		if downloadArtifactBytes == nil {
			return nil, "", "", fmt.Errorf("artifact sources not supported (downloadArtifactBytes is nil)")
		}
		art, err := sdk.ReadArtifact(ctx, &sdk.ReadArtifactRequest{Id: src.ArtifactID}, adpt, logger)
		if err != nil {
			return nil, "", "", err
		}
		if art == nil || art.DataHref == nil {
			return nil, "", "", fmt.Errorf("artifact %q has no data", src.ArtifactID)
		}
		mime := safeString(art.MimeType)
		dataURL := *art.DataHref
		data, err := downloadArtifactBytes(ctx, dataURL, adpt)
		if err != nil {
			return nil, "", "", err
		}
		if src.ArtifactPath != "" {
			inner, innerMT, err := extractFromTarAuto(data, src.ArtifactPath)
			if err != nil {
				return nil, "", "", err
			}
			if src.MediaType != "" {
				innerMT = src.MediaType
			}
			return inner, innerMT, fmt.Sprintf("artifact:%s#%s", src.ArtifactID, path.Clean(strings.TrimPrefix(src.ArtifactPath, "/"))), nil
		}
		if src.MediaType != "" {
			mime = src.MediaType
		}
		if mime == "" {
			mime = "application/octet-stream"
		}
		return data, mime, "artifact:" + src.ArtifactID, nil
	default:
		return nil, "", "", fmt.Errorf("unsupported source type %q for %q", src.Type, src.Path)
	}
}
