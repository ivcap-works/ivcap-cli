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
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	dockerclient "github.com/docker/docker/client"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/tarball"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/inhies/go-bytesize"
	log "go.uber.org/zap"

	"github.com/ivcap-works/ivcap-cli/pkg/adapter"
	api "github.com/ivcap-works/ivcap-core-api/http/package_"
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

var _ partial.WithRawConfigFile = (*withRawConfig)(nil)

type withRawConfig struct {
	Raw []byte
}

func (w withRawConfig) RawConfigFile() ([]byte, error) {
	return w.Raw, nil
}

func PushServicePackage(srcTagName string, forcePush, localImage bool, adpt *adapter.Adapter, logger *log.Logger) (*api.PushResponseBody, error) {
	srcTag, err := name.NewTag(srcTagName, name.WeakValidation, name.WithDefaultRegistry("local"))
	if err != nil {
		return nil, fmt.Errorf("invalid src tag format: %w", err)
	}

	if srcTag.RegistryStr() == "local" || localImage {
		// check size
		client, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv)
		if err != nil {
			return nil, fmt.Errorf("failed to create docker client: %w", err)
		}
		inspect, _, err := client.ImageInspectWithRaw(context.Background(), srcTag.String())
		if err != nil {
			return nil, fmt.Errorf("failed to get inspect: %w", err)
		}
		if inspect.Size > 2*1024*1024*1024 {
			fmt.Println("Image too large, please upload from a local docker registry, check README for how to do that.")
			return nil, nil
		}
	}

	fmt.Printf("\033[2K\r Pushing %s from %s, may take multiple minutes depending on the size of the image ...\n", srcTag.String(), srcTag.RegistryStr())

	var img v1.Image
	var cl v1.Layer
	// push from another repo registry
	if srcTag.RegistryStr() != "local" {
		ref, err := name.ParseReference(srcTagName)
		if err != nil {
			return nil, fmt.Errorf("parsing reference %q: %w", srcTagName, err)
		}

		desc, err := remote.Get(ref)
		if err != nil {
			return nil, fmt.Errorf("failed to get %s, %w", srcTag, err)
		}
		img, err = desc.Image()
		if err != nil {
			return nil, fmt.Errorf("failed to get image from description: %w", err)
		}
		config, err := img.RawConfigFile()
		if err != nil {
			return nil, fmt.Errorf("failed to get image raw config: %w", err)
		}
		cl, err = partial.ConfigLayer(&withRawConfig{
			Raw: config,
		})
		if err != nil {
			return nil, fmt.Errorf("failed to get config layer: %w", err)
		}

	} else {
		// load docker image
		ref, err := name.ParseReference(srcTag.String())
		if err != nil {
			return nil, fmt.Errorf("failed to parse name reference: %s, %w", srcTag.String(), err)
		}
		img, err = daemon.Image(ref)
		if err != nil {
			return nil, fmt.Errorf("reading image %q: %w", ref, err)
		}
		cl, err = partial.ConfigLayer(img)
		if err != nil {
			return nil, fmt.Errorf("failed to get config layer: %w", err)
		}
	}

	layers, err := img.Layers()
	if err != nil {
		return nil, fmt.Errorf("failed to get image layers: %w", err)
	}
	layers = append(layers, cl)

	// send layers
	for _, layer := range layers {
		mediaType, err := layer.MediaType()
		if err != nil {
			return nil, fmt.Errorf("failed to get media type: %w", err)
		}
		if mediaType == types.OCIConfigJSON {
			if res, err := pushConfig(layer, adpt, srcTag, forcePush, logger); err != nil {
				return res, err
			}
		} else {
			if res, err := pushLayer(layer, adpt, srcTag, forcePush, logger); err != nil {
				return res, err
			}
		}
	}

	// send the image manifest
	manifest, err := img.RawManifest()
	if err != nil {
		return nil, fmt.Errorf("failed to get image manifest: %w", err)
	}

	return pushManifest(manifest, adpt, srcTag, forcePush, logger)
}

