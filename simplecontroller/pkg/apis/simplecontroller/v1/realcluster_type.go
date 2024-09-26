package v1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

const (
	RealClusterKind string = "RealCluster"
)

// +genclient
// +genclient:nonNamespaced
// +kubebuilder:resource:scope="Cluster"
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="IP_FAMILY",type=string,JSONPath=`.spec.Options.ipFamily`

type RealCluster struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the specification for the behaviour of the real cluster.
	Spec RealClusterSpec `json:"spec"`

	// Status describes the current status of a real cluster.
	// +optional
	Status RealClusterStatus `json:"status"`
}

type RealClusterSpec struct {
	// +optional
	Kubeconfig []byte `json:"kubeconfig,omitempty"`

	// +kubebuilder:default=greatlazyman-system
	// +optional
	Namespace string `json:"namespace"`
}

type RealClusterStatus struct {
	// SubStatus contain some information
	// +optional
	SubStatus SubStatus `json:"subStatus,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type RealClusterList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []RealCluster `json:"items"`
}
