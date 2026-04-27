// Package git handles GitHub webhook payloads and HMAC validation.
package git

import (
	"crypto/hmac"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"strings"
)

// PushEvent is the relevant subset of a GitHub push webhook payload.
type PushEvent struct {
	Ref  string `json:"ref"`
	After string `json:"after"`

	Repository struct {
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
	} `json:"repository"`

	HeadCommit struct {
		ID      string `json:"id"`
		Message string `json:"message"`
		Author  struct {
			Name string `json:"name"`
		} `json:"author"`
	} `json:"head_commit"`
}

// PREvent is the relevant subset of a GitHub pull_request webhook payload.
type PREvent struct {
	Action string `json:"action"`
	Number int    `json:"number"`

	PullRequest struct {
		Head struct {
			Ref string `json:"ref"`
			SHA string `json:"sha"`
		} `json:"head"`
	} `json:"pull_request"`

	Repository struct {
		FullName string `json:"full_name"`
		CloneURL string `json:"clone_url"`
	} `json:"repository"`
}

// ValidateSignature verifies GitHub's X-Hub-Signature-256 header.
// Returns nil when validation passes or when secret is empty (disabled).
func ValidateSignature(secret string, body []byte, sigHeader string) error {
	if secret == "" {
		return nil
	}
	if !strings.HasPrefix(sigHeader, "sha256=") {
		return fmt.Errorf("missing sha256= prefix in signature header")
	}
	sig := sigHeader[7:]

	mac := hmac.New(sha256.New, []byte(secret))
	mac.Write(body)
	expected := hex.EncodeToString(mac.Sum(nil))

	if !hmac.Equal([]byte(sig), []byte(expected)) {
		return fmt.Errorf("signature mismatch")
	}
	return nil
}

// ParsePushEvent unmarshals a push event payload.
func ParsePushEvent(body []byte) (*PushEvent, error) {
	var evt PushEvent
	if err := json.Unmarshal(body, &evt); err != nil {
		return nil, fmt.Errorf("parse push event: %w", err)
	}
	return &evt, nil
}

// ParsePREvent unmarshals a pull_request event payload.
func ParsePREvent(body []byte) (*PREvent, error) {
	var evt PREvent
	if err := json.Unmarshal(body, &evt); err != nil {
		return nil, fmt.Errorf("parse pr event: %w", err)
	}
	return &evt, nil
}

// BranchFromRef extracts the branch name from a Git ref string (e.g. "refs/heads/main" → "main").
func BranchFromRef(ref string) string {
	return strings.TrimPrefix(ref, "refs/heads/")
}
