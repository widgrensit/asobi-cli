package client

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/widgrensit/asobi-cli/internal/config"
)

type Client struct {
	cfg  *config.Config
	http *http.Client
}

func New(cfg *config.Config) *Client {
	return &Client{
		cfg:  cfg,
		http: &http.Client{Timeout: 300 * time.Second},
	}
}

func (c *Client) Health() (map[string]any, error) {
	return c.get("/health")
}

type Script struct {
	Path    string `json:"path"`
	Content string `json:"content"`
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