func pushConfig(layer v1.Layer, adpt *adapter.Adapter, srcTag name.Tag, forcePush bool, logger *log.Logger) (*api.PushResponseBody, error) {
	digest, err := layer.Digest()
	if err != nil {
		return nil, fmt.Errorf("failed to get layer digest: %w", err)
	}

	total, err := layer.Size()
	if err != nil {
		return nil, fmt.Errorf("failed to get layer size: %w", err)
	}

	path := pkgPath(nil) + "/push"
	q := url.Values{}
	q.Set("force", strconv.FormatBool(forcePush))
	q.Set("tag", srcTag.RepositoryStr()+":"+srcTag.TagStr())
	q.Set("total", strconv.FormatInt(int64(total), 10))
	q.Set("type", "config")
	q.Set("digest", digest.String())
	path += "?" + q.Encode()

	layerData, err := layer.Compressed()
	if err != nil {
		return nil, fmt.Errorf("failed to get compressed data for layer %s: %w", digest.Hex[:10], err)
	}

	res, err := (*adpt).Post(context.Background(), path, layerData, -1, nil, logger)
	if err != nil {
		// error type assertion with goa ???
		if strings.Contains(err.Error(), "already created") {
			return nil, fmt.Errorf("tag: %s already created, use -f to force overwrite", srcTag)
		}
		return nil, fmt.Errorf("failed to push layer %s, %s, error: %w", digest.Hex[:10], bytesize.New(float64(total)), err)
	}

	var body api.PushResponseBody
	if err := res.AsType(&body); err != nil {
		return nil, fmt.Errorf("failed to decode update service response body; %w", err)
	}

	fmt.Printf("\033[2K\r %s %12s uploaded\n", digest.Hex[:10], bytesize.New(float64(total)))

	return &body, nil
}

func pushLayer(layer v1.Layer, adpt *adapter.Adapter, srcTag name.Tag, forcePush bool, logger *log.Logger) (*api.PushResponseBody, error) {
	digest, err := layer.Digest()
	if err != nil {
		return nil, fmt.Errorf("failed to get layer digest: %w", err)
	}

	total, err := layer.Size()
	if err != nil {
		return nil, fmt.Errorf("failed to get layer size: %w", err)
	}

	path := pkgPath(nil) + "/push"
	q := url.Values{}
	q.Set("force", strconv.FormatBool(forcePush))
	q.Set("tag", srcTag.RepositoryStr()+":"+srcTag.TagStr())
	q.Set("type", "layer")
	q.Set("digest", digest.String())
	postPath := path + "?" + q.Encode()

	// do an inital post
	fmt.Printf("\033[2K\r %s %10s uploading...", digest.Hex[:10], bytesize.New(float64(total)))
	ctxt, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()

	res, err := (*adpt).Post(ctxt, postPath, bytes.NewReader([]byte{}), -1, nil, logger)
	if err != nil {
		if strings.Contains(err.Error(), "already created") {
			return nil, fmt.Errorf("tag: %s already created, use -f to force overwrite", srcTag)
		}
		return nil, fmt.Errorf("failed to push layer %s, %s, error: %w", digest.Hex[:10], bytesize.New(float64(total)), err)
	}
	var body api.PushResponseBody
	if err = res.AsType(&body); err != nil {
		return nil, fmt.Errorf("failed to decode push layer response body; %w", err)
	}
	if body.Mounted != nil && *body.Mounted { // already exists
		fmt.Printf("\033[2K\r %s %10s already exits\n", digest.Hex[:10], bytesize.New(float64(total)))
		return &body, nil
	}

	if body.Location == nil || *body.Location == "" {
		return nil, fmt.Errorf("expecting locaton response from push")
	}

	layerData, err := layer.Compressed()
	if err != nil {
		return nil, fmt.Errorf("failed to get compressed data for layer %s: %w", digest.Hex[:10], err)
	}

	location := *body.Location
	chunkSize := 10 * 1024 * 1024 // 10MB
	buffer := make([]byte, chunkSize)
	start, end := 0, 0
	for {
		q := url.Values{}
		q.Set("tag", srcTag.RepositoryStr()+":"+srcTag.TagStr())
		q.Set("digest", digest.String())
		q.Set("total", strconv.FormatInt(int64(total), 10))
		q.Set("location", location)

		n, err := io.ReadFull(layerData, buffer)
		if err != nil && !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
			return nil, fmt.Errorf("failed to read layer data: %w", err)
		}
		if n == 0 {
			break
		}
		end = start + n

		q.Set("start", strconv.FormatInt(int64(start), 10))
		q.Set("end", strconv.FormatInt(int64(end), 10))
		patchPath := pkgPath(nil) + "/blob"
		patchPath += "?" + q.Encode()

		fmt.Printf("\033[2K\r %s %10s%10s%10s uploading...", digest.Hex[:10], bytesize.New(float64(end)), "out of", bytesize.New(float64(total)))
		ctxt, cancel := context.WithTimeout(context.Background(), 60*time.Second)
		defer cancel()

		res, err := (*adpt).Patch(ctxt, patchPath, bytes.NewReader(buffer[:n]), -1, nil, logger)
		if err != nil {
			if strings.Contains(err.Error(), "already created") {
				return nil, fmt.Errorf("tag: %s already created, use -f to force overwrite", srcTag)
			}
			return nil, fmt.Errorf("failed to patch layer %s, %s, error: %w", digest.Hex[:10], bytesize.New(float64(total)), err)
		}

		var body api.PatchResponseBody
		if err = res.AsType(&body); err != nil {
			return nil, fmt.Errorf("failed to decode push layer response body; %w", err)
		}

		if body.Location == nil || *body.Location == "" {
			return nil, fmt.Errorf("expecting location from patch response")
		}
		location = *body.Location

		// step forward
		start += n
	}

	// commit
	q = url.Values{}
	q.Set("tag", srcTag.RepositoryStr()+":"+srcTag.TagStr())
	q.Set("digest", digest.String())
	q.Set("location", location)
	putPath := pkgPath(nil) + "/blob"
	putPath += "?" + q.Encode()

	ctxt, cancel = context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	if _, err = (*adpt).Put(ctxt, putPath, bytes.NewReader([]byte{}), -1, nil, logger); err != nil {
		return nil, fmt.Errorf("failed to commit layer %s, %s, error: %w", digest.Hex[:10], bytesize.New(float64(total)), err)
	}
	fmt.Printf("\033[2K\r %s %10s uploaded\n", digest.Hex[:10], bytesize.New(float64(total)))

	d := digest.String()
	return &api.PushResponseBody{
		Digest: &d,
	}, nil
}

