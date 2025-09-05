package v1alpha1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PangolinTunnelSpec defines the desired state of PangolinTunnel
type PangolinTunnelSpec struct {
	// Pangolin API configuration
	APIEndpoint string                   `json:"apiEndpoint"`
	APIKeyRef   corev1.SecretKeySelector `json:"apiKeyRef"`

	// Organization info (can be discovered from API)
	OrganizationID string `json:"organizationId,omitempty"`

	// Site configuration
	SiteName string `json:"siteName"`
	SiteType string `json:"siteType"` // "newt", "wireguard", "local"

	// Newt client configuration
	Replicas   *int32          `json:"replicas,omitempty"`
	Image      string          `json:"image,omitempty"` // Custom Newt image
	NewtClient *NewtClientSpec `json:"newtClient,omitempty"`

	// Optional: Advanced configuration
	Config map[string]string `json:"config,omitempty"`
}

// PangolinTunnelStatus defines the observed state of PangolinTunnel
type PangolinTunnelStatus struct {
	// Site information from Pangolin API
	SiteID string `json:"siteId,omitempty"`
	Status string `json:"status,omitempty"`

	// Connection details for Newt client
	Endpoint      string `json:"endpoint,omitempty"`
	NewtID        string `json:"newtId,omitempty"`
	NewtSecretRef string `json:"newtSecretRef,omitempty"`

	// Deployment status
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`

	// Condition tracking
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Last observed generation
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=ptunnel
//+kubebuilder:printcolumn:name="Site ID",type=string,JSONPath=`.status.siteId`
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`
//+kubebuilder:printcolumn:name="Replicas",type=integer,JSONPath=`.status.readyReplicas`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// PangolinTunnel is the Schema for the pangolintunel API
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

func init() {
	SchemeBuilder.Register(&PangolinTunnel{}, &PangolinTunnelList{})
}

type NewtClientSpec struct {
	Enabled  bool   `json:"enabled,omitempty"`
	Replicas *int32 `json:"replicas,omitempty"`
	Image    string `json:"image,omitempty"`
}
