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

package client

import (
	"bytes"
	"context"
	"encoding/base64"
	"fmt"
	"io"
	"net/url"
	"strconv"

	api "github.com/ivcap-works/ivcap-core-api/http/artifact"

	"github.com/k0kubun/go-ansi"
	"github.com/schollz/progressbar/v3"

	"github.com/ivcap-works/ivcap-cli/pkg/adapter"

	log "go.uber.org/zap"
)

/**** LIST ****/

type ListArtifactRequest struct {
	Offset int
	Limit  int
	Page   *string
}

func ListArtifacts(ctxt context.Context, cmd *ListRequest, adpt *adapter.Adapter, logger *log.Logger) (*api.ListResponseBody, error) {
	pyl, err := ListArtifactsRaw(ctxt, cmd, adpt, logger)
	if err != nil {
		return nil, err
	}
	var list api.ListResponseBody
	if err = pyl.AsType(&list); err != nil {
		return nil, fmt.Errorf("failed to parse list response body: %w", err)
	}

	return &list, nil
}

func ListArtifactsRaw(ctxt context.Context, cmd *ListRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	path, err := createListPath(cmd, artifactPath(nil, adpt))
	if err != nil {
		return nil, err
	}
	// fmt.Printf("PATH: %s\n", path)
	return (*adpt).Get(ctxt, path.String(), logger)
}

// /**** CREATE ****/

type CreateArtifactRequest struct {
	Name       string            `json:"name"`
	Size       int64             `json:"size"`
	Collection string            `json:"collection"`
	Policy     string            `json:"policy"`
	Meta       map[string]string `json:"meta"`
}

