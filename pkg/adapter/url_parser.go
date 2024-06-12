package adapter

import (
	"fmt"
	neturl "net/url"
)

func parseURL(endpoint string, connCtxt *ConnectionCtxt) (*neturl.URL, error) {
	// Try to parse the endpoint as an absolute URL
	parsedURL, err := neturl.Parse(endpoint)
	if err != nil {
		return nil, fmt.Errorf("failed to parse endpoint: %w", err)
	}

	// If the endpoint is not absolute, resolve it against the base URL
	if !parsedURL.IsAbs() {
		base, err := neturl.Parse(connCtxt.URL)
		if err != nil {
			return nil, fmt.Errorf("failed to parse base URL: %w", err)
		}
		parsedURL = base.ResolveReference(parsedURL)
	}

	// Check if the scheme is http
	if parsedURL.Scheme != "http" && parsedURL.Scheme != "https" {
		return nil, fmt.Errorf("invalid scheme: %s", parsedURL.Scheme)
	}

	return parsedURL, nil
}
