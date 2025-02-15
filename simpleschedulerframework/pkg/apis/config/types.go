package config

import (
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// +k8s:deepcopy-gen:interfaces=k8s.io/apimachinery/pkg/runtime.Object

// CoschedulePluginArgs holds arguments used to configure CoschedulePlugin plugin.
type CoschedulePluginArgs struct {
	metav1.TypeMeta `json:",inline"`

	DeploymentName string `json:"deploymentName,omitempty"`
	Image          string `json:"image,omitempty"`
}

func (c *CoschedulePluginArgs) GetArgs() string {
	return fmt.Sprintf("%s-%s", c.DeploymentName, c.Image)
}
