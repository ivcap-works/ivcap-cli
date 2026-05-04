// Copyright 2023 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
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
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strings"

	sdk "github.com/ivcap-works/ivcap-cli/pkg"
	a "github.com/ivcap-works/ivcap-cli/pkg/adapter"

	"github.com/jedib0t/go-pretty/v6/table"
	"github.com/spf13/cobra"
	log "go.uber.org/zap"
)

var (
	useCache bool

	tarCmd = &cobra.Command{
		Use:   "tar",
		Short: "Manage tar/tar.gz artifacts",
		Long:  `List and extract files from tar/tar.gz artifacts, with optional caching`,
	}

	tarListCmd = &cobra.Command{
		Use:     "list artifact_id [--cache]",
		Short:   "List files in a tar/tar.gz artifact",
		Aliases: []string{"ls"},
		Args:    cobra.ExactArgs(1),
		RunE:    tarListFunc,
	}

	tarDownloadCmd = &cobra.Command{
		Use:   "download artifact_id file_path [-f output_file] [--cache]",
		Short: "Download a specific file from a tar/tar.gz artifact",
		Args:  cobra.ExactArgs(2),
		RunE:  tarDownloadFunc,
	}

	tarCleanCmd = &cobra.Command{
		Use:   "clean",
		Short: "Remove all cached tar artifacts",
		RunE:  tarCleanFunc,
	}
)

func init() {
	artifactCmd.AddCommand(tarCmd)
	tarCmd.AddCommand(tarListCmd)
	tarCmd.AddCommand(tarDownloadCmd)
	tarCmd.AddCommand(tarCleanCmd)

	tarListCmd.Flags().BoolVar(&useCache, "cache", false, "Cache the artifact locally for future access")
	tarDownloadCmd.Flags().BoolVar(&useCache, "cache", false, "Cache the artifact locally for future access")
	tarDownloadCmd.Flags().StringVarP(&fileName, "file", "f", "", "File to write content to [stdout]")
}

type tarFileInfo struct {
	Name string
	Size int64
	Mode string
}

func tarListFunc(cmd *cobra.Command, args []string) error {
	artifactID := GetHistory(args[0])
	ctx := context.Background()
	adapter := CreateAdapter(true)

	// Get artifact info and download
	data, mimeType, err := downloadTarArtifact(ctx, artifactID, adapter)
	if err != nil {
		return err
	}

	// Verify it's a tar file
	if !isTarFile(mimeType, data) {
		return fmt.Errorf("artifact '%s' is not a tar or tar.gz file (mime-type: %s)", artifactID, mimeType)
	}

	// Cache if requested
	if useCache {
		if err := saveTarCache(artifactID, data); err != nil {
			logger.Warn("failed to cache artifact", log.Error(err))
		}
	}

	// List files
	files, err := listTarFiles(data)
	if err != nil {
		return fmt.Errorf("failed to list tar contents: %w", err)
	}

	// Print table
	printTarFileTable(files)
	return nil
}

func tarDownloadFunc(cmd *cobra.Command, args []string) error {
	artifactID := GetHistory(args[0])
	filePath := args[1]
	ctx := context.Background()
	adapter := CreateAdapter(true)

	// Try to get from cache first
	var data []byte
	var err error

	cachedData, found := loadTarCache(artifactID)
	if found {
		logger.Debug("using cached artifact", log.String("id", artifactID))
		data = cachedData
	} else {
		// Download artifact
		var mimeType string
		data, mimeType, err = downloadTarArtifact(ctx, artifactID, adapter)
		if err != nil {
			return err
		}

		// Verify it's a tar file
		if !isTarFile(mimeType, data) {
			return fmt.Errorf("artifact '%s' is not a tar or tar.gz file (mime-type: %s)", artifactID, mimeType)
		}

		// Cache if requested
		if useCache {
			if err := saveTarCache(artifactID, data); err != nil {
				logger.Warn("failed to cache artifact", log.Error(err))
			}
		}
	}

	// Extract file
	fileData, err := extractFileFromTar(data, filePath)
	if err != nil {
		return fmt.Errorf("failed to extract file '%s': %w", filePath, err)
	}

	// Write to output
	return writeOutput(fileData)
}

