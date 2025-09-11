package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// PangolinBindingSpec defines the desired state of PangolinBinding
type PangolinBindingSpec struct {
	// Reference to the Kubernetes Service to expose
	// +kubebuilder:validation:Required
	ServiceRef ServiceReference `json:"serviceRef"`

	// Reference to the organization to use
	// +kubebuilder:validation:Required
	OrganizationRef LocalObjectReference `json:"organizationRef"`

	// Optional: Reference to specific tunnel
	// If not specified, will use/create default tunnel for the organization
	TunnelRef *LocalObjectReference `json:"tunnelRef,omitempty"`

	// Protocol type
	// +kubebuilder:validation:Enum=http;tcp;udp
	// +kubebuilder:validation:Required
	Protocol string `json:"protocol"`

	// Port on the service to expose
	// +kubebuilder:validation:Minimum=1
	// +kubebuilder:validation:Maximum=65535
	ServicePort int32 `json:"servicePort"`

	// HTTP-specific configuration
	HTTPConfig *HTTPConfig `json:"httpConfig,omitempty"`

	// TCP/UDP-specific configuration
	ProxyConfig *ProxyConfig `json:"proxyConfig,omitempty"`

	// Auto-update targets based on Service endpoints
	// +kubebuilder:default=true
	AutoUpdateTargets *bool `json:"autoUpdateTargets,omitempty"`
}

// ServiceReference contains enough information to locate a service
type ServiceReference struct {
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// +kubebuilder:validation:Required
	Namespace string `json:"namespace"`
}

// PangolinBindingStatus defines the observed state of PangolinBinding
type PangolinBindingStatus struct {
	// Generated resource name
	GeneratedResourceName string `json:"generatedResourceName,omitempty"`

	// Public URL for HTTP resources
	URL string `json:"url,omitempty"`

	// Proxy endpoint for TCP/UDP resources
	ProxyEndpoint string `json:"proxyEndpoint,omitempty"`

	// Service endpoints currently being targeted
	ServiceEndpoints []string `json:"serviceEndpoints,omitempty"`

	// Current status: Creating, Ready, Error, Updating
	// +kubebuilder:validation:Enum=Creating;Ready;Error;Updating
	Status string `json:"status,omitempty"`

	// Conditions represent the latest available observations
	Conditions []metav1.Condition `json:"conditions,omitempty"`

	// ObservedGeneration reflects the generation most recently observed
	ObservedGeneration int64 `json:"observedGeneration,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status
//+kubebuilder:resource:shortName=pbinding
//+kubebuilder:printcolumn:name="Service",type=string,JSONPath=`.spec.serviceRef.name`
//+kubebuilder:printcolumn:name="Protocol",type=string,JSONPath=`.spec.protocol`
//+kubebuilder:printcolumn:name="URL",type=string,JSONPath=`.status.url`
//+kubebuilder:printcolumn:name="Status",type=string,JSONPath=`.status.status`
//+kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// PangolinBinding is the Schema for the pangolinbindings API
type PangolinBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   PangolinBindingSpec   `json:"spec,omitempty"`
	Status PangolinBindingStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// PangolinBindingList contains a list of PangolinBinding
type PangolinBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []PangolinBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&PangolinBinding{}, &PangolinBindingList{})
}
