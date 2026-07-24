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

// SDK wrappers for the ivcap-accounts service (accounts / projects / invitations).
// Data models come from the generated pkg/accountsapi package (source of truth:
// the ivcap-accounts OpenAPI3 spec). Requests carry the caller's opaque token via
// the Authorization header and the selected project via the Ivcap-Project header;
// the CLI never mints project-scoped JWTs, so request bodies omit any token field.
package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	neturl "net/url"

	log "go.uber.org/zap"

	"github.com/ivcap-works/ivcap-cli/pkg/accountsapi"
	"github.com/ivcap-works/ivcap-cli/pkg/adapter"
)

/**** PATH HELPERS ****/

func accountPath(id *string) string {
	p := "/accounts"
	if id != nil {
		p = p + "/" + *id
	}
	return p
}

func projectPath(id *string) string {
	p := "/projects"
	if id != nil {
		p = p + "/" + *id
	}
	return p
}

/**** ACCOUNTS ****/

func ListAccountsRaw(ctxt context.Context, cmd *ListRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	u, err := createListPath(cmd, accountPath(nil))
	if err != nil {
		return nil, err
	}
	return (*adpt).Get(ctxt, u.String(), logger)
}

func ListAccounts(ctxt context.Context, cmd *ListRequest, adpt *adapter.Adapter, logger *log.Logger) (*accountsapi.ListAccountsResult, error) {
	pyl, err := ListAccountsRaw(ctxt, cmd, adpt, logger)
	if err != nil {
		return nil, err
	}
	var res accountsapi.ListAccountsResult
	if err = pyl.AsType(&res); err != nil {
		return nil, fmt.Errorf("failed to parse accounts response: %w", err)
	}
	return &res, nil
}

func ReadAccountRaw(ctxt context.Context, id string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	return (*adpt).Get(ctxt, accountPath(&id), logger)
}

func CreateAccountRaw(ctxt context.Context, name string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	return postJSON(ctxt, accountPath(nil), accountsapi.CreateAccountPayload2{Name: name}, adpt, logger)
}

func ListAccountMembersRaw(ctxt context.Context, accountID string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	return (*adpt).Get(ctxt, accountPath(&accountID)+"/members", logger)
}

// GrantAccountRaw grants account-admin capabilities to a user (batch add).
func GrantAccountRaw(ctxt context.Context, accountID string, req *accountsapi.AddAccountGrantPayload2, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	return postJSON(ctxt, accountPath(&accountID)+"/grants", req, adpt, logger)
}

// RemoveAccountGrantRaw revokes a single account capability from a user.
func RemoveAccountGrantRaw(ctxt context.Context, accountID, userID, capability string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	q := neturl.Values{"capability": {capability}}
	return (*adpt).Delete(ctxt, accountPath(&accountID)+"/grants/"+userID+"?"+q.Encode(), logger)
}

// RemoveAccountMemberRaw removes a user from an account entirely (revoking all their grants).
func RemoveAccountMemberRaw(ctxt context.Context, accountID, userID string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	return (*adpt).Delete(ctxt, accountPath(&accountID)+"/members/"+userID, logger)
}

/**** PROJECTS ****/

func ListProjectsRaw(ctxt context.Context, cmd *ListRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	u, err := createListPath(cmd, projectPath(nil))
	if err != nil {
		return nil, err
	}
	return (*adpt).Get(ctxt, u.String(), logger)
}

func ListProjects(ctxt context.Context, cmd *ListRequest, adpt *adapter.Adapter, logger *log.Logger) (*accountsapi.ListProjectsResult, error) {
	pyl, err := ListProjectsRaw(ctxt, cmd, adpt, logger)
	if err != nil {
		return nil, err
	}
	var res accountsapi.ListProjectsResult
	if err = pyl.AsType(&res); err != nil {
		return nil, fmt.Errorf("failed to parse projects response: %w", err)
	}
	return &res, nil
}

func ReadProjectRaw(ctxt context.Context, id string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	return (*adpt).Get(ctxt, projectPath(&id), logger)
}

func ReadProject(ctxt context.Context, id string, adpt *adapter.Adapter, logger *log.Logger) (*accountsapi.Project, error) {
	pyl, err := ReadProjectRaw(ctxt, id, adpt, logger)
	if err != nil {
		return nil, err
	}
	var p accountsapi.Project
	if err = pyl.AsType(&p); err != nil {
		return nil, fmt.Errorf("failed to parse project response: %w", err)
	}
	return &p, nil
}

func CreateProjectRaw(ctxt context.Context, req *accountsapi.CreateProjectPayload2, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	return postJSON(ctxt, projectPath(nil), req, adpt, logger)
}

func UpdateProjectRaw(ctxt context.Context, id string, req *accountsapi.UpdateProjectPayload2, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	body, err := json.MarshalIndent(req, "", "  ")
	if err != nil {
		return nil, err
	}
	return (*adpt).Patch(ctxt, projectPath(&id), bytes.NewReader(body), int64(len(body)), nil, logger)
}

func DeleteProjectRaw(ctxt context.Context, id string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	return (*adpt).Delete(ctxt, projectPath(&id), logger)
}

