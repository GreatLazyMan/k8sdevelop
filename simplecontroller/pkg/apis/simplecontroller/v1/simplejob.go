package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

const (
	SimpleJobKind string = "SimpleJob"
)

// +genclient
// +kubebuilder:resource:scope="Namespaced",categories=clusternet
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:printcolumn:name="Replica",type=string,JSONPath=`.spec.replica`

type SimpleJob struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	// Spec is the specification for the behaviour of the real cluster.
	Spec SimpleJobSpec `json:"spec"`

	// Status describes the current status of a real cluster.
	// +optional
	Status SimpleJobStatus `json:"status"`
}

type SimpleJobSpec struct {
	// +optional
	Command []string `json:"command,omitempty"`
	// +kubebuilder:default=1
	// +optional
	Replica int `json:"replica,omitempty"`
	// +optional
	Image string `json:"image,omitempty"`
}

type SimpleJobStatus struct {
	// SubStatus contain some information
	// +optional
	SubStatus SubStatus `json:"subStatus,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

type SimpleJobList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`
	Items           []SimpleJob `json:"items"`
}