func pushManifest(manifest []byte, adpt *adapter.Adapter, srcTag name.Tag, forcePush bool, logger *log.Logger) (*api.PushResponseBody, error) {
	digest, _, err := v1.SHA256(bytes.NewReader(manifest))
	if err != nil {
		return nil, fmt.Errorf("failed to get img digest: %w", err)
	}

	fmt.Printf("\033[2K\r %s pushing ...", srcTag.String())

	path := pkgPath(nil) + "/push"
	q := url.Values{}
	q.Set("force", strconv.FormatBool(forcePush))
	q.Set("tag", srcTag.RepositoryStr()+":"+srcTag.TagStr())
	q.Set("type", "manifest")
	q.Set("digest", digest.String())
	postPath := path + "?" + q.Encode()

	ctxt, cancel := context.WithTimeout(context.Background(), 10*time.Minute)
	defer cancel()
	res, err := (*adpt).Post(ctxt, postPath, bytes.NewReader(manifest), -1, nil, logger)
	if err != nil {
		// error type assertion with goa ???
		if strings.Contains(err.Error(), "already created") {
			return nil, fmt.Errorf("tag: %s already created, use -f to force overwrite", srcTag)
		}
		return nil, fmt.Errorf("failed to push service package: %w", err)
	}
	var body api.PushResponseBody
	if err := res.AsType(&body); err != nil {
		return nil, fmt.Errorf("failed to decode update service response body; %w", err)
	}
	if body.Digest != nil {
		fmt.Printf("\033[2K\r %s pushed\n", *body.Digest)
	}

	return &body, nil
}

var _ partial.CompressedImageCore = (*image)(nil)

type image struct {
	RawC []byte
	RawM []byte
	Ls   []v1.Layer
}

func (i *image) RawManifest() ([]byte, error) {
	return i.RawM, nil
}
func (i *image) MediaType() (types.MediaType, error) {
	return types.DockerManifestSchema2, nil
}
func (i *image) RawConfigFile() ([]byte, error) {
	return i.RawC, nil
}
func (i *image) LayerByDigest(h v1.Hash) (partial.CompressedLayer, error) {
	for _, layer := range i.Ls {
		d, err := layer.Digest()
		if err != nil {
			return nil, err
		}
		if h == d {
			return layer, nil
		}
	}
	return nil, fmt.Errorf("blob %v not found", h)
}

