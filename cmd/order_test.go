package cmd

import (
	"context"
	"testing"

	sdk "github.com/reinventingscience/ivcap-cli/pkg"
	a "github.com/reinventingscience/ivcap-cli/pkg/adapter"
	api "github.com/reinventingscience/ivcap-core-api/http/order"
)

var (
	createOrderReqBody = []byte(`
	{
		"account-id": "urn:ivcap:account:0f0e3f57-80f7-4899-9b69-459af2efd789",
		"name": "Some order name",
		"parameters": [
		  {
			"name": "region",
			"value": "Upper Valley"
		  },
		  {
			"name": "threshold",
			"value": "10"
		  }
		],
		"tags": [
		  "tag1",
		  "tag2"
		]
	  }
`)
)

var orderID string

func testCreateOrder(t *testing.T) {
	if testToken == "" {
		t.Skip("access token not found, login to run unit test...")
	}
	if serviceID == "" {
		t.Skip("service id not set...")
	}
	pyld, err := a.LoadPayloadFromBytes(createOrderReqBody, false)
	if err != nil {
		t.Fatalf("failed to load payload from file: %s, %v", serviceFile, err)
	}
	var req api.CreateRequestBody
	if err = pyld.AsType(&req); err != nil {
		t.Fatalf("failed to unmarshal request body: %v", err)
	}
	req.ServiceID = serviceID

	res, err := sdk.CreateOrder(context.Background(), &req, adapter, tlogger)
	if err != nil {
		t.Fatalf("failed to create order: %v", err)
	}
	if res.ID == nil {
		t.Fatalf("order id not exists")
	}

	orderID = *res.ID
}

func TestListOrder(t *testing.T) {
	if testToken == "" {
		t.Skip("access token not found, login to run unit test...")
	}
	req := &sdk.ListOrderRequest{Offset: 0, Limit: 5}
	_, err := sdk.ListOrders(context.Background(), req, adapter, tlogger)
	if err != nil {
		t.Fatalf("failed to list orders: %v", err)
	}
}

func testGetOrder(t *testing.T) {
	if testToken == "" {
		t.Skip("access token not found, login to run unit test...")
	}

	if orderID == "" {
		t.Skip("order id not set...")
	}

	req := &sdk.ReadOrderRequest{Id: orderID}
	res, err := sdk.ReadOrder(context.Background(), req, adapter, tlogger)
	if err != nil {
		t.Fatalf("failed to read service: %v", err)
	}
	if res.ID == nil {
		t.Fatalf("order id not exists")
	}
	if *res.ID != orderID {
		t.Fatalf("unexpected updated id: %v, expecting: %s", *res.ID, orderID)
	}
}