func tarCleanFunc(cmd *cobra.Command, args []string) error {
	cacheDir, err := getTarCacheDir()
	if err != nil {
		return err
	}

	if _, err := os.Stat(cacheDir); os.IsNotExist(err) {
		fmt.Println("No cache directory found")
		return nil
	}

	entries, err := os.ReadDir(cacheDir)
	if err != nil {
		return fmt.Errorf("failed to read cache directory: %w", err)
	}

	count := 0
	for _, entry := range entries {
		if !entry.IsDir() && strings.HasSuffix(entry.Name(), ".tar.cache") {
			path := filepath.Join(cacheDir, entry.Name())
			if err := os.Remove(path); err != nil {
				logger.Warn("failed to remove cache file", log.String("path", path), log.Error(err))
			} else {
				count++
			}
		}
	}

	fmt.Printf("Removed %d cached artifact(s)\n", count)
	return nil
}

// downloadTarArtifact downloads an artifact and returns its data and mime type
func downloadTarArtifact(ctx context.Context, artifactID string, adapter *a.Adapter) ([]byte, string, error) {
	req := &sdk.ReadArtifactRequest{Id: artifactID}
	artifact, err := sdk.ReadArtifact(ctx, req, adapter, logger)
	if err != nil {
		return nil, "", err
	}

	if artifact.DataHref == nil {
		return nil, "", fmt.Errorf("artifact has no data")
	}

	dataURL, err := url.ParseRequestURI(*artifact.DataHref)
	if err != nil {
		return nil, "", err
	}

	var data []byte
	downloadHandler := func(resp *http.Response, path string, logger *log.Logger) error {
		if resp.StatusCode >= 300 {
			return a.ProcessErrorResponse(resp, path, nil, logger)
		}

		var reader io.Reader
		if silent {
			reader = resp.Body
		} else {
			reader = sdk.AddProgressBar("... downloading artifact", resp.ContentLength, resp.Body)
		}

		buf := new(bytes.Buffer)
		_, err := io.Copy(buf, reader)
		if err != nil {
			return err
		}
		data = buf.Bytes()
		return nil
	}

	err = (*adapter).GetWithHandler(ctx, dataURL.Path, nil, downloadHandler, logger)
	if err != nil {
		return nil, "", err
	}

	mimeType := "application/octet-stream"
	if artifact.MimeType != nil {
		mimeType = *artifact.MimeType
	}

	if !silent {
		fmt.Printf("\n") // Move past progress bar
	}

	return data, mimeType, nil
}

// isTarFile checks if the data appears to be a tar or tar.gz file
func isTarFile(mimeType string, data []byte) bool {
	mt := strings.ToLower(mimeType)
	if strings.Contains(mt, "tar") || strings.Contains(mt, "gzip") || strings.Contains(mt, "tgz") {
		return true
	}
	// Check for gzip magic bytes
	return len(data) > 2 && data[0] == 0x1f && data[1] == 0x8b
}

// listTarFiles extracts file information from a tar archive
func listTarFiles(data []byte) ([]tarFileInfo, error) {
	tr, err := openTarReader(data)
	if err != nil {
		return nil, err
	}

	var files []tarFileInfo
	for {
		header, err := tr.Next()
		if err == io.EOF {
			break
		}
		if err != nil {
			return nil, err
		}

		// Skip directories in listing (they don't have meaningful sizes)
		if header.Typeflag == tar.TypeDir {
			continue
		}

		files = append(files, tarFileInfo{
			Name: header.Name,
			Size: header.Size,
			Mode: fmt.Sprintf("%o", header.Mode),
		})
	}

	return files, nil
}

