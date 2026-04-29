package auth

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"biene/internal/bienehome"
)

const chatgptTokensFile = "chatgpt_tokens.json"

// ChatGPTState is what lands on disk in ~/.biene/chatgpt_tokens.json.
//
// AccessToken is the bearer token sent to chatgpt.com/backend-api on
// every request. AccountID is the value of the `chatgpt-account-id`
// header (extracted from the access_token's JWT claims at login). Both
// rotate together when the refresh_token mints a new access_token.
//
// Email is parsed from the id_token's email claim once at login for UI
// display ("Authorized as foo@bar.com"). The id_token itself is NOT
// persisted: it serves no runtime purpose after login (the access_token
// is what authenticates Codex requests), and keeping it on disk just
// widens the secret surface area for no benefit.
//
// File mode is 0600: this file carries the refresh_token, which grants
// ongoing access to the user's ChatGPT account.
type ChatGPTState struct {
	AccessToken  string `json:"access_token,omitempty"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresAt    int64  `json:"expires_at,omitempty"`
	Email        string `json:"email,omitempty"`
	AccountID    string `json:"account_id,omitempty"`
}

// chatgptTokensPath returns the on-disk location of the credential file.
func chatgptTokensPath() (string, error) {
	root, err := bienehome.HomeDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(root, chatgptTokensFile), nil
}

// LoadChatGPTState reads the persisted ChatGPT credential state. Returns
// (nil, nil) when the file does not exist (= user is logged out); other
// errors propagate.
func LoadChatGPTState() (*ChatGPTState, error) {
	path, err := chatgptTokensPath()
	if err != nil {
		return nil, err
	}
	data, err := os.ReadFile(path)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read chatgpt tokens: %w", err)
	}
	if len(strings.TrimSpace(string(data))) == 0 {
		return nil, nil
	}
	var s ChatGPTState
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("parse chatgpt tokens: %w", err)
	}
	return &s, nil
}

// SaveChatGPTState writes the credential state back to disk with 0600.
func SaveChatGPTState(s *ChatGPTState) error {
	if s == nil {
		return errors.New("chatgpt state is nil")
	}
	path, err := chatgptTokensPath()
	if err != nil {
		return err
	}
	return bienehome.WriteJSON(path, s, 0o700, 0o600)
}

// DeleteChatGPTState removes the credential file. Used by /logout.
// Returns nil if the file did not exist.
func DeleteChatGPTState() error {
	path, err := chatgptTokensPath()
	if err != nil {
		return err
	}
	if err := os.Remove(path); err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("delete chatgpt tokens: %w", err)
	}
	return nil
}
