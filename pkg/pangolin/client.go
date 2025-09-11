package pangolin

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
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
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("list orgs failed: status %d: %s", resp.StatusCode, string(b))
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

// ListDomains retrieves all domains for an organization
func (c *Client) ListDomains(ctx context.Context, orgID string) ([]Domain, error) {
	path := fmt.Sprintf("/org/%s/domains?limit=1000&offset=0", orgID)
	resp, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("list domains failed: status %d: %s", resp.StatusCode, string(b))
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Domains []Domain `json:"domains"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("API request was not successful")
	}
	return result.Data.Domains, nil
}

// ListSites retrieves all sites for an organization
func (c *Client) ListSites(ctx context.Context, orgID string) ([]Site, error) {
	path := fmt.Sprintf("/org/%s/sites?limit=1000&offset=0", orgID)
	resp, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("list sites failed: status %d: %s", resp.StatusCode, string(b))
	}

	var result struct {
		Success bool `json:"success"`
		Data    struct {
			Sites []Site `json:"sites"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, fmt.Errorf("failed to decode response: %w", err)
	}
	if !result.Success {
		return nil, fmt.Errorf("API request was not successful")
	}
	return result.Data.Sites, nil
}

// GetSiteByID retrieves a site by numeric siteId
func (c *Client) GetSiteByID(ctx context.Context, siteID int) (*Site, error) {
	resp, err := c.makeRequest(ctx, "GET", fmt.Sprintf("/site/%d", siteID), nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("get site by id failed: status %d: %s", resp.StatusCode, string(b))
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

// GetSiteByNiceID retrieves a site by orgId and niceId
func (c *Client) GetSiteByNiceID(ctx context.Context, orgID, niceID string) (*Site, error) {
	path := fmt.Sprintf("/org/%s/site/%s", orgID, niceID)
	resp, err := c.makeRequest(ctx, "GET", path, nil)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("get site by niceId failed: status %d: %s", resp.StatusCode, string(b))
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
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("create site failed: status %d: %s", resp.StatusCode, string(b))
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

// CreateResource creates a new resource in Pangolin
func (c *Client) CreateResource(ctx context.Context, orgID, siteID string, spec ResourceCreateSpec) (*Resource, error) {
	data := map[string]interface{}{
		"name":     spec.Name,
		"siteId":   spec.SiteID,
		"http":     spec.HTTP,
		"protocol": spec.Protocol,
	}
	if spec.HTTP {
		data["subdomain"] = spec.Subdomain
		if spec.DomainID != "" {
			data["domainId"] = spec.DomainID
		}
	} else {
		data["proxyPort"] = spec.ProxyPort
		data["enableProxy"] = spec.EnableProxy
	}

	resp, err := c.makeRequest(ctx, "PUT", fmt.Sprintf("/org/%s/site/%s/resource", orgID, siteID), data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	// Guard against HTML error pages so JSON decoder doesn't fail with '<'
	if ct := resp.Header.Get("Content-Type"); ct == "" || !strings.HasPrefix(ct, "application/json") {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("unexpected content-type %q (status %d): %s", ct, resp.StatusCode, string(b))
	}

	var result struct {
		Success bool     `json:"success"`
		Data    Resource `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}

	// Normalize ID from resourceId when id is empty (matches Integration API response)
	if result.Data.ID == "" && result.Data.ResourceID > 0 {
		result.Data.ID = strconv.Itoa(result.Data.ResourceID)
	}
	return &result.Data, nil
}

// CreateTarget creates a target for a resource
func (c *Client) CreateTarget(ctx context.Context, resourceID string, spec TargetCreateSpec) (*Target, error) {
	data := map[string]interface{}{
		"ip":      spec.IP,
		"port":    spec.Port,
		"method":  spec.Method,
		"enabled": spec.Enabled,
	}
	resp, err := c.makeRequest(ctx, "PUT", fmt.Sprintf("/resource/%s/target", resourceID), data)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	if ct := resp.Header.Get("Content-Type"); ct == "" || !strings.HasPrefix(ct, "application/json") {
		b, _ := io.ReadAll(io.LimitReader(resp.Body, 1024))
		return nil, fmt.Errorf("unexpected content-type %q (status %d): %s", ct, resp.StatusCode, string(b))
	}

	var result struct {
		Success bool   `json:"success"`
		Data    Target `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return nil, err
	}
	return &result.Data, nil
}
