package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PangolinResourceSpec defines the desired state of PangolinResource
type PangolinResourceSpec struct {
	// Reference to the tunnel
	TunnelRef LocalObjectReference `json:"tunnelRef"`

	// Resource configuration
	Name     string `json:"name"`
	Protocol string `json:"protocol"` // "http", "tcp", "udp"

	// HTTP-specific fields
	Subdomain string `json:"subdomain,omitempty"`
	DomainID  string `json:"domainId,omitempty"`

	// TCP/UDP-specific fields
	ProxyPort   *int32 `json:"proxyPort,omitempty"`
	EnableProxy *bool  `json:"enableProxy,omitempty"`

	// Target configuration
	TargetIP     string `json:"targetIp"`
	TargetPort   int32  `json:"targetPort"`
	TargetMethod string `json:"targetMethod,omitempty"` // "http", "https", "tcp", "udp"

	// Optional: Load balancing
	Enabled *bool `json:"enabled,omitempty"`
}

// LocalObjectReference contains enough information to locate a resource within the same namespace
type LocalObjectReference struct {
	Name string `json:"name"`
}

// PangolinResourceStatus defines the observed state of PangolinResource
type PangolinResourceStatus struct {
	ResourceID string `json:"resourceId,omitempty"`
	TargetID   string `json:"targetId,omitempty"`
	Status     string `json:"status,omitempty"`
	URL        string `json:"url,omitempty"`

	// Condition tracking
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// Last observed generation
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=presource
//+kubebuilder:printcolumn:name="Resource ID",type=string,JSONPath=`.status.resourceId`
//+kubebuilder:printcolumn:name="Protocol",type=string,JSONPath=`.spec.protocol`
//+kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.status.url`
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// PangolinResource is the Schema for the pangolinresources API
type PangolinResource struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PangolinResourceSpec   `json:"spec,omitempty"`
	Status PangolinResourceStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PangolinResourceList contains a list of PangolinResource
type PangolinResourceList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PangolinResource `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PangolinResource{}, &PangolinResourceList{})
}
