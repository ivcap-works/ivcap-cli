// Copyright 2024 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
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

package client

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"github.com/inhies/go-bytesize"
	log "go.uber.org/zap"

	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/ivcap-works/ivcap-cli/pkg/adapter"
	api "github.com/ivcap-works/ivcap-core-api/http/pkg"
)

/********** packages operations ************/

func ListPackages(ctxt context.Context, tag string, adpt *adapter.Adapter, logger *log.Logger) (*api.ListResponseBody, error) {
	path := pkgPath(nil) + "/list"
	if tag != "" {
		srcTag, err := name.NewTag(tag, name.WeakValidation)
		if err != nil {
			return nil, fmt.Errorf("invalid src tag format: %w", err)
		}
		tag = srcTag.String()
	}

	q := url.Values{}
	q.Set("tag", tag)
	path += "?" + q.Encode()

	res, err := (*adpt).Get(ctxt, path, logger)
	if err != nil {
		return nil, fmt.Errorf("failed to list service packages: %w", err)
	}

	var body api.ListResponseBody
	if err := res.AsType(&body); err != nil {
		return nil, fmt.Errorf("failed to decode list package response body; %w", err)
	}

	return &body, nil
}

func PushServicePackage(ctxt context.Context, srcTagName string, forcePush bool, adpt *adapter.Adapter, logger *log.Logger) (*api.PushResponseBody, error) {
	srcTag, err := name.NewTag(srcTagName, name.WeakValidation)
	if err != nil {
		return nil, fmt.Errorf("invalid src tag format: %w", err)
	}

	path := pkgPath(nil) + "/push"
	q := url.Values{}
	q.Set("tag", srcTag.String())
	q.Set("force", strconv.FormatBool(forcePush))
	path += "?" + q.Encode()

	// copy docker image
	ref, err := name.ParseReference(srcTag.String())
	if err != nil {
		return nil, fmt.Errorf("failed to parse name reference: %s, %w", srcTag.String(), err)
	}

	img, err := daemon.Image(ref)
	if err != nil {
		return nil, fmt.Errorf("reading image %q: %w", ref, err)
	}

	errChan := make(chan error, 1)
	updatesChan := make(chan v1.Update)
	// Write
	reader, writer := io.Pipe()
	go func() {
		defer func() {
			if err := writer.Close(); err != nil {
				fmt.Printf("failed to close pipe writer: %s\n", err)
			}
		}()
		if err := tarball.Write(ref, img, writer, tarball.WithProgress(updatesChan)); err != nil {
			errChan <- err
		}
	}()

	go func() {
		for update := range updatesChan {
			if errors.Is(update.Error, io.EOF) {
				fmt.Printf("\033[2K\rDone! %s completed out of %s\n", bytesize.New(float64(update.Complete)), bytesize.New(float64(update.Total)))
				return
			}
			fmt.Printf("\033[2K\rUploading..., %s completed out of %s", bytesize.New(float64(update.Complete)), bytesize.New(float64(update.Total)))
		}
	}()

	res, err := (*adpt).Post(ctxt, path, reader, -1, nil, logger)
	if err != nil {
		// error type assertion with goa ???
		if strings.Contains(err.Error(), "already created") {
			return nil, fmt.Errorf("tag: %s already created, use -f to force overwrite", srcTag)
		}
		return nil, fmt.Errorf("failed to push service package: %w", err)
	}

	if len(errChan) > 0 {
		err := <-errChan
		return nil, fmt.Errorf("failed to write image: %w", err)
	}

	var body api.PushResponseBody
	if err := res.AsType(&body); err != nil {
		return nil, fmt.Errorf("failed to decode update service response body; %w", err)
	}

	return &body, nil
}

func PullPackage(ctxt context.Context, tag string, adpt *adapter.Adapter, logger *log.Logger) error {
	srcTag, err := name.NewTag(tag, name.WeakValidation)
	if err != nil {
		return fmt.Errorf("invalid src tag format: %w", err)
	}
	tag = srcTag.String()

	path := pkgPath(nil) + "/pull"
	q := url.Values{}
	q.Set("tag", tag)
	path += "?" + q.Encode()

	handler := func(resp *http.Response, path string, logger *log.Logger) error {
		if resp.StatusCode != 200 {
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read res body: %w", err)
			}
			return fmt.Errorf("statusCode: %d, error: %s", resp.StatusCode, string(data))
		}
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read res body: %w", err)
		}

		// the tarfile needs to seek to the beginning of the stream to check existence
		var bytesOpener = func(b []byte) tarball.Opener {
			return func() (io.ReadCloser, error) {
				return io.NopCloser(bytes.NewReader(b)), nil
			}
		}

		img, err := tarball.Image(bytesOpener(data), nil)
		if err != nil {
			return fmt.Errorf("failed to read image: %w", err)
		}

		_, err = daemon.Write(srcTag, img)
		if err != nil {
			return fmt.Errorf("failed to save image: %w", err)
		}

		fmt.Printf("%s\n", srcTag)

		return nil
	}

	return (*adpt).GetWithHandler(ctxt, path, nil, handler, logger)
}

func RemovePackage(ctxt context.Context, tag string, adpt *adapter.Adapter, logger *log.Logger) error {
	srcTag, err := name.NewTag(tag, name.WeakValidation)
	if err != nil {
		return fmt.Errorf("invalid src tag format: %w", err)
	}
	tag = srcTag.String()

	path := pkgPath(nil) + "/remove"
	q := url.Values{}
	q.Set("tag", tag)
	path += "?" + q.Encode()

	_, err = (*adpt).Delete(ctxt, path, logger)
	if err != nil {
		return fmt.Errorf("failed to remove service packages: %w", err)
	}

	return nil
}

func pkgPath(id *string) string {
	path := "/1/pkgs"
	if id != nil {
		path = path + "/" + *id
	}
	return path
}
