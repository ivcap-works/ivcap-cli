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
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	dockerclient "github.com/docker/docker/client"
	"github.com/google/go-containerregistry/pkg/name"
	v1 "github.com/google/go-containerregistry/pkg/v1"
	"github.com/google/go-containerregistry/pkg/v1/daemon"
	"github.com/google/go-containerregistry/pkg/v1/partial"
	"github.com/google/go-containerregistry/pkg/v1/remote"
	"github.com/google/go-containerregistry/pkg/v1/stream"
	"github.com/google/go-containerregistry/pkg/v1/types"
	"github.com/inhies/go-bytesize"
	log "go.uber.org/zap"

	"github.com/ivcap-works/ivcap-cli/pkg/adapter"
	genpkg "github.com/ivcap-works/ivcap-core-api/gen/package_"
	grpcpkgclient "github.com/ivcap-works/ivcap-core-api/grpc/package_/client"
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

func PushPackage(ctx context.Context, ra *adapter.GrpcAdapter, srcTagName string, forcePush, localImage bool, logger *log.Logger) error {
	srcTag, err := name.NewTag(srcTagName, name.WeakValidation, name.WithDefaultRegistry("local"))
	if err != nil {
		return fmt.Errorf("invalid src tag format: %w", err)
	}

	if srcTag.RegistryStr() == "local" || localImage {
		// check size
		client, err := dockerclient.NewClientWithOpts(dockerclient.FromEnv)
		if err != nil {
			return fmt.Errorf("failed to create docker client: %w", err)
		}
		inspect, err := client.ImageInspect(context.Background(), srcTag.String())
		if err != nil {
			return fmt.Errorf("failed to get inspect: %w", err)
		}
		if inspect.Size > 2*1024*1024*1024 {
			// fmt.Println("Image too large, please upload from a local docker registry, check README for how to do that.")
			// return nil
		}
	}

	fmt.Printf("\033[2K\r Pushing %s from %s, may take multiple minutes depending on the size of the image ...\n", srcTag.String(), srcTag.RegistryStr())

	var img v1.Image
	var cl v1.Layer
	// push from another repo registry
	if srcTag.RegistryStr() != "local" && !localImage {
		ref, err := name.ParseReference(srcTagName)
		if err != nil {
			return fmt.Errorf("parsing reference %q: %w", srcTagName, err)
		}

		desc, err := remote.Get(ref)
		if err != nil {
			return fmt.Errorf("failed to get %s, %w", srcTag, err)
		}
		img, err = desc.Image()
		if err != nil {
			return fmt.Errorf("failed to get image from description: %w", err)
		}
		config, err := img.RawConfigFile()
		if err != nil {
			return fmt.Errorf("failed to get image raw config: %w", err)
		}
		cl, err = partial.ConfigLayer(&withRawConfig{
			Raw: config,
		})
		if err != nil {
			return fmt.Errorf("failed to get config layer: %w", err)
		}

	} else {
		// load docker image
		ref, err := name.ParseReference(srcTag.String())
		if err != nil {
			return fmt.Errorf("failed to parse name reference: %s, %w", srcTag.String(), err)
		}
		img, err = daemon.Image(ref)
		if err != nil {
			return fmt.Errorf("reading image %q: %w", ref, err)
		}
		cl, err = partial.ConfigLayer(img)
		if err != nil {
			return fmt.Errorf("failed to get config layer: %w", err)
		}
	}

	layers, err := img.Layers()
	if err != nil {
		return fmt.Errorf("failed to get image layers: %w", err)
	}
	layers = append(layers, cl)

	fmt.Printf("start pushing layers\n")

	// send layers
	for _, layer := range layers {
		digest, err := layer.Digest()
		if err != nil {
			return fmt.Errorf("failed to get layer digest: %w", err)
		}
		total, err := layer.Size()
		if err != nil {
			return fmt.Errorf("failed to get layer size: %w", err)
		}

		fmt.Printf("start pushing layer: %s, %d\n", digest.Hex[:10], total)

		if err := pushLayer(ctx, ra, layer, srcTag, forcePush); err != nil {
			return fmt.Errorf("failed to push layer: %w", err)
		}
	}

	// send the image manifest
	manifest, err := img.Manifest()
	if err != nil {
		return fmt.Errorf("failed to get image manifest: %w", err)
	}
	if manifest.MediaType == types.OCIManifestSchema1 {
		manifest.MediaType = types.DockerManifestSchema2
	}

	data, err := json.Marshal(manifest)
	if err != nil {
		return fmt.Errorf("failed to marshal manifest: %w", err)
	}

	return pushManifest(ctx, data, ra, srcTag, forcePush, logger)
}

