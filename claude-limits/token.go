package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os/exec"
)

var errNoAccessToken = errors.New("no Claude.ai login in Claude Code's credentials")

func accessToken(item []byte) (string, error) {
	var credentials struct {
		ClaudeAIOAuth *struct {
			AccessToken string `json:"accessToken"`
		} `json:"claudeAiOauth"`
	}
	if err := json.Unmarshal(item, &credentials); err != nil {
		return "", fmt.Errorf("reading Claude Code's credentials: %w", err)
	}

	if credentials.ClaudeAIOAuth == nil || credentials.ClaudeAIOAuth.AccessToken == "" {
		return "", errNoAccessToken
	}

	return credentials.ClaudeAIOAuth.AccessToken, nil
}

func keychainCredentials(ctx context.Context) ([]byte, error) {
	item, err := exec.CommandContext(ctx, "/usr/bin/security", "find-generic-password", "-s", "Claude Code-credentials", "-w").Output()
	if err != nil {
		return nil, fmt.Errorf("reading Claude Code's credentials from the Keychain: %w", err)
	}

	return item, nil
}
