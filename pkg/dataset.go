// Copyright 2026 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
//
// Licensed under the Apache License, Version 2.0 (the "License");
// you may not use this file except in compliance with the License.
// You may obtain a copy of the License at
//
//      http://www.apache.org/licenses/LICENSE-2.0
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
	"fmt"
	neturl "net/url"
	"strings"

	log "go.uber.org/zap"

	"github.com/ivcap-works/ivcap-cli/pkg/accountsapi"
	"github.com/ivcap-works/ivcap-cli/pkg/adapter"
)

// Dataset grantee kinds and access levels, as the ivcap-accounts API spells them.
const (
	DatasetGranteeProject = "project"
	DatasetGranteeService = "service"
	DatasetGranteePublic  = "public"

	DatasetAccessRead  = "read"
	DatasetAccessWrite = "write"
)

const datasetURNPrefix = "urn:ivcap:dataset:"

// ValidateDatasetURN checks that id is a dataset URN. Datasets are addressed by
// URN only; a project's default dataset has the project's UUID under the
// dataset prefix.
func ValidateDatasetURN(id string) error {
	if !strings.HasPrefix(id, datasetURNPrefix) || len(id) == len(datasetURNPrefix) {
		return fmt.Errorf("%q is not a dataset URN (expected %s<uuid>)", id, datasetURNPrefix)
	}
	return nil
}

// ValidateDatasetGrant checks a grant's shape before any request is made: the
// grantee kind, that a project or service grantee names an id and a public one
// does not, and that public access is read-only.
func ValidateDatasetGrant(granteeKind, granteeID, access string) error {
	switch access {
	case DatasetAccessRead, DatasetAccessWrite:
	default:
		return fmt.Errorf("access must be %q or %q, got %q", DatasetAccessRead, DatasetAccessWrite, access)
	}
	switch granteeKind {
	case DatasetGranteeProject, DatasetGranteeService:
		if granteeID == "" {
			return fmt.Errorf("a %s grantee needs an id", granteeKind)
		}
	case DatasetGranteePublic:
		if granteeID != "" {
			return fmt.Errorf("a public grant takes no grantee id")
		}
		if access != DatasetAccessRead {
			return fmt.Errorf("public access is read-only")
		}
	default:
		return fmt.Errorf("grantee kind must be project, service or public, got %q", granteeKind)
	}
	return nil
}

func datasetPath(id *string) string {
	p := accountsAPIPrefix + "/datasets"
	if id != nil {
		p = p + "/" + *id
	}
	return p
}

func ListProjectDatasetsRaw(ctxt context.Context, projectID string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	return (*adpt).Get(ctxt, projectPath(&projectID)+"/datasets", logger)
}

func CreateDatasetRaw(ctxt context.Context, projectID string, req *accountsapi.CreateDatasetPayload, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	return postJSON(ctxt, projectPath(&projectID)+"/datasets", req, adpt, logger)
}

func ReadDatasetRaw(ctxt context.Context, id string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	return (*adpt).Get(ctxt, datasetPath(&id), logger)
}

func ReadDataset(ctxt context.Context, id string, adpt *adapter.Adapter, logger *log.Logger) (*accountsapi.Dataset, error) {
	pyl, err := ReadDatasetRaw(ctxt, id, adpt, logger)
	if err != nil {
		return nil, err
	}
	var d accountsapi.Dataset
	if err = pyl.AsType(&d); err != nil {
		return nil, fmt.Errorf("failed to parse dataset response: %w", err)
	}
	return &d, nil
}

func UpdateDatasetRaw(ctxt context.Context, id string, req *accountsapi.UpdateDatasetPayload, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	body, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return nil, err
	}
	return (*adpt).Patch(ctxt, datasetPath(&id), bytes.NewReader(body), int64(len(body)), nil, logger)
}

func ListDatasetGrantsRaw(ctxt context.Context, id string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	return (*adpt).Get(ctxt, datasetPath(&id)+"/grants", logger)
}

func AddDatasetGrantRaw(ctxt context.Context, id string, req *accountsapi.DatasetGrantPayload, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	if err := ValidateDatasetGrant(req.GranteeKind, deref(req.GranteeId), req.Access); err != nil {
		return nil, err
	}
	return postJSON(ctxt, datasetPath(&id)+"/grants", req, adpt, logger)
}

// RemoveDatasetGrantRaw revokes one grant. Like AddDatasetGrantRaw it refuses a
// malformed grant before making a request.
func RemoveDatasetGrantRaw(ctxt context.Context, id string, req *accountsapi.DatasetGrantPayload, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	if err := ValidateDatasetGrant(req.GranteeKind, deref(req.GranteeId), req.Access); err != nil {
		return nil, err
	}
	q := neturl.Values{"grantee_kind": {req.GranteeKind}, "access": {req.Access}}
	if req.GranteeId != nil {
		q.Set("grantee_id", *req.GranteeId)
	}
	return (*adpt).Delete(ctxt, datasetPath(&id)+"/grants?"+q.Encode(), logger)
}

func ListDatasetEventsRaw(ctxt context.Context, id string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	return (*adpt).Get(ctxt, datasetPath(&id)+"/events", logger)
}

func deref(s *string) string {
	if s == nil {
		return ""
	}
	return *s
}
