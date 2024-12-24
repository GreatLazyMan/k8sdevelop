package v1

import (
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"

	//schedconfig "k8s.io/kubernetes/pkg/scheduler/apis/config"
	schedconfig "k8s.io/kube-scheduler/config/v1"
)

const GroupName = "kubescheduler.config.k8s.io"

// SchemeGroupVersion is group version used to register these objects
var SchemeGroupVersion = schema.GroupVersion{Group: GroupName, Version: "v1"}

var (
	localSchemeBuilder = &schedconfig.SchemeBuilder
	// AddToScheme is a global function that registers this API group & version to a scheme
	AddToScheme = localSchemeBuilder.AddToScheme
)

// Adds the list of known types to Scheme.
func addKnownTypes(scheme *runtime.Scheme) error {
	scheme.AddKnownTypes(SchemeGroupVersion,
		&CoschedulePluginArgs{},
	)
	return nil
}

func init() {
	localSchemeBuilder.Register(addKnownTypes)
	localSchemeBuilder.Register(RegisterDefaults)
	localSchemeBuilder.Register(RegisterConversions)
}
