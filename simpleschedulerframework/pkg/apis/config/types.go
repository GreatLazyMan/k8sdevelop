package config

import metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// FooPluginArgs holds arguments used to configure FooPlugin plugin.
type FooPluginArgs struct {
	metav1.TypeMeta `json:",inline"`

	DeploymentName string `json:"deploymentName,omitempty"`
	Image          string `json:"image,omitempty"`
}