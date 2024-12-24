// +k8s:defaulter-gen=true

package v1

import "k8s.io/utils/ptr"

// v1beta1 refers to k8s.io/kube-scheduler/config/v1 (package is in staging/src)
func SetDefaults_CoschedulePluginArgs(obj *CoschedulePluginArgs) {
	if obj.DeploymentName == nil {
		obj.DeploymentName = ptr.To("")
	}
	if obj.Image == nil {
		obj.Image = ptr.To("")
	}
}
