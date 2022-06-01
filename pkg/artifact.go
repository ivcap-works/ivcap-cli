package client

import (
	"context"
	"encoding/base64"
	"fmt"
	_ "fmt"
	"io"
	"net/url"
	"strconv"
	"strings"

	api "cayp/api_gateway/gen/http/artifact/client"

	"github.com/reinventingscience/ivcap-client/pkg/adapter"

	log "go.uber.org/zap"
)

/**** LIST ****/

type ListArtifactRequest struct {
	Offset int
	Limit  int
}

func ListArtifacts(ctxt context.Context, cmd *ListArtifactRequest, adpt *adapter.Adapter, logger *log.Logger) (*api.ListResponseBody, error) {
	pyl, err := ListArtifactsRaw(ctxt, cmd, adpt, logger)
	if err != nil {
		return nil, err
	}
	var list api.ListResponseBody
	pyl.AsType(&list)
	return &list, nil
}

func ListArtifactsRaw(ctxt context.Context, cmd *ListArtifactRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	path := artifactPath(nil, adpt)

	pa := []string{}
	if cmd.Offset > 0 {
		pa = append(pa, "offset="+url.QueryEscape(strconv.Itoa(cmd.Offset)))
	}
	if cmd.Limit > 0 {
		pa = append(pa, "limit="+url.QueryEscape(strconv.Itoa(cmd.Limit)))
	}
	if len(pa) > 0 {
		path = path + "?" + strings.Join(pa, "&")
	}
	//fmt.Printf("PATH: %s\n", path)
	return (*adpt).Get(ctxt, path, logger)
}

// /**** CREATE ****/

type CreateArtifactRequest struct {
	Name       string            `json:"name"`
	Collection string            `json:"collection"`
	Meta       map[string]string `json:"meta"`
}

func CreateArtifact(ctxt context.Context, cmd *CreateArtifactRequest, reader io.Reader, adpt *adapter.Adapter, logger *log.Logger) (*api.UploadResponseBody, error) {
	if res, err := CreateArtifactRaw(ctxt, cmd, reader, adpt, logger); err == nil {
		var artifact api.UploadResponseBody
		if err := res.AsType(&artifact); err != nil {
			return nil, err
		}
		return &artifact, nil
	} else {
		return nil, err
	}
}

func CreateArtifactRaw(ctxt context.Context, cmd *CreateArtifactRequest, reader io.Reader, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	path := artifactPath(nil, adpt)
	headers := make(map[string]string)
	if cmd.Name != "" {
		headers["X-Name"] = cmd.Name
	}
	if cmd.Collection != "" {
		headers["X-Collection"] = cmd.Name
	}
	var meta []string
	for key, value := range cmd.Meta {
		k := base64.StdEncoding.EncodeToString([]byte(key))
		v := base64.StdEncoding.EncodeToString([]byte(value))
		meta = append(meta, fmt.Sprintf("%s %s", k, v))
	}
	if len(meta) > 0 {
		headers["Upload-Metadata"] = cmd.Name
	}
	return (*adpt).Post(ctxt, path, reader, &headers, logger)
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

/**** UTILS ****/

func artifactPath(id *string, adpt *adapter.Adapter) string {
	path := "/1/artifacts"
	if id != nil {
		path = path + "/" + *id
	}
	return path
}