// pushLayer push&stream config/layer from v1.Layer
func pushLayer(ctx context.Context, ra *adapter.GrpcAdapter, layer v1.Layer, srcTag name.Tag, forcePush bool) error {
	var layerType string
	mediaType, err := layer.MediaType()
	if err != nil {
		return fmt.Errorf("failed to get media type: %w", err)
	}
	if mediaType == types.OCIConfigJSON {
		layerType = "config"
	} else {
		layerType = "layer"
	}

	digest, err := layer.Digest()
	if err != nil {
		return fmt.Errorf("failed to get layer digest: %w", err)
	}

	rc, err := layer.Compressed()
	if err != nil {
		return fmt.Errorf("failed to get compressed data for layer %s: %w", digest.Hex[:10], err)
	}

	total, err := layer.Size()
	if err != nil {
		return fmt.Errorf("failed to get layer size: %w", err)
	}

	return doImagePush(ctx, ra, &localPushRequest{
		rc:        rc,
		layerType: layerType,
		srcTag:    srcTag,
		digest:    digest,
		forcePush: forcePush,
		total:     total,
	})
}

func pushManifest(ctx context.Context, manifest []byte, ra *adapter.GrpcAdapter, srcTag name.Tag, forcePush bool, logger *log.Logger) error {
	digest, _, err := v1.SHA256(bytes.NewReader(manifest))
	if err != nil {
		return fmt.Errorf("failed to get img digest: %w", err)
	}

	fmt.Printf("\033[2K\r %s pushing ...", srcTag.String())

	return doImagePush(ctx, ra, &localPushRequest{
		rc:        io.NopCloser(bytes.NewReader(manifest)),
		layerType: "manifest",
		srcTag:    srcTag,
		forcePush: forcePush,
		digest:    digest,
		total:     int64(len(manifest)),
	})
}

type localPushRequest struct {
	rc        io.ReadCloser // readCloser input
	layerType string
	srcTag    name.Tag
	forcePush bool
	digest    v1.Hash
	total     int64
}

