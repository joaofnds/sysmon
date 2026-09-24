package main

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
)

var errUsageAPI = errors.New("the usage API refused the request")

// usageAPI is the endpoint Claude Code's /usage reads. Anthropic does not document it.
type usageAPI struct {
	client  *http.Client
	baseURL string
}

func (a usageAPI) limits(ctx context.Context, token string) ([]limit, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, a.baseURL+"/api/oauth/usage", nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)
	req.Header.Set("anthropic-beta", "oauth-2025-04-20")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("User-Agent", "sysmon-claude-limits")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("calling the usage API: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("%w: %s", errUsageAPI, resp.Status)
	}

	body, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return nil, fmt.Errorf("reading the usage API's answer: %w", err)
	}
	return parseLimits(body)
}
