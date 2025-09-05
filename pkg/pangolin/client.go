package pangolin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Client represents a Pangolin API client
type Client struct {
	endpoint string
	apiKey   string
	client   *http.Client
}

// NewClient creates a new Pangolin API client
func NewClient(endpoint, apiKey string) *Client {
	return &Client{
		endpoint: endpoint,
		apiKey:   apiKey,
		client: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
}

// makeRequest makes an HTTP request to the Pangolin API
func (c *Client) makeRequest(ctx context.Context, method, path string, body interface{}) (*http.Response, error) {
	url := fmt.Sprintf("%s/v1%s", c.endpoint, path)

	var reqBody []byte
	var err error

	if body != nil {
		reqBody, err = json.Marshal(body)
		if err != nil {
			return nil, fmt.Errorf("failed to marshal request body: %w", err)
		}
	}

	req, err := http.NewRequestWithContext(ctx, method, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("User-Agent", "pangolin-operator/1.0")
	req.Header.Set("Authorization", fmt.Sprintf("Bearer %s", c.apiKey))

	resp, err := c.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to make request: %w", err)
	}

	return resp, nil
}

// ListOrganizations retrieves all organizations
func (c *Client) ListOrganizations(ctx context.Context) ([]Organization, error) {
	resp, err := c.makeRequest(ctx, "GET", "/orgs", nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Orgs []Organization `json:"orgs"`
		} `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("API request was not successful")
	}

	return result.Data.Orgs, nil
}

// CreateSite creates a new site
func (c *Client) CreateSite(ctx context.Context, orgID, name, siteType string) (*Site, error) {
	body := map[string]interface{}{
		"name": name,
		"type": siteType,
	}

	path := fmt.Sprintf("/org/%s/site", orgID)
	resp, err := c.makeRequest(ctx, "PUT", path, body)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var result struct {
		Success bool `json:"success"`
		Data    Site `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("API request was not successful")
	}

	return &result.Data, nil
}

// CreateResource creates a new resource
func (c *Client) CreateResource(ctx context.Context, orgID, siteID string, spec ResourceCreateSpec) (*Resource, error) {
	path := fmt.Sprintf("/org/%s/site/%s/resource", orgID, siteID)
	resp, err := c.makeRequest(ctx, "PUT", path, spec)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var result struct {
		Success bool     `json:"success"`
		Data    Resource `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("API request was not successful")
	}

	return &result.Data, nil
}

// CreateTarget creates a new target for a resource
func (c *Client) CreateTarget(ctx context.Context, resourceID string, spec TargetCreateSpec) (*Target, error) {
	path := fmt.Sprintf("/resource/%s/target", resourceID)
	resp, err := c.makeRequest(ctx, "PUT", path, spec)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK && resp.StatusCode != http.StatusCreated {
		return nil, fmt.Errorf("API request failed with status %d", resp.StatusCode)
	}

	var result struct {
		Success bool   `json:"success"`
		Data    Target `json:"data"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}

	if !result.Success {
		return nil, fmt.Errorf("API request was not successful")
	}

	return &result.Data, nil
}
