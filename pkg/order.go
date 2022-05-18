package client

import (
	"bytes"
	"context"
	"encoding/json"
	_ "fmt"
	"net/url"
	"strconv"
	"strings"

	"cayp/api_gateway/gen/http/order/client"

	"github.com/reinventingscience/ivcap-client/pkg/adapter"

	log "go.uber.org/zap"
)

/**** LIST ****/

type ListOrderRequest struct {
	Offset int
	Limit  int
}

// type ListResult struct {
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

func ListOrders(ctxt context.Context, cmd *ListOrderRequest, adpt *adapter.Adapter, logger *log.Logger) (*client.ListResponseBody, error) {
	pyl, err := ListOrdersRaw(ctxt, cmd, adpt, logger)
	if err != nil {
		return nil, err
	}
	var list client.ListResponseBody
	pyl.AsType(&list)
	return &list, nil
}

func ListOrdersRaw(ctxt context.Context, cmd *ListOrderRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	path := orderPath(nil, adpt)

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

func CreateOrder(ctxt context.Context, cmd *client.CreateRequestBody, adpt *adapter.Adapter, logger *log.Logger) (*client.CreateResponseBody, error) {
	pyl, err := CreateOrderRaw(ctxt, cmd, adpt, logger)
	if err != nil {
		return nil, err
	}
	var resp client.CreateResponseBody
	pyl.AsType(&resp)
	return &resp, nil
}

func CreateOrderRaw(ctxt context.Context, cmd *client.CreateRequestBody, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	body, err := json.MarshalIndent(*cmd, "", "  ")
	if err != nil {
		logger.Error("error marshalling body.", log.Error(err))
		return nil, err
	}
	// fmt.Printf("RECORD %+v - %s\n", cmd, body)

	path := orderPath(nil, adpt)
	return (*adpt).Post(ctxt, path, bytes.NewReader(body), logger)
}

/**** READ ****/

type ReadOrderRequest struct {
	Id string
}

func ReadOrder(ctxt context.Context, cmd *ReadOrderRequest, adpt *adapter.Adapter, logger *log.Logger) (*client.ReadResponseBody, error) {
	pyl, err := ReadOrderRaw(ctxt, cmd, adpt, logger)
	if err != nil {
		return nil, err
	}
	var order client.ReadResponseBody
	pyl.AsType(&order)
	return &order, nil
}

func ReadOrderRaw(ctxt context.Context, cmd *ReadOrderRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	path := orderPath(&cmd.Id, adpt)
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

func orderPath(id *string, adpt *adapter.Adapter) string {
	path := "/1/orders"
	if id != nil {
		path = path + "/" + *id
	}
	return path
}