// extractFileFromTar extracts a specific file from a tar archive
func extractFileFromTar(data []byte, filePath string) ([]byte, error) {
	tr, err := openTarReader(data)
	if err != nil {
		return nil, err
	}

	// Normalize the path
	filePath = normalizeFilePath(filePath)

	for {
		header, err := tr.Next()
		if err == io.EOF {
			return nil, fmt.Errorf("file '%s' not found in archive", filePath)
		}
		if err != nil {
			return nil, err
		}

		// Normalize the tar entry name
		entryPath := normalizeFilePath(header.Name)
		if entryPath == filePath {
			return io.ReadAll(tr)
		}
	}
}

// openTarReader creates a tar reader, handling both .tar and .tar.gz
func openTarReader(data []byte) (*tar.Reader, error) {
	// Check if it's gzipped
	if len(data) > 2 && data[0] == 0x1f && data[1] == 0x8b {
		gzr, err := gzip.NewReader(bytes.NewReader(data))
		if err != nil {
			return nil, fmt.Errorf("failed to create gzip reader: %w", err)
		}
		return tar.NewReader(gzr), nil
	}
	return tar.NewReader(bytes.NewReader(data)), nil
}

// normalizeFilePath normalizes a file path for comparison
func normalizeFilePath(p string) string {
	// Remove leading slashes and "./"
	p = strings.TrimPrefix(p, "/")
	p = strings.TrimPrefix(p, "./")
	return p
}

// getTarCacheDir returns the directory for tar cache files
func getTarCacheDir() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("failed to get home directory: %w", err)
	}
	cacheDir := filepath.Join(homeDir, ".ivcap", "tar-cache")
	return cacheDir, nil
}

// saveTarCache saves an artifact to the cache
func saveTarCache(artifactID string, data []byte) error {
	cacheDir, err := getTarCacheDir()
	if err != nil {
		return err
	}

	if err := os.MkdirAll(cacheDir, 0755); err != nil { // #nosec G301 -- cache directory under user's home
		return fmt.Errorf("failed to create cache directory: %w", err)
	}

	// Use a safe filename based on artifact ID
	safeID := strings.ReplaceAll(artifactID, ":", "_")
	safeID = strings.ReplaceAll(safeID, "/", "_")
	cachePath := filepath.Join(cacheDir, safeID+".tar.cache")

	if err := os.WriteFile(cachePath, data, 0644); err != nil { // #nosec G306 -- cache file under user's home
		return fmt.Errorf("failed to write cache file: %w", err)
	}

	logger.Debug("cached artifact", log.String("id", artifactID), log.String("path", cachePath))
	return nil
}

// loadTarCache loads an artifact from the cache
func loadTarCache(artifactID string) ([]byte, bool) {
	cacheDir, err := getTarCacheDir()
	if err != nil {
		return nil, false
	}

	safeID := strings.ReplaceAll(artifactID, ":", "_")
	safeID = strings.ReplaceAll(safeID, "/", "_")
	cachePath := filepath.Join(cacheDir, safeID+".tar.cache")

	data, err := os.ReadFile(cachePath) // #nosec G304 -- cachePath is constructed from safe getTarCacheDir
	if err != nil {
		return nil, false
	}

	return data, true
}

// writeOutput writes data to the specified output file or stdout
func writeOutput(data []byte) error {
	var outFile *os.File
	var err error

	if fileName == "" || fileName == "-" {
		outFile = os.Stdout
	} else {
		outFile, err = os.Create(filepath.Clean(fileName))
		if err != nil {
			return fmt.Errorf("failed to create output file: %w", err)
		}
		defer outFile.Close()
	}

	_, err = outFile.Write(data)
	if err != nil {
		return fmt.Errorf("failed to write output: %w", err)
	}

	if fileName != "" && fileName != "-" {
		fmt.Printf("Extracted file written to '%s'\n", fileName)
	}

	return nil
}

// printTarFileTable prints a table of tar file contents
func printTarFileTable(files []tarFileInfo) {
	t := table.NewWriter()
	t.SetOutputMirror(os.Stdout)
	t.AppendHeader(table.Row{"Name", "Size", "Mode"})

	rows := make([]table.Row, len(files))
	for i, f := range files {
		rows[i] = table.Row{f.Name, safeBytes(&f.Size), f.Mode}
	}
	t.AppendRows(rows)
	t.Render()
}