func doImagePush(ctx context.Context, ra *adapter.GrpcAdapter, req *localPushRequest) error {
	gclient := grpcpkgclient.NewClient(ra.Cc)
	s, err := gclient.Push()(ctx, &genpkg.PushPayload{
		Force:  &req.forcePush,
		Tag:    req.srcTag.RepositoryStr() + ":" + req.srcTag.TagStr(),
		Type:   req.layerType,
		Digest: req.digest.String(),
		JWT:    ra.Cctx.AccessToken,
	})
	if err != nil {
		return fmt.Errorf("failed to push image: %w", err)
	}

	strm, ok := s.(*grpcpkgclient.PushClientStream)
	if !ok {
		return fmt.Errorf("invalid pull client stream type")
	}

	defer req.rc.Close()

	const chunkSize = 512 * 1024
	chunk := make([]byte, chunkSize)
	var rc *genpkg.PushResult
	for {
		n, err := req.rc.Read(chunk)
		if err != nil {
			if !errors.Is(err, io.EOF) && !errors.Is(err, io.ErrUnexpectedEOF) {
				return fmt.Errorf("streaming layer %s:%s error, failed to read from layer: %w", req.layerType, req.digest.Hex[:10], err)
			}
		}
		if n > 0 {
			if err = strm.SendWithContext(ctx, &genpkg.DockerImageChunk{
				Data: chunk[:n],
			}); err != nil && !errors.Is(err, io.EOF) { // not reach the end of layer reader
				return fmt.Errorf("failed to streaming layer %s:%s, %w", req.layerType, req.digest.Hex[:10], err)
			}
		}

		if errors.Is(err, io.EOF) || errors.Is(err, io.ErrUnexpectedEOF) {
			break
		}
	}

	if rc, err = strm.CloseAndRecvWithContext(ctx); err != nil {
		return fmt.Errorf("failed to receive push result: %w", err)
	}

	if req.layerType != "manifest" {
		fmt.Printf("\033[2K\r %s %12s uploaded\n", req.digest.Hex[:10], bytesize.New(float64(req.total)))
	} else {
		fmt.Printf("\033[2K\r %s pushed\n", rc.Digest)
	}

	return nil
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

type pullClientStreamDataHandler func(strm *grpcpkgclient.PullClientStream, logger *log.Logger) error

func PullPackage(ctxt context.Context, tag string, ra *adapter.GrpcAdapter, logger *log.Logger) error {
	srcTag, err := name.NewTag(tag, name.WeakValidation)
	if err != nil {
		return fmt.Errorf("invalid src tag format: %w", err)
	}
	tag = srcTag.String()

	// the image to store
	img := &image{}

	// pull the image config
	configHander := func(strm *grpcpkgclient.PullClientStream, logger *log.Logger) error {
		var buf bytes.Buffer
		for {
			chunk, err := strm.Recv()
			if err != nil && !errors.Is(err, io.EOF) {
				return err
			}
			if chunk != nil && len(chunk.Data) > 0 {
				if _, e := buf.Write(chunk.Data); e != nil {
					return e
				}
			}
			if errors.Is(err, io.EOF) {
				break
			}
		}
		img.RawC = buf.Bytes()

		fmt.Printf("\033[2K\r Writing image %s ...", tag)
		dockerImage, err := partial.CompressedToImage(img)
		if err != nil {
			return fmt.Errorf("failed to convert compressed image: %w", err)
		}
		if _, err = daemon.Write(srcTag, dockerImage); err != nil {
			return fmt.Errorf("failed to write image: %w", err)
		}
		fmt.Printf("\033[2K\r %s image pulled \n", tag)

		return nil
	}

	var layers []v1.Layer
	manifestHandler := func(strm *grpcpkgclient.PullClientStream, logger *log.Logger) error {
		var buf bytes.Buffer
		for {
			chunk, err := strm.Recv()
			if err != nil && !errors.Is(err, io.EOF) {
				return err
			}
			if chunk != nil && len(chunk.Data) > 0 {
				if _, e := buf.Write(chunk.Data); e != nil {
					return e
				}
			}

			if errors.Is(err, io.EOF) {
				break
			}
		}
		// read manifest
		img.RawM = buf.Bytes()

		manifest, err := v1.ParseManifest(bytes.NewReader(img.RawM))
		if err != nil {
			return fmt.Errorf("failed to parse manifest %s: %w", string(img.RawM), err)
		}

		// get layers in reverse order
		manifestLayers := len(manifest.Layers)
		for i := len(manifest.Layers) - 1; i >= 0; i-- {
			layerDesc := manifest.Layers[i]
			ref := strings.TrimSuffix(tag, ":"+srcTag.TagStr()) + "@" + layerDesc.Digest.String()

			layer, err := pullLayer(ctxt, ref, layerDesc, ra, logger)
			if err != nil {
				return fmt.Errorf("failed to retrieve full layer: %w", err)
			}

			layers = append(layers, layer)

			// pull config
			if len(layers) == manifestLayers {
				if err = pullConfig(ctxt, tag, ra, configHander, logger); err != nil {
					return err
				}
			}
		}
		return nil
	}

	return pullManifest(ctxt, tag, ra, manifestHandler, logger)
}

type pullStreamReadCloser struct {
	strm *grpcpkgclient.PullClientStream
}

func (s *pullStreamReadCloser) Close() error {
	// server will do close
	return nil
}

func (s *pullStreamReadCloser) Read(p []byte) (n int, err error) {
	chunk, err := s.strm.Recv()
	if err != nil {
		return 0, fmt.Errorf("failed to receive from strm: %w", err)
	}
	if chunk != nil && len(chunk.Data) > 0 {
		return copy(p, chunk.Data), nil
	}
	return 0, nil
}

func pullLayer(ctx context.Context, ref string, layerDesc v1.Descriptor, ra *adapter.GrpcAdapter, logger *log.Logger) (layer v1.Layer, err error) {
	gclient := grpcpkgclient.NewClient(ra.Cc)
	strm, err := gclient.Pull()(ctx, &genpkg.PullPayload{
		Type: "layer",
		Ref:  ref,
		JWT:  ra.Cctx.AccessToken,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to pull image config: %w", err)
	}

	s, ok := strm.(*grpcpkgclient.PullClientStream)
	if !ok {
		return nil, fmt.Errorf("invalid pull client stream type")
	}

	fmt.Printf("\033[2K\r %s %d out of %s ...", layerDesc.Digest.Hex[:10], 0, bytesize.New(float64(layerDesc.Size)))

	return stream.NewLayer(
		&pullStreamReadCloser{
			strm: s,
		},
		stream.WithMediaType(types.DockerLayer),
	), nil
}

func pullConfig(ctx context.Context, tag string, ra *adapter.GrpcAdapter, configHandler pullClientStreamDataHandler, logger *log.Logger) error {
	gclient := grpcpkgclient.NewClient(ra.Cc)
	strm, err := gclient.Pull()(ctx, &genpkg.PullPayload{
		Type: "config",
		Ref:  tag,
		JWT:  ra.Cctx.AccessToken,
	})
	if err != nil {
		return fmt.Errorf("failed to pull image config: %w", err)
	}

	s, ok := strm.(*grpcpkgclient.PullClientStream)
	if !ok {
		return fmt.Errorf("invalid pull client stream type")
	}

	return configHandler(s, logger)
}

func pullManifest(ctx context.Context, tag string, ra *adapter.GrpcAdapter, manifestHandler pullClientStreamDataHandler, logger *log.Logger) error {
	fmt.Printf("\033[2K\r Pulling from %s, may take multiple minutes depending on the size of the image ...\n", tag)
	// pull the image manifest
	gclient := grpcpkgclient.NewClient(ra.Cc)
	strm, err := gclient.Pull()(ctx, &genpkg.PullPayload{
		Type: "manifest",
		Ref:  tag,
		JWT:  ra.Cctx.AccessToken,
	})
	if err != nil {
		return fmt.Errorf("failed to pull image manifest: %w", err)
	}

	s, ok := strm.(*grpcpkgclient.PullClientStream)
	if !ok {
		return fmt.Errorf("invalid pull client stream type")
	}

	return manifestHandler(s, logger)
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
