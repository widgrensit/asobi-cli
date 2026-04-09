package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"

	"github.com/widgrensit/asobi-go/internal/config"
)

type Client struct {
	cfg    *config.Config
	http   *http.Client
}

func New(cfg *config.Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: 30 * time.Second},
	}
}

func (c *Client) Health() (map[string]any, error) {
	return c.get("/health")
}

type DeployRequest struct {
	Scripts []Script `json:"scripts"`
}

type Script struct {
	Path    string `json:"path"`
	Content string `json:"content"`
}

type DeployResponse struct {
	Deployed int      `json:"deployed"`
	Scripts  []string `json:"scripts"`
	Error    string   `json:"error,omitempty"`
	Message  string   `json:"message,omitempty"`
}

func (c *Client) Deploy(scripts []Script) (*DeployResponse, error) {
	body, err := json.Marshal(DeployRequest{Scripts: scripts})
	if err != nil {
		return nil, fmt.Errorf("marshal: %w", err)
	}

	req, err := http.NewRequest("POST", c.cfg.URL+"/internal/deploy", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("x-api-key", c.cfg.APIKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("read response: %w", err)
	}

	var result DeployResponse
	if err := json.Unmarshal(data, &result); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}

	if resp.StatusCode != 200 {
		msg := result.Message
		if msg == "" {
			msg = result.Error
		}
		return nil, fmt.Errorf("deploy failed (%d): %s", resp.StatusCode, msg)
	}

	return &result, nil
}

func (c *Client) get(path string) (map[string]any, error) {
	req, err := http.NewRequest("GET", c.cfg.URL+path, nil)
	if err != nil {
		return nil, err
	}
	if c.cfg.APIKey != "" {
		req.Header.Set("x-api-key", c.cfg.APIKey)
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return result, nil
}
