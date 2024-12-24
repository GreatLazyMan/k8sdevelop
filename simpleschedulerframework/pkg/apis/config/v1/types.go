package v1

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object
// +k8s:defaulter-gen=true

type CoschedulePluginArgs struct {
	metav1.TypeMeta `json:",inline"`

	DeploymentName *string `json:"deploymentName,omitempty"`
	Image          *string `json:"image,omitempty"`
}
