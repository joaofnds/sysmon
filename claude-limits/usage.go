package main

import (
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

var (
	errNoLimits        = errors.New("the usage response has no limits")
	errIncompleteLimit = errors.New("a limit in the usage response has no kind or no percent")
)

// A limit is one share of the plan's usage that Anthropic caps and resets on a schedule:
// the session limit, the weekly limit across all models, or the weekly limit of one model
// or one surface.
type limit struct {
	Kind        string
	Model       string
	Surface     string
	UsedPercent float64
	ResetsAt    time.Time
}

type usageResponse struct {
	Limits []struct {
		Kind     string     `json:"kind"`
		Percent  *float64   `json:"percent"`
		ResetsAt *time.Time `json:"resets_at"`
		Scope    *struct {
			Model   *scopeName `json:"model"`
			Surface *scopeName `json:"surface"`
		} `json:"scope"`
	} `json:"limits"`
}

type scopeName struct {
	DisplayName string `json:"display_name"`
}

func parseLimits(body []byte) ([]limit, error) {
	var r usageResponse
	if err := json.Unmarshal(body, &r); err != nil {
		return nil, fmt.Errorf("reading the usage response: %w", err)
	}

	if r.Limits == nil {
		return nil, errNoLimits
	}

	limits := make([]limit, 0, len(r.Limits))
	for i, l := range r.Limits {
		if l.Kind == "" || l.Percent == nil {
			return nil, fmt.Errorf("%w: limit %d", errIncompleteLimit, i)
		}

		parsed := limit{Kind: l.Kind, UsedPercent: *l.Percent}
		if l.ResetsAt != nil {
			parsed.ResetsAt = l.ResetsAt.UTC()
		}
		if l.Scope != nil && l.Scope.Model != nil {
			parsed.Model = l.Scope.Model.DisplayName
		}
		if l.Scope != nil && l.Scope.Surface != nil {
			parsed.Surface = l.Scope.Surface.DisplayName
		}

		limits = append(limits, parsed)
	}

	return limits, nil
}
