package client

import (
	"bytes"
	"context"
	"encoding/json"
	_ "fmt"
	"net/url"
	"strconv"
	"strings"

	api "github.com/reinventingscience/ivcap-core-api/http/service"

	"github.com/reinventingscience/ivcap-client/pkg/adapter"

	log "go.uber.org/zap"
)

/**** LIST ****/

type ListServiceRequest struct {
	Offset int
	Limit  int
}

// type ListServiceResult struct {
// 	HasMore       bool   `json:"hasMore"`
// 	NextPageToken string `json:"nextPageToken"`
// 	Records       []struct {
// 		Aspects   map[string]interface{} `json:"aspects"`
// 		ID        string                 `json:"id"`
// 		Name      string                 `json:"name"`
// 		SourceTag string                 `json:"sourceTag"`
// 		TenantID  int                    `json:"tenantId"`
// 	} `json:"records"`
// }

func ListServices(ctxt context.Context, cmd *ListServiceRequest, adpt *adapter.Adapter, logger *log.Logger) (*api.ListResponseBody, error) {
	pyl, err := ListServicesRaw(ctxt, cmd, adpt, logger)
	if err != nil {
		return nil, err
	}
	var list api.ListResponseBody
	pyl.AsType(&list)
	return &list, nil
}

func ListServicesRaw(ctxt context.Context, cmd *ListServiceRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	path := servicePath(nil, adpt)

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

// func (t *QueryTerm) asUrlQuery() string {
// 	if t.urlQ != "" {
// 		return t.urlQ
// 	}
// 	v := fmt.Sprint(t.Value)
// 	v = strings.ReplaceAll(v, ":", "%3A") // ':' is the separation character, so it needs to be escaped
// 	op := t.Op
// 	if op == "=" {
// 		op = ""
// 	}
// 	q := fmt.Sprintf("%s:%s%s", t.Path, op, v)
// 	return url.QueryEscape(q)
// }

/**** CREATE ****/

// type CreateServiceRequest struct {
// 	Id   string `json:"id"`
// 	Name string `json:"name"`
// }

func CreateServiceRaw(ctxt context.Context, cmd *api.CreateRequestBody, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	body, err := json.MarshalIndent(*cmd, "", "  ")
	if err != nil {
		logger.Error("error marshalling body.", log.Error(err))
		return nil, err
	}
	// fmt.Printf("RECORD %+v - %s\n", cmd, body)

	path := servicePath(nil, adpt)
	return (*adpt).Post(ctxt, path, bytes.NewReader(body), int64(len(body)), nil, logger)
}

/**** UPDATE ****/

// type CreateServiceRequest struct {
// 	Id   string `json:"id"`
// 	Name string `json:"name"`
// }

func UpdateServiceRaw(ctxt context.Context, id string, createAnyway bool, cmd *api.UpdateRequestBody, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	body, err := json.MarshalIndent(*cmd, "", "  ")
	if err != nil {
		logger.Error("error marshalling body.", log.Error(err))
		return nil, err
	}
	// fmt.Printf("RECORD %+v - %s\n", cmd, body)

	path := servicePath(&id, adpt)
	if createAnyway {
		path += "?force-create=true"
	}
	return (*adpt).Put(ctxt, path, bytes.NewReader(body), int64(len(body)), nil, logger)
}

/**** READ ****/

type ReadServiceRequest struct {
	Id string
}

func ReadService(ctxt context.Context, cmd *ReadServiceRequest, adpt *adapter.Adapter, logger *log.Logger) (*api.ReadResponseBody, error) {
	if res, err := ReadServiceRaw(ctxt, cmd, adpt, logger); err == nil {
		var service api.ReadResponseBody
		if err := res.AsType(&service); err != nil {
			return nil, err
		}
		return &service, nil
	} else {
		return nil, err
	}
}

func ReadServiceRaw(ctxt context.Context, cmd *ReadServiceRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	path := servicePath(&cmd.Id, adpt)
	return (*adpt).Get(ctxt, path, logger)
}

/**** UPDATE ****/

// type UpdateRequest = CreateRequest

// func UpdateRaw(ctxt context.Context, cmd *UpdateRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
// 	r := *cmd

// 	path := recordPath(&r.Id, adpt)
// 	if r.Name == "" {
// 		// get current 'name' first as it is required
// 		pld, err := (*adpt).Get(ctxt, path, logger)
// 		if err != nil {
// 			return nil, err
// 		}
// 		obj, err := pld.AsObject()
// 		if err != nil {
// 			logger.Error("no record body found", log.Error(err))
// 			return nil, err
// 		}
// 		r.Name = obj["name"].(string)
// 	}
// 	body, err := json.MarshalIndent(r, "", "  ")
// 	if err != nil {
// 		logger.Error("error marshalling body.", log.Error(err))
// 		return nil, err
// 	}
// 	return (*adpt).Put(ctxt, path, bytes.NewReader(body), logger)
// }

/**** UTILS ****/

func servicePath(id *string, adpt *adapter.Adapter) string {
	path := "/1/services"
	if id != nil {
		path = path + "/" + *id
	}
	return path
}
