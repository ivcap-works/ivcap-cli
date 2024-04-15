package client

import (
	"fmt"
	"net/url"
	"strconv"
	"time"
)

type ListRequest struct {
	Limit     int
	Page      *string
	Filter    *string
	OrderBy   *string
	OrderDesc bool
	AtTime    *time.Time
}

func createListPath(cmd *ListRequest, path string) (*url.URL, error) {
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
