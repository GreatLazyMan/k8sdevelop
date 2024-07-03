package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	VirtualClusterKind string = "VirtualCluster"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="IP_FAMILY",type=string,JSONPath=`.spec.Options.ipFamily`

type VirtualCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the specification for the behaviour of the virtual cluster.
	Spec VirtualClusterSpec `json:"spec"`

	// Status describes the current status of a virtual cluster.
	// +optional
	Status VirtualClusterStatus `json:"status"`
}

type VirtualClusterSpec struct {
	// +optional
	Kubeconfig []byte `json:"kubeconfig,omitempty"`

	// +kubebuilder:default=greatlazyman-system
	// +optional
	Namespace string `json:"namespace"`
}

type VirtualClusterStatus struct {
	// SubStatus contain some information
	// +optional
	SubStatus SubStatus `json:"subStatus,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type VirtualClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []VirtualCluster `json:"items"`
}