var _ partial.CompressedLayer = (*imageLayer)(nil)

type imageLayer struct {
	Data []byte // compressed data
	Hash v1.Hash
}

func (l *imageLayer) Digest() (v1.Hash, error) {
	return l.Hash, nil
}

func (l *imageLayer) Compressed() (io.ReadCloser, error) {
	return io.NopCloser(bytes.NewReader(l.Data)), nil
}

func (l *imageLayer) Size() (int64, error) {
	return int64(len(l.Data)), nil
}

func (l *imageLayer) MediaType() (types.MediaType, error) {
	return types.DockerLayer, nil
}

func PullPackage(ctxt context.Context, tag string, adpt *adapter.Adapter, logger *log.Logger) error {
	srcTag, err := name.NewTag(tag, name.WeakValidation)
	if err != nil {
		return fmt.Errorf("invalid src tag format: %w", err)
	}
	tag = srcTag.String()

	// the image to store
	img := &image{}

	// pull the image config
	configHander := func(resp *http.Response, path string, logger *log.Logger) error {
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

		// no copy, don't change data afterwards
		img.RawC = data

		fmt.Printf("\033[2K\r Writing image %s ...", tag)
		dockerImage, err := partial.CompressedToImage(img)
		if err != nil {
			return fmt.Errorf("failed to convert compressed image: %w", err)
		}
		if _, err = daemon.Write(srcTag, dockerImage); err != nil {
			return fmt.Errorf("failed to write image: %w", err)
		}
		fmt.Printf("\033[2K\r %s image pulled \n", tag)

		// clean the temp file if any
		m, err := dockerImage.Manifest()
		if err != nil {
			return fmt.Errorf("failed to get manifest: %w", err)
		}
		for _, l := range m.Layers {
			ref := strings.TrimSuffix(tag, ":"+srcTag.TagStr()) + "@" + l.Digest.String()
			filePath := filepath.Clean(filepath.Join(os.TempDir(), ref))
			if _, err := os.Stat(filePath); err != os.ErrNotExist {
				_ = os.Remove(filePath)
			}
		}

		return nil
	}

	var layers []v1.Layer
	manifestHandler := func(resp *http.Response, path string, logger *log.Logger) error {
		if resp.StatusCode != 200 {
			data, err := io.ReadAll(resp.Body)
			if err != nil {
				return fmt.Errorf("failed to read res body: %w", err)
			}
			return fmt.Errorf("statusCode: %d, error: %s", resp.StatusCode, string(data))
		}

		// read manifest
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to get data from manifest response: %w", err)
		}

		img.RawM = data

		manifest, err := v1.ParseManifest(bytes.NewReader(img.RawM))
		if err != nil {
			return fmt.Errorf("failed to parse manifest %s: %w", string(img.RawM), err)
		}

		// get layers in reverse order
		manifestLayers := len(manifest.Layers)
		for i := len(manifest.Layers) - 1; i >= 0; i-- {
			layerDesc := manifest.Layers[i]
			ref := strings.TrimSuffix(tag, ":"+srcTag.TagStr()) + "@" + layerDesc.Digest.String()

			layer, err := retreiveFullLayer(ref, layerDesc, adpt, logger)
			if err != nil {
				return fmt.Errorf("failed to retrieve full layer: %w", err)
			}

			layers = append(layers, layer)

			// pull config
			if len(layers) == manifestLayers {
				if err = pullConfig(tag, adpt, configHander, logger); err != nil {
					return err
				}
			}
		}
		return nil
	}

	return pullManifest(tag, adpt, manifestHandler, logger)
}

func pullLayerWithOffset(ref string, offset int, adpt *adapter.Adapter, layerHandler adapter.ResponseHandler, logger *log.Logger) error {
	lpath := pkgPath(nil) + "/pull"
	q := url.Values{
		"type":   []string{"layer"},
		"ref":    []string{ref},
		"offset": []string{strconv.Itoa(offset)},
	}

	lpath += "?" + q.Encode()

	if err := (*adpt).GetWithHandler(context.Background(), lpath, nil, layerHandler, logger); err != nil {
		return fmt.Errorf("failed to get layer: %w", err)
	}

	return nil
}

