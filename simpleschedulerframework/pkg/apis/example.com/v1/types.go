// pkg/crd.example.com/v1/types.go

package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// Bar is a specification for a Bar resource
type Bar struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec BarSpec `json:"spec"`
	// Status BarStatus `json:"status"`
}

// BarSpec is the spec for a Bar resource
type BarSpec struct {
	DeploymentName string `json:"deploymentName"`
	Image          string `json:"image"`
	Replicas       *int32 `json:"replicas"`
}

// BarStatus is the status for a Bar resource
type BarStatus struct {
	AvailableReplicas int32 `json:"availableReplicas"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// BarList is a list of Bar resources
type BarList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Bar `json:"items"`
}
