// Copyright 2024 Commonwealth Scientific and Industrial Research Organisation (CSIRO) ABN 41 687 119 230
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
	"fmt"
	"io"
	"net/url"
	"strconv"
	"time"

	"github.com/k0kubun/go-ansi"
	"github.com/schollz/progressbar/v3"
)

func CreateListPath(cmd *ListRequest, path string) (*url.URL, error) {
	u, err := url.Parse(path)
	if err != nil {
		return nil, fmt.Errorf("failed to parse path %s to url: %w", path, err)
	}

	query := u.Query()
	if cmd.Limit > 0 {
		query.Set("limit", strconv.FormatInt(int64(cmd.Limit), 10))
	}
	if cmd.Page != nil {
		query.Set("page", *cmd.Page)
	}
	if cmd.Search != nil {
		query.Set("search", *cmd.Search)
	}
	if cmd.Filter != nil {
		query.Set("filter", *cmd.Filter)
	}
	if cmd.OrderBy != nil {
		query.Set("order-by", *cmd.OrderBy)
	}
	if cmd.OrderDesc {
		query.Set("order-desc", strconv.FormatBool(cmd.OrderDesc))
	}
	if cmd.AtTime != nil {
		query.Set("at-time", cmd.AtTime.Format(time.RFC3339))
	}

	u.RawQuery = query.Encode()
	return u, nil
}

func AddProgressBar(description string, size int64, reader io.Reader) io.Reader {
	bar := GetProgressBar(description, size)
	return io.TeeReader(reader, bar)
}

func GetProgressBar(description string, size int64) io.Writer {
	return progressbar.NewOptions64(size,
		progressbar.OptionSetWriter(ansi.NewAnsiStderr()),
		progressbar.OptionEnableColorCodes(true),
		progressbar.OptionShowBytes(true),
		progressbar.OptionSetWidth(30),
		progressbar.OptionSetDescription(description),
		progressbar.OptionSetTheme(progressbar.Theme{
			Saucer:        "[green]=[reset]",
			SaucerHead:    "[green]>[reset]",
			SaucerPadding: " ",
			BarStart:      "[",
			BarEnd:        "]",
		}),
	)
}