func retreiveFullLayer(ref string, layerDesc v1.Descriptor, adpt *adapter.Adapter, logger *log.Logger) (layer v1.Layer, err error) {
	layerOffset := 0
	// temp file to start layer data
	filePath := filepath.Clean(filepath.Join(os.TempDir(), ref))

	fmt.Printf("\033[2K\r %s %d out of %s ...", layerDesc.Digest.Hex[:10], 0, bytesize.New(float64(layerDesc.Size)))

	layerWithOffsetHandler := func(resp *http.Response, path string, logger *log.Logger) error {
		data, err := io.ReadAll(resp.Body)
		if err != nil {
			return fmt.Errorf("failed to read res body: %w", err)
		}

		if layerOffset == 0 {
			if err = os.MkdirAll(filepath.Dir(filePath), 0750); err != nil {
				return fmt.Errorf("failed to create path: %s, error: %w", filepath.Dir(filePath), err)
			}
			// file need to be truncated
			if _, err = os.Create(filePath); err != nil {
				return fmt.Errorf("failed to create file: %s, error: %w", filePath, err)
			}
		}

		if len(data) == 0 {
			return nil
		}

		// append to file
		f, err := os.OpenFile(filePath, os.O_APPEND|os.O_WRONLY, 0600)
		if err != nil {
			return fmt.Errorf("failed to open file: %s for write, error: %w", filepath.Dir(filePath), err)
		}
		defer func() {
			if err = f.Close(); err != nil && !errors.Is(err, os.ErrClosed) {
				fmt.Printf("file %s close error : %v\n", filePath, err)
			}
		}()
		if _, err = f.Write(data); err != nil {
			return fmt.Errorf("failed to copy to file: %s, error: %w", filePath, err)
		}

		layerOffset += len(data)

		fmt.Printf("\033[2K\r %s %s out of %s", layerDesc.Digest.Hex[:10], bytesize.New(float64(layerOffset)), bytesize.New(float64(layerDesc.Size)))

		return nil
	}

	retries := 0
	const maxRetries = 4

	for layerOffset < int(layerDesc.Size) {
		prev := layerOffset
		if err = pullLayerWithOffset(ref, layerOffset, adpt, layerWithOffsetHandler, logger); err != nil {
			return nil, err
		}
		if layerOffset == prev { // wait to catch up
			retries++
			time.Sleep(10 * time.Second)
		} else {
			retries = 0
		}

		if retries > maxRetries {
			return nil, fmt.Errorf("max retries which got 0 bytes happend")
		}
	}

	layer, err = tarball.LayerFromFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to create layer from path: %s, error: %w", filePath, err)
	}

	fmt.Printf("\033[2K\r %s %s pulled\n", layerDesc.Digest.Hex[:10], bytesize.New(float64(layerDesc.Size)))

	return layer, err
}

func pullConfig(tag string, adpt *adapter.Adapter, configHandler adapter.ResponseHandler, logger *log.Logger) error {
	cpath := pkgPath(nil) + "/pull"
	q := url.Values{
		"type": []string{"config"},
		"ref":  []string{tag},
	}
	cpath += "?" + q.Encode()

	fmt.Printf("\033[2K\r Pulling config from %s ...", tag)

	if err := (*adpt).GetWithHandler(context.Background(), cpath, nil, configHandler, logger); err != nil {
		return fmt.Errorf("failed to get config file: %w", err)
	}
	return nil
}

func pullManifest(tag string, adpt *adapter.Adapter, manifestHandler adapter.ResponseHandler, logger *log.Logger) error {
	// pull the image manifest
	mpath := pkgPath(nil) + "/pull"
	q := url.Values{
		"ref":  []string{tag},
		"type": []string{"manifest"},
	}
	mpath += "?" + q.Encode()

	fmt.Printf("\033[2K\r Pulling from %s, may take multiple minutes depending on the size of the image ...\n", tag)

	if err := (*adpt).GetWithHandler(context.Background(), mpath, nil, manifestHandler, logger); err != nil {
		return fmt.Errorf("failed to get manifest: %s, %w", mpath, err)
	}
	return nil
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
	path := "/1/packages"
	if id != nil {
		path = path + "/" + *id
	}
	return path
}
