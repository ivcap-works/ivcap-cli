// Copyright 2023 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
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
	"net/url"
	"strconv"
	"strings"

	log "go.uber.org/zap"

	"github.com/ivcap-works/ivcap-cli/pkg/adapter"
	api "github.com/ivcap-works/ivcap-core-api/http/project"
	"github.com/spf13/cobra"
)

/**** LIST ****/

type ListProjectsRequest struct {
	Limit int
	Page  string
}

func ListProjects(ctx context.Context, cmd *ListProjectsRequest, adpt *adapter.Adapter, logger *log.Logger) (*api.ListResponseBody, error) {
	lr := &ListRequest{
		Limit: cmd.Limit,
		Page:  &cmd.Page,
	}
	pyl, err := ListProjectsRaw(ctx, lr, adpt, logger)
	if err != nil {
		return nil, err
	}
	var list api.ListResponseBody
	if err = pyl.AsType(&list); err != nil {
		return nil, fmt.Errorf("failed to parse list response body: %w", err)
	}
	return &list, nil
}

func ListProjectsRaw(ctx context.Context, cmd *ListRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	u, err := createListPath(cmd, projectPath(nil))
	if err != nil {
		return nil, err
	}
	return (*adpt).Get(ctx, u.String(), logger)
}

/**** LIST PROJECT MEMBERS ****/
type ListProjectMembersRequest struct {
	ProjectURN string
	Limit      int
	Page       string
}

func ListProjectMembersRaw(ctx context.Context, cmd *ListProjectMembersRequest, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	if cmd.ProjectURN == "" {
		cobra.CheckErr("No project urn provided")
	}

	path := membersPath(&cmd.ProjectURN, nil)

	pa := []string{}
	if cmd.Limit > 0 {
		pa = append(pa, "limit="+url.QueryEscape(strconv.Itoa(cmd.Limit)))
	}
	if cmd.Page != "" {
		pa = append(pa, "page="+url.QueryEscape(cmd.Page))
	}
	if len(pa) > 0 {
		path = path + "?" + strings.Join(pa, "&")
	}
	return (*adpt).Get(ctx, path, logger)
}

/**** UPDATE MEMBERSHIP ****/
func UpdateMembershipRaw(ctx context.Context,
	projectURN string,
	userURN string,
	cmd *api.UpdateMembershipRequestBody,
	adpt *adapter.Adapter,
	logger *log.Logger) (adapter.Payload, error) {
	if projectURN == "" {
		cobra.CheckErr("No project URN provided")
	}
	if userURN == "" {
		cobra.CheckErr("No user URN provided")
	}

	path := membershipsPath(projectURN, userURN)

	body, err := json.MarshalIndent(*cmd, "", "  ")
	if err != nil {
		logger.Error("error marshalling body.", log.Error(err))
		return nil, err
	}

	return (*adpt).Put(ctx, path, bytes.NewReader(body), int64(len(body)), nil, logger)
}

/**** REMOVE MEMBERSHIP ****/
func RemoveMembershipRaw(ctx context.Context,
	projectURN string,
	userURN string,
	adpt *adapter.Adapter,
	logger *log.Logger) (adapter.Payload, error) {
	if projectURN == "" {
		cobra.CheckErr("No project URN provided")
	}
	if userURN == "" {
		cobra.CheckErr("No user URN provided")
	}

	path := membershipsPath(projectURN, userURN)

	return (*adpt).Delete(ctx, path, logger)
}

/**** Project Info ****/
func ProjectInfoRaw(ctx context.Context, projectURN string, adpt *adapter.Adapter, logger *log.Logger) (adapter.Payload, error) {
	if projectURN == "" {
		cobra.CheckErr("No project URN provided")
	}

	path := projectPath(&projectURN)

	return (*adpt).Get(ctx, path, logger)
}

/**** CREATE ****/

func CreateProjectRaw(
	ctxt context.Context,
	cmd *api.CreateProjectRequestBody,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (adapter.Payload, error) {
	body, err := json.MarshalIndent(*cmd, "", "  ")
	if err != nil {
		logger.Error("error marshalling body.", log.Error(err))
		return nil, err
	}

	path := projectPath(nil)
	return (*adpt).Post(ctxt, path, bytes.NewReader(body), int64(len(body)), nil, logger)
}

/**** DELETE ****/

type DeleteProjectRequest struct {
	ProjectId string
}

func DeleteProjectRaw(
	ctx context.Context,
	cmd *DeleteProjectRequest,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (adapter.Payload, error) {
	path := projectPath(&cmd.ProjectId)

	return (*adpt).Delete(ctx, path, logger)
}

/**** Get Default Project ****/
func GetDefaultProjectRaw(
	ctx context.Context,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (adapter.Payload, error) {
	path := defaultProjectPath()

	return (*adpt).Get(ctx, path, logger)
}

/**** Set Default Project ****/
func SetDefaultProjectRaw(
	ctx context.Context,
	cmd *api.SetDefaultProjectRequestBody,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (adapter.Payload, error) {
	path := defaultProjectPath()

	body, err := json.MarshalIndent(*cmd, "", "  ")
	if err != nil {
		logger.Error("error marshalling body.", log.Error(err))
		return nil, err
	}

	return (*adpt).Put(ctx, path, bytes.NewReader(body), int64(len(body)), nil, logger)
}

/**** Get Project Account ****/
func GetProjectAccountRaw(
	ctx context.Context,
	projectURN string,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (adapter.Payload, error) {
	path := accountPath(&projectURN)

	return (*adpt).Get(ctx, path, logger)
}

/**** Set Project Account ****/
func SetProjectAccountRaw(
	ctx context.Context,
	projectURN string,
	cmd *api.SetProjectAccountRequestBody,
	adpt *adapter.Adapter,
	logger *log.Logger,
) (adapter.Payload, error) {
	path := accountPath(&projectURN)

	body, err := json.MarshalIndent(*cmd, "", "  ")
	if err != nil {
		logger.Error("error marshalling body.", log.Error(err))
		return nil, err
	}

	return (*adpt).Put(ctx, path, bytes.NewReader(body), int64(len(body)), nil, logger)
}

/**** UTILS ****/

func projectPath(projectURN *string) string {
	path := "/1/project"
	if projectURN != nil {
		path = path + "/" + *projectURN
	}
	return path
}

func defaultProjectPath() string {
	path := projectPath(nil) + "/default"
	return path
}

func membersPath(projectURN *string, userURN *string) string {
	path := projectPath(projectURN) + "/members"
	return path
}

func membershipsPath(projectURN string, userURN string) string {
	path := projectPath(&projectURN) + "/memberships/" + userURN
	return path
}

func accountPath(projectURN *string) string {
	path := projectPath(projectURN) + "/account"
	return path
}
