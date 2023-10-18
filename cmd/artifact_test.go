package cmd

import (
	"context"
	"testing"

	sdk "github.com/reinventingscience/ivcap-cli/pkg"
)

func TestListArtifact(t *testing.T) {
	if testToken == "" {
		t.Skip("access token not found, login to run unit test...")
	}
	req := &sdk.ListArtifactRequest{Offset: 0, Limit: 5}
	_, err := sdk.ListArtifacts(context.Background(), req, adapter, tlogger)
	if err != nil {
		t.Fatalf("failed to list artifacts: %v", err)
	}
}

func testAddArtifact(t *testing.T) {
	if testToken == "" {
		t.Skip("access token not found, login to run unit test...")
	}
}
