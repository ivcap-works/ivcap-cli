package client

import (
	"bytes"
	"context"
	"encoding/json"
	_ "fmt"
	"net/url"
	"strconv"
	"strings"

	api "github.com/reinventingscience/ivcap-core-api/http/order"

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

func ListOrders(ctxt context.Context, cmd *ListOrderRequest, adpt *adapter.Adapter, logger *log.Logger) (*api.ListResponseBody, error) {
	pyl, err := ListOrdersRaw(ctxt, cmd, adpt, logger)
	if err != nil {
		return nil, err
	}
	var list api.ListResponseBody
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
	return (*adpt).Get(ctxt, path, logger)
}

/**** CREATE ****/

func CreateOrder(ctxt context.Context, cmd *api.CreateRequestBody, adpt *adapter.Adapter, logger *log.Logger) (*api.CreateResponseBody, error) {
	pyl, err := CreateOrderRaw(ctxt, cmd, adpt, logger)
	if err != nil {
		return nil, err
	}
	var resp api.CreateResponseBody
	pyl.AsType(&resp)
	return &resp, nil
}

func CreateOrderRaw(ctxt context.Context, cmd *api.CreateRequestBody, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	body, err := json.MarshalIndent(*cmd, "", "  ")
	if err != nil {
		logger.Error("error marshalling body.", log.Error(err))
		return nil, err
	}
	path := orderPath(nil, adpt)
	return (*adpt).Post(ctxt, path, bytes.NewReader(body), int64(len(body)), nil, logger)
}

/**** READ ****/

type ReadOrderRequest struct {
	Id string
}

func ReadOrder(ctxt context.Context, cmd *ReadOrderRequest, adpt *adapter.Adapter, logger *log.Logger) (*api.ReadResponseBody, error) {
	pyl, err := ReadOrderRaw(ctxt, cmd, adpt, logger)
	if err != nil {
		return nil, err
	}
	var order api.ReadResponseBody
	pyl.AsType(&order)
	return &order, nil
}

func ReadOrderRaw(ctxt context.Context, cmd *ReadOrderRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	path := orderPath(&cmd.Id, adpt)
	return (*adpt).Get(ctxt, path, logger)
}

/**** UTILS ****/

func orderPath(id *string, adpt *adapter.Adapter) string {
	path := "/1/orders"
	if id != nil {
		path = path + "/" + *id
	}
	return path
}
