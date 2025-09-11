package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PangolinTunnelSpec defines the desired state of PangolinTunnel
type PangolinTunnelSpec struct {
	// Reference to the organization
	// +kubebuilder:validation:Required
	OrganizationRef LocalObjectReference `json:"organizationRef"`

	// Site configuration for NEW sites
	SiteName string `json:"siteName,omitempty"`
	SiteType string `json:"siteType,omitempty"`

	// BINDING MODE: Bind to existing site using EITHER field
	// Numeric site ID (e.g., 3)
	SiteID *int `json:"siteId,omitempty"`

	// OR nice ID (e.g., "impractical-oriental-wolf-snake")
	NiceID string `json:"niceId,omitempty"`

	// Newt client configuration (overrides org defaults)
	NewtClient *NewtClientSpec `json:"newtClient,omitempty"`

	// Custom configuration
	Config map[string]string `json:"config,omitempty"`
}

// PangolinTunnelStatus defines the observed state of PangolinTunnel
type PangolinTunnelStatus struct {
	// Site information from Pangolin API (all populated)
	SiteID   int    `json:"siteId,omitempty"`
	NiceID   string `json:"niceId,omitempty"`
	SiteName string `json:"siteName,omitempty"`
	SiteType string `json:"siteType,omitempty"`

	// Network information from API
	Subnet  string `json:"subnet,omitempty"`
	Address string `json:"address,omitempty"`

	// Connection status from API
	Online   bool   `json:"online,omitempty"`
	Endpoint string `json:"endpoint,omitempty"`

	// Newt-specific fields from API
	NewtID        string `json:"newtId,omitempty"`
	NewtSecretRef string `json:"newtSecretRef,omitempty"`

	// Binding mode: "Created" or "Bound"
	BindingMode string `json:"bindingMode,omitempty"`

	// Deployment status
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Current status
	Status             string             `json:"status,omitempty"`
	Conditions         []metav1.Condition `json:"conditions,omitempty"`
	ObservedGeneration int64              `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=ptunnel
//+kubebuilder:printcolumn:name="Site ID",type=integer,JSONPath=`.status.siteId`
//+kubebuilder:printcolumn:name="Nice ID",type=string,JSONPath=`.status.niceId`
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`
//+kubebuilder:printcolumn:name="Binding Mode",type=string,JSONPath=`.status.bindingMode`
//+kubebuilder:printcolumn:name="Online",type=boolean,JSONPath=`.status.online`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// PangolinTunnel is the Schema for the pangolintunnel API
type PangolinTunnel struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PangolinTunnelSpec   `json:"spec,omitempty"`
	Status PangolinTunnelStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PangolinTunnelList contains a list of PangolinTunnel
type PangolinTunnelList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PangolinTunnel `json:"items"`
}

type NewtClientSpec struct {
	Enabled  bool   `json:"enabled,omitempty"`
	Replicas *int32 `json:"replicas,omitempty"`
	Image    string `json:"image,omitempty"`
}

func init() {
	SchemeBuilder.Register(&PangolinTunnel{}, &PangolinTunnelList{})
}
