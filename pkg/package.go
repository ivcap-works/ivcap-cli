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
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/url"
	"strings"

	dockerimage "github.com/docker/docker/api/types/image"
	dockerregistry "github.com/docker/docker/api/types/registry"
	dockerclient "github.com/docker/docker/client"
	"github.com/google/go-containerregistry/pkg/name"
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

func PushPackage(ctx context.Context, srcTagName string, forcePush, localImage bool, adpt adapter.Adapter, logger *log.Logger) (*api.PushResponseBody, error) {
	srcTag, err := name.NewTag(srcTagName, name.WeakValidation, name.WithDefaultRegistry("local"))
	if err != nil {
		return nil, fmt.Errorf("invalid src tag format: %w", err)
	}

	client, err := dockerclient.NewClientWithOpts(
		dockerclient.WithAPIVersionNegotiation(),
		dockerclient.FromEnv,
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	fmt.Printf("\033[2K\r Pushing %s from %s, may take multiple minutes depending on the size of the image ...\n", srcTag.String(), srcTag.RegistryStr())

	registrySrvHost, err := getDockerRegistryHost(adpt)
	if err != nil {
		return nil, err
	}

	targetImage := registrySrvHost + "/docker-registry/" + srcTagName
	if err := client.ImageTag(ctx, srcTagName, targetImage); err != nil {
		return nil, fmt.Errorf("failed to tag image: %w", err)
	}

	// Encode bearer token as Docker AuthConfig
	encodedAuth, err := getDockerRegistryAuth(registrySrvHost, adpt)
	if err != nil {
		return nil, err
	}

	pushResp, err := client.ImagePush(context.Background(), targetImage, dockerimage.PushOptions{
		RegistryAuth: encodedAuth,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to push image: %w", err)
	}
	defer func() { _ = pushResp.Close() }()

	if err = checkPushResponse(pushResp, srcTagName); err != nil {
		fmt.Printf("\033[2K\r %s push failed, error: %s\n", srcTagName, err.Error())
	} else {
		fmt.Printf("\033[2K\r %s pushed\n", srcTagName)
	}

	return &api.PushResponseBody{
		Digest: &srcTagName,
	}, nil
}

func PullPackage(ctxt context.Context, tag string, adpt adapter.Adapter, logger *log.Logger) error {
	srcTag, err := name.NewTag(tag, name.WeakValidation)
	if err != nil {
		return fmt.Errorf("invalid src tag format: %w", err)
	}
	tag = srcTag.String()

	client, err := dockerclient.NewClientWithOpts(
		dockerclient.WithAPIVersionNegotiation(),
		dockerclient.FromEnv,
	)
	if err != nil {
		return fmt.Errorf("failed to create docker client: %w", err)
	}

	fmt.Printf("\033[2K\r Pulling from %s, may take multiple minutes depending on the size of the image ...\n", tag)

	registrySrvHost, err := getDockerRegistryHost(adpt)
	if err != nil {
		return err
	}
	sourceImage := registrySrvHost + "/docker-registry/" + tag

	// Encode bearer token as Docker AuthConfig
	encodedAuth, err := getDockerRegistryAuth(registrySrvHost, adpt)
	if err != nil {
		return err
	}

	pullResp, err := client.ImagePull(context.Background(), sourceImage, dockerimage.PullOptions{
		RegistryAuth: encodedAuth,
	})
	if err != nil {
		return fmt.Errorf("failed to push image: %w", err)
	}
	defer func() { _ = pullResp.Close() }()

	if err = checkPullResponse(pullResp, tag); err != nil {
		fmt.Printf("\033[2K\r %s pull failed, error: %s\n", srcTag.TagStr(), err.Error())
	} else {
		fmt.Printf("\033[2K\r %s pulled\n", tag)
	}

	return nil

}

func getDockerRegistryHost(adpt adapter.Adapter) (string, error) {
	apiSrvAddr := adpt.GetConnectionContext().URL
	u, err := url.Parse(apiSrvAddr)
	if err != nil {
		return "", fmt.Errorf("failed to parse URL: %s, %w", apiSrvAddr, err)
	}

	// registry.develop.ivcap.net
	return "registry." + u.Host, nil
}

func getDockerRegistryAuth(registrySrvHost string, adpt adapter.Adapter) (string, error) {
	// Encode bearer token as Docker AuthConfig
	authConfig := dockerregistry.AuthConfig{
		Username:      "oauth2accesstoken", // required placeholder
		Password:      adpt.GetConnectionContext().AccessToken,
		ServerAddress: registrySrvHost,
	}

	// #nosec G117 - Password is a required identifier for the API endpoint
	authJSON, err := json.Marshal(authConfig)
	if err != nil {
		return "", fmt.Errorf("failed to marshal registry auth config: %w", err)
	}
	encodedJSON := base64.URLEncoding.EncodeToString(authJSON)
	return encodedJSON, nil
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

func checkPushResponse(pushResp io.Reader, digest string) error {
	for {
		data := make([]byte, 1024)
		if _, err := pushResp.Read(data); err != nil {
			if !errors.Is(err, io.EOF) {
				return fmt.Errorf("failed to read push response: %w", err)
			}
			break
		}
		output := string(bytes.ReplaceAll(data, []byte{0x00}, nil))
		switch {
		case strings.Contains(output, `"error":`):
			var errResult struct {
				Error       string          `json:"error,omitempty"`
				ErrorDetail json.RawMessage `json:"errorDetail,omitempty"`
			}
			if err := json.Unmarshal([]byte(output), &errResult); err == nil {
				return fmt.Errorf("failed to push :%s, detail:%s", strings.TrimSpace(errResult.Error), errResult.ErrorDetail)
			}
			return fmt.Errorf("failed to push : %s", output)
		default:
			var progress struct {
				Status         string          `json:"status,omitempty"`
				ProgressDetail json.RawMessage `json:"progressDetail,omitempty"`
				ID             string          `json:"id,omitempty"`
			}
			if err := json.Unmarshal([]byte(output), &progress); err == nil {
				if progress.Status == "Pushing" {
					var pushingDetail struct {
						Current float64 `json:"current,omitempty"`
						Total   float64 `json:"total,omitempty"`
					}
					if err := json.Unmarshal(progress.ProgressDetail, &pushingDetail); err == nil {
						fmt.Printf("\033[2K\r %10s %12s %12s%10s Pushing", digest, progress.ID, bytesize.New(pushingDetail.Current), bytesize.New(pushingDetail.Total))
					} else {
						fmt.Printf("\033[2K\r %10s %12s %10s", digest, progress.ID, progress.Status)
					}
				} else {
					fmt.Printf("\033[2K\r %10s %12s %10s", digest, progress.ID, progress.Status)
				}
			}
		}
	}

	return nil
}

func checkPullResponse(pullResp io.Reader, digest string) error {
	for {
		data := make([]byte, 1024)
		if _, err := pullResp.Read(data); err != nil {
			if !errors.Is(err, io.EOF) {
				return fmt.Errorf("failed to read pull response: %w", err)
			}
			break
		}
		output := string(bytes.ReplaceAll(data, []byte{0x00}, nil))
		switch {
		case strings.Contains(output, `"error":`):
			var errResult struct {
				Error       string          `json:"error,omitempty"`
				ErrorDetail json.RawMessage `json:"errorDetail,omitempty"`
			}
			if err := json.Unmarshal([]byte(output), &errResult); err == nil {
				return fmt.Errorf("failed to pull :%s, detail:%s", strings.TrimSpace(errResult.Error), errResult.ErrorDetail)
			}
			return fmt.Errorf("failed to pull : %s", output)
		default:
			var progress struct {
				Status         string          `json:"status,omitempty"`
				ProgressDetail json.RawMessage `json:"progressDetail,omitempty"`
			}
			if err := json.Unmarshal([]byte(output), &progress); err == nil {
				switch {
				case strings.Contains(progress.Status, "Downloading"), strings.Contains(progress.Status, "Extracting"):
					var pushingDetail struct {
						Current float64 `json:"current,omitempty"`
						Total   float64 `json:"total,omitempty"`
					}
					if err := json.Unmarshal(progress.ProgressDetail, &pushingDetail); err == nil {
						fmt.Printf("\033[2K\r %10s %12s%10s Downloading", digest, bytesize.New(pushingDetail.Current), bytesize.New(pushingDetail.Total))
					} else {
						fmt.Printf("\033[2K\r %10s Downloading", digest)
					}
				default:
					fmt.Printf("\033[2K\r %10s Pulling", digest)
				}
			}
		}
	}

	return nil
}