func CreateArtifact(
	ctxt context.Context,
	cmd *CreateArtifactRequest,
	contentType string,
	size int64,
	reader io.Reader,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (*api.UploadResponseBody, error) {
	if res, err := CreateArtifactRaw(ctxt, cmd, contentType, size, reader, adpt, logger); err == nil {
		var artifact api.UploadResponseBody
		if err := res.AsType(&artifact); err != nil {
			return nil, err
		}
		return &artifact, nil
	} else {
		return nil, err
	}
}

func UploadArtifact(
	ctxt context.Context,
	reader io.Reader,
	size int64,
	offset int64,
	chunkSize int64,
	path string,
	adpt *adapter.Adapter,
	silent bool,
	logger *log.Logger,
) (err error) {
	if offset > 0 {
		switch r := reader.(type) {
		case io.Seeker:
			if _, err = r.Seek(offset, io.SeekCurrent); err != nil {
				return fmt.Errorf("reader seek error : %w", err)
			}
		default:
			if _, err = io.CopyN(io.Discard, r, offset); err != nil {
				return fmt.Errorf("io copyN error: %w", err)
			}
		}
	}

	if size < 0 {
		// unknown size, just uploading whatever is in the reader
		return uploadUnknownSize(ctxt, reader, offset, chunkSize, path, adpt, logger)
	}

	remaining := size - offset
	fragSize := chunkSize
	if fragSize < 0 {
		fragSize = remaining // no chunking
	}
	if !silent {
		reader = AddProgressBar("... uploading file", remaining, reader)
	}
	// var pyld adapter.Payload
	for remaining > 0 {
		psize := remaining
		if psize > fragSize {
			psize = fragSize
		}
		off := size - remaining
		r := &io.LimitedReader{R: reader, N: psize}
		h := map[string]string{
			"Content-Type":  "application/offset+octet-stream",
			"Upload-Offset": fmt.Sprintf("%d", off),
			"Tus-Resumable": "1.0.0",
		}
		// var pyld adapter.Payload
		_, err = (*adpt).Patch(context.Background(), path, r, psize, &h, logger)
		if err != nil {
			if !silent {
				fmt.Printf("\n") // To move past progress bar
			}
			return
		}
		remaining -= psize - r.N
	}
	if !silent {
		fmt.Printf("\n") // To move past progress bar
	}
	return
}

func uploadUnknownSize(
	ctxt context.Context,
	reader io.Reader,
	offset int64,
	chunkSize int64,
	path string,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (err error) {
	off := offset
	p := make([]byte, chunkSize)
	for {
		h := map[string]string{
			"Content-Type":  "application/offset+octet-stream",
			"Upload-Offset": fmt.Sprintf("%d", off),
			"Tus-Resumable": "1.0.0",
		}

		var n int
		if n, err = reader.Read(p); err != nil || n == 0 {
			if err != nil && err != io.EOF {
				return
			}
			// need to inform about size
			h["Upload-Length"] = fmt.Sprintf("%d", off)
			_, err = (*adpt).Patch(context.Background(), path, nil, 0, &h, logger)
			return
		}
		r := bytes.NewReader(p[:n])
		h["Upload-Defer-Length"] = "1"
		var pyld adapter.Payload
		pyld, err = (*adpt).Patch(context.Background(), path, r, int64(n), &h, logger)
		if err != nil {
			return
		}
		if noffh := pyld.Header("Upload-Offset"); noffh == "" {
			return fmt.Errorf("missing 'Upload-Offset' header")
		} else {
			if noff, err := strconv.ParseInt(noffh, 10, 64); err != nil {
				return err
			} else {
				if (off + int64(n)) != noff {
					return fmt.Errorf("unexpected 'Upload-Offset', expected %d but got %d", off+int64(n), noff)
				}
				off = noff
			}
		}
	}
}

func AddProgressBar(description string, size int64, reader io.Reader) io.Reader {
	bar := GetProgressBar(description, size)
	return io.TeeReader(reader, bar)
}

func GetProgressBar(description string, size int64) io.Writer {
	return progressbar.NewOptions64(size,
		progressbar.OptionSetWriter(ansi.NewAnsiStderr()),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(30),
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)
}

func CreateArtifactRaw(
	ctxt context.Context,
	cmd *CreateArtifactRequest,
	contentType string,
	size int64,
	reader io.Reader,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (adapter.Payload, error) {
	path := artifactPath(nil, adpt)
	headers := make(map[string]string)
	contentLength := cmd.Size
	if reader != nil {
		headers["Upload-Length"] = fmt.Sprintf("%d", cmd.Size)
		headers["Tus-Resumable"] = "1.0.0"
		if size > 0 {
			headers["Upload-Length"] = fmt.Sprintf("%d", size)
		}
		headers["Content-Type"] = contentType
	} else {
		headers["X-Content-Type"] = contentType
		headers["X-Content-Length"] = fmt.Sprintf("%d", size)
		contentLength = 0
	}
	if cmd.Name != "" {
		headers["X-Name"] = BaseEncode(cmd.Name)
	}
	if cmd.Collection != "" {
		headers["X-Collection"] = cmd.Collection
	}
	if cmd.Policy != "" {
		headers["X-Policy"] = cmd.Policy
	}
	var meta []string
	for key, value := range cmd.Meta {
		k := BaseEncode(key)
		v := BaseEncode(value)
		meta = append(meta, fmt.Sprintf("%s %s", k, v))
	}
	if len(meta) > 0 {
		headers["Upload-Metadata"] = cmd.Name
	}
	return (*adpt).Post(ctxt, path, reader, contentLength, &headers, logger)
}

func BaseEncode(value string) string {
	return base64.StdEncoding.EncodeToString([]byte(value))
}

/**** READ ****/

type ReadArtifactRequest struct {
	Id string
}

func ReadArtifact(ctxt context.Context, cmd *ReadArtifactRequest, adpt *adapter.Adapter, logger *log.Logger) (*api.ReadResponseBody, error) {
	if res, err := ReadArtifactRaw(ctxt, cmd, adpt, logger); err == nil {
		var artifact api.ReadResponseBody
		if err := res.AsType(&artifact); err != nil {
			return nil, err
		}
		return &artifact, nil
	} else {
		return nil, err
	}
}

func ReadArtifactRaw(ctxt context.Context, cmd *ReadArtifactRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	path := artifactPath(&cmd.Id, adpt)
	return (*adpt).Get(ctxt, path, logger)
}

/**** COLLECTION ****/

func AddArtifactToCollection(
	ctxt context.Context,
	artifactID string,
	collectionName string,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (adapter.Payload, error) {
	path := artifactPath(&artifactID, adpt)
	path = fmt.Sprintf("%s/.collections/%s", path, url.PathEscape(collectionName))
	return (*adpt).Put(ctxt, path, nil, -1, nil, logger)
}

func RemoveArtifactToCollection(
	ctxt context.Context,
	artifactID string,
	collectionName string,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (adapter.Payload, error) {
	path := artifactPath(&artifactID, adpt)
	path = fmt.Sprintf("%s/.collections/%s", path, url.PathEscape(collectionName))
	return (*adpt).Delete(ctxt, path, logger)
}

/**** METADATA ****/

func AddArtifactMeta(
	ctxt context.Context,
	artifactID string,
	schemaName string,
	meta io.Reader,
	size int64,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (adapter.Payload, error) {
	path := artifactPath(&artifactID, adpt)
	path = fmt.Sprintf("%s/.metadata/%s", path, url.PathEscape(schemaName))
	return (*adpt).Put(ctxt, path, meta, size, nil, logger)
}

/**** UTILS ****/

func artifactPath(id *string, adpt *adapter.Adapter) string {
	path := "/1/artifacts"
	if id != nil {
		path = path + "/" + *id
	}
	return path
}
