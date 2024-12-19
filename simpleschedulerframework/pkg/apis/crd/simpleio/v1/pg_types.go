package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +genclient
// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +kubebuilder:object:root=true
// +kubebuilder:resource:shortName={pg,pgs}
// +kubebuilder:subresource:status
// +kubebuilder:metadata:annotations="api-approved.kubernetes.io=https://github.com/kubernetes-sigs/scheduler-plugins/pull/50"
// +kubebuilder:printcolumn:name="Phase",JSONPath=".status.phase",type=string,description="Current phase of PodGroup."
// +kubebuilder:printcolumn:name="MinReplicas",JSONPath=".spec.minReplicas",type=integer,description="MinMember defines the minimal number of members/tasks to run the pod group."

// Bar is a specification for a Bar resource
type PodGroup struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec PodGroupSpec `json:"spec"`
	// +optional
	Status PodGroupSpec `json:"status"`
}

// PodGroupSpec is the spec for a PodGroup resource
type PodGroupSpec struct {
	// +kubebuilder:validation:Minimum=1
	MinReplicas int32 `json:"minReplicas"`
	// +optional
	PodLabels map[string]string `json:"podLabels"`
	// +optional
	Queue *string `json:"queue"`
}

// PodGroupStatus is the status for a PodGroup resource
type PodGroupStatus struct {
	// +optional
	Phase string `json:"phase,omitempty"`
}

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// PodGroupList is a list of Bar resources
type PodGroupList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata"`

	Items []PodGroup `json:"items"`
}
