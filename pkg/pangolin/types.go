package pangolin

// Organization represents a Pangolin organization
type Organization struct {
	OrgID  string `json:"orgId"`
	Name   string `json:"name"`
	Subnet string `json:"subnet"`
}

// Site represents a Pangolin site
type Site struct {
	ID       string `json:"id,omitempty"`
	SiteID   string `json:"siteId,omitempty"`
	Name     string `json:"name"`
	Type     string `json:"type"`
	Status   string `json:"status,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`

	// Newt-specific fields
	NewtID        string `json:"newtId,omitempty"`
	NewtSecretKey string `json:"newtSecretKey,omitempty"`
}

// Resource represents a Pangolin resource
type Resource struct {
	ID          string `json:"id,omitempty"`
	ResourceID  string `json:"resourceId,omitempty"`
	Name        string `json:"name"`
	SiteID      int    `json:"siteId"`
	HTTP        bool   `json:"http"`
	Protocol    string `json:"protocol"`
	Subdomain   string `json:"subdomain,omitempty"`
	DomainID    string `json:"domainId,omitempty"`
	ProxyPort   int32  `json:"proxyPort,omitempty"`
	EnableProxy bool   `json:"enableProxy,omitempty"`
	Enabled     bool   `json:"enabled,omitempty"`
}

// ResourceCreateSpec defines the spec for creating a resource
type ResourceCreateSpec struct {
	Name        string `json:"name"`
	SiteID      int    `json:"siteId"`
	HTTP        bool   `json:"http"`
	Protocol    string `json:"protocol"`
	Subdomain   string `json:"subdomain,omitempty"`
	DomainID    string `json:"domainId,omitempty"`
	ProxyPort   int32  `json:"proxyPort,omitempty"`
	EnableProxy bool   `json:"enableProxy,omitempty"`
}

// Target represents a Pangolin target
type Target struct {
	ID      string `json:"id,omitempty"`
	IP      string `json:"ip"`
	Port    int32  `json:"port"`
	Method  string `json:"method"`
	Enabled bool   `json:"enabled"`
}

// TargetCreateSpec defines the spec for creating a target
type TargetCreateSpec struct {
	IP      string `json:"ip"`
	Port    int32  `json:"port"`
	Method  string `json:"method"`
	Enabled bool   `json:"enabled"`
}
