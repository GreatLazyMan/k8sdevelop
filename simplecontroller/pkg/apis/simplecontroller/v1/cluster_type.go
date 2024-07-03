package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="IP_FAMILY",type=string,JSONPath=`.spec.Options.ipFamily`

type Cluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the specification for the behaviour of the cluster.
	Spec ClusterSpec `json:"spec"`

	// Status describes the current status of a cluster.
	// +optional
	Status ClusterStatus `json:"status"`
}

type ClusterSpec struct {
	// +optional
	Kubeconfig []byte `json:"kubeconfig,omitempty"`

	// +kubebuilder:default=greatlazyman-system
	// +optional
	Namespace string `json:"namespace"`

	// +optional
	Options *Options `json:"options,omitempty"`
}

type ClusterStatus struct {
	// SubStatus contain some information
	// +optional
	SubStatus SubStatus `json:"subStatus,omitempty"`
}

type Options struct {
	// +kubebuilder:default=true
	// +optional
	Enable bool `json:"enable"`

	// +kubebuilder:default=calico
	// +optional
	CNI string `json:"cni"`

	// +kubebuilder:default=all
	// +optional
	IPFamily IPFamilyType `json:"ipFamily"`

	// +optional
	CIDRSlice []string `json:"crdslice,omitempty"`
}

type IPFamilyType string

type SubStatus struct {
	// +optional
	PodCIDRs []string `json:"podCIDRs,omitempty"`
	// +optional
	ServiceCIDRs []string `json:"serviceCIDRs,omitempty"`
}


// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type ClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []Cluster `json:"items"`
}