func LeaveProjectRaw(ctxt context.Context, id string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	return (*adpt).Post(ctxt, projectPath(&id)+"/leave", nil, -1, nil, logger)
}

func GrantProjectRaw(ctxt context.Context, id string, req *accountsapi.AddProjectGrantPayload2, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	return postJSON(ctxt, projectPath(&id)+"/grants", req, adpt, logger)
}

// RemoveProjectGrantRaw revokes a single project capability from a principal
// (user or service). Removal is per-capability; call once per capability.
func RemoveProjectGrantRaw(ctxt context.Context, projectID, principalID, principalKind, capability string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	q := neturl.Values{"principal_kind": {principalKind}, "capability": {capability}}
	return (*adpt).Delete(ctxt, projectPath(&projectID)+"/grants/"+principalID+"?"+q.Encode(), logger)
}

// RemoveProjectMemberRaw removes a principal from a project entirely, atomically
// revoking all of their project capabilities in one server-side transaction.
func RemoveProjectMemberRaw(ctxt context.Context, projectID, principalID, principalKind string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	q := neturl.Values{"principal_kind": {principalKind}}
	return (*adpt).Delete(ctxt, projectPath(&projectID)+"/members/"+principalID+"?"+q.Encode(), logger)
}

func ListProjectMembersRaw(ctxt context.Context, projectID string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	return (*adpt).Get(ctxt, projectPath(&projectID)+"/members", logger)
}

/**** INVITATIONS ****/

func ListMyInvitationsRaw(ctxt context.Context, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	return (*adpt).Get(ctxt, "/invitations/mine", logger)
}

func ListMyInvitations(ctxt context.Context, adpt *adapter.Adapter, logger *log.Logger) (*accountsapi.ListInvitationsResult, error) {
	pyl, err := ListMyInvitationsRaw(ctxt, adpt, logger)
	if err != nil {
		return nil, err
	}
	var res accountsapi.ListInvitationsResult
	if err = pyl.AsType(&res); err != nil {
		return nil, fmt.Errorf("failed to parse invitations response: %w", err)
	}
	return &res, nil
}

func AcceptInvitationRaw(ctxt context.Context, id string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	return (*adpt).Post(ctxt, "/invitations/"+id+"/accept", nil, -1, nil, logger)
}

func DeclineInvitationRaw(ctxt context.Context, id string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	return (*adpt).Post(ctxt, "/invitations/"+id+"/decline", nil, -1, nil, logger)
}

func CreateProjectInvitationRaw(ctxt context.Context, projectID string, req *accountsapi.CreateInvitationPayload2, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	id := projectID
	return postJSON(ctxt, projectPath(&id)+"/invitations", req, adpt, logger)
}

func CreateAccountInvitationRaw(ctxt context.Context, accountID string, req *accountsapi.CreateAccountInvitationPayload2, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	id := accountID
	return postJSON(ctxt, accountPath(&id)+"/invitations", req, adpt, logger)
}

// ListProjectInvitationsRaw lists the pending invitations on a project (admin view).
func ListProjectInvitationsRaw(ctxt context.Context, projectID string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	return (*adpt).Get(ctxt, projectPath(&projectID)+"/invitations", logger)
}

// ListAccountInvitationsRaw lists the pending invitations on an account (admin view).
func ListAccountInvitationsRaw(ctxt context.Context, accountID string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	return (*adpt).Get(ctxt, accountPath(&accountID)+"/invitations", logger)
}

// RevokeInvitationRaw cancels a pending invitation the caller issued.
func RevokeInvitationRaw(ctxt context.Context, id string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	return (*adpt).Delete(ctxt, "/invitations/"+id, logger)
}

/**** CAPABILITIES ****/

// CapabilitiesForKind returns the capabilities for "project" or "account" from a
// CapabilitiesResult (nil otherwise).
func CapabilitiesForKind(c *accountsapi.CapabilitiesResult, kind string) []accountsapi.Capability {
	if c == nil {
		return nil
	}
	switch kind {
	case "project":
		return c.Project
	case "account":
		return c.Account
	}
	return nil
}

// GetCapabilities fetches the grantable capability vocabulary from ivcap-accounts.
// The endpoint is public reference data; no project scope is required.
func GetCapabilities(ctxt context.Context, adpt *adapter.Adapter, logger *log.Logger) (*accountsapi.CapabilitiesResult, error) {
	pyl, err := (*adpt).Get(ctxt, "/capabilities", logger)
	if err != nil {
		return nil, err
	}
	var caps accountsapi.CapabilitiesResult
	if err := pyl.AsType(&caps); err != nil {
		return nil, fmt.Errorf("failed to parse capabilities response: %w", err)
	}
	return &caps, nil
}

/**** UTILS ****/

// postJSON marshals v and POSTs it as application/json.
func postJSON(ctxt context.Context, path string, v any, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	body, err := json.MarshalIndent(v, "", "  ")
	if err != nil {
		logger.Error("error marshalling body.", log.Error(err))
		return nil, err
	}
	return (*adpt).Post(ctxt, path, bytes.NewReader(body), int64(len(body)), nil, logger)
}
