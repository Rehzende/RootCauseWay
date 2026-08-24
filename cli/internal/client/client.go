package client

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"time"
)

// Config holds CLI configuration persisted to disk.
type Config struct {
	APIURL string `json:"api_url"`
	Token  string `json:"token"`
}

// Client is the HTTP client for the RootCauseway API.
type Client struct {
	BaseURL string
	Token   string
	HTTP    *http.Client
}

// configPath returns ~/.rootcauseway/config.json.
func configPath() string {
	home, _ := os.UserHomeDir()
	return filepath.Join(home, ".rootcauseway", "config.json")
}

// LoadConfig reads the config from disk.
func LoadConfig() (*Config, error) {
	data, err := os.ReadFile(configPath())
	if err != nil {
		if os.IsNotExist(err) {
			return &Config{APIURL: "http://localhost:8080"}, nil
		}
		return nil, err
	}
	var cfg Config
	if err := json.Unmarshal(data, &cfg); err != nil {
		return nil, err
	}
	if cfg.APIURL == "" {
		cfg.APIURL = "http://localhost:8080"
	}
	return &cfg, nil
}

// SaveConfig writes the config to disk.
func SaveConfig(cfg *Config) error {
	dir := filepath.Dir(configPath())
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return err
	}
	data, err := json.MarshalIndent(cfg, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath(), data, 0o600)
}

// NewFromConfig creates a Client from the saved config.
func NewFromConfig() (*Client, error) {
	cfg, err := LoadConfig()
	if err != nil {
		return nil, fmt.Errorf("failed to load config: %w", err)
	}
	return &Client{
		BaseURL: cfg.APIURL,
		Token:   cfg.Token,
		HTTP:    &http.Client{Timeout: 30 * time.Second},
	}, nil
}

// Get performs an authenticated GET request.
func (c *Client) Get(path string) (*http.Response, error) {
	req, err := http.NewRequest("GET", c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)
	return c.HTTP.Do(req)
}

// Post performs an authenticated POST request with a JSON body.
func (c *Client) Post(path string, body interface{}) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest("POST", c.BaseURL+path, reader)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.HTTP.Do(req)
}

// Patch performs an authenticated PATCH request with a JSON body.
func (c *Client) Patch(path string, body interface{}) (*http.Response, error) {
	var reader io.Reader
	if body != nil {
		data, err := json.Marshal(body)
		if err != nil {
			return nil, err
		}
		reader = bytes.NewReader(data)
	}
	req, err := http.NewRequest("PATCH", c.BaseURL+path, reader)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.HTTP.Do(req)
}

// Delete performs an authenticated DELETE request.
func (c *Client) Delete(path string) (*http.Response, error) {
	req, err := http.NewRequest("DELETE", c.BaseURL+path, nil)
	if err != nil {
		return nil, err
	}
	c.setHeaders(req)
	return c.HTTP.Do(req)
}

func (c *Client) setHeaders(req *http.Request) {
	if c.Token != "" {
		req.Header.Set("Authorization", "Bearer "+c.Token)
	}
	req.Header.Set("Accept", "application/json")
}

// ReadBody reads and closes the response body.
func ReadBody(resp *http.Response) ([]byte, error) {
	defer resp.Body.Close()
	return io.ReadAll(resp.Body)
}

// CheckResponse returns an error if the response status is not 2xx.
func CheckResponse(resp *http.Response) error {
	if resp.StatusCode >= 200 && resp.StatusCode < 300 {
		return nil
	}
	body, _ := ReadBody(resp)
	return fmt.Errorf("API error (HTTP %d): %s", resp.StatusCode, string(body))
}
