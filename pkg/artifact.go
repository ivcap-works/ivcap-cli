package client

import (
	"context"
	_ "fmt"
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

// type CreateArtifactRequest struct {
// 	Id   string `json:"id"`
// 	Name string `json:"name"`
// }

// func CreateArtifactRaw(ctxt context.Context, cmd *CreateArtifactRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
// 	if (*cmd).Id == "" {
// 		(*cmd).Id = uuid.New().String()
// 	}

// 	body, err := json.MarshalIndent(*cmd, "", "  ")
// 	if err != nil {
// 		logger.Error("error marshalling body.", log.Error(err))
// 		return nil, err
// 	}
// 	// fmt.Printf("RECORD %+v - %s\n", cmd, body)

// 	path := artifactPath(nil, adpt)
// 	return (*adpt).Post(ctxt, path, bytes.NewReader(body), logger)
// }

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
