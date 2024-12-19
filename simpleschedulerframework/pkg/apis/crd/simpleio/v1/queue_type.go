package v1

import (
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +genclient
// +genclient:nonNamespaced
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:scope=Cluster,shortName={q,qs}
// +kubebuilder:subresource:status
// +kubebuilder:metadata:annotations="api-approved.kubernetes.io=https://github.com/kubernetes-sigs/scheduler-plugins/pull/50"

// Bar is a specification for a Bar resource
type Queue struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec QueueSpec `json:"spec"`
	// +optional
	Status QueueSpec `json:"status"`
}

// QueueSpec is the spec for a Queue resource
type QueueSpec struct {
	// +optional
	ResourceLimits *corev1.ResourceList `json:"resourceLimits,omitempty"`
}

// QueueStatus is the status for a Queue resource
type QueueStatus struct {
	// State is status of queue
	State string `json:"state,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// QueueList is a list of Bar resources
type QueueList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []Queue `json:"items"`
}
