package scheme

import (
	apisixv2 "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v2"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	simplecontrollerv1 "github.com/GreatLazyMan/simplecontroller/pkg/apis/simplecontroller/v1"
)

// aggregatedScheme aggregates Kubernetes and extended schemes.
var aggregatedScheme = runtime.NewScheme()

func init() {
	err := scheme.AddToScheme(aggregatedScheme) // add Kubernetes schemes
	if err != nil {
		panic(err)
	}
	err = simplecontrollerv1.Install(aggregatedScheme) // add Kubernetes schemes
	if err != nil {
		panic(err)
	}
	err = apisixv2.Install(aggregatedScheme) // add Kubernetes schemes
	if err != nil {
		panic(err)
	}
}

// NewSchema returns a singleton schema set which aggregated Kubernetes's schemes and extended schemes.
func NewSchema() *runtime.Scheme {
	return aggregatedScheme
}

// NewForConfig creates a new client for the given config.
func NewForConfig(config *rest.Config) (client.Client, error) {
	return client.New(config, client.Options{
		Scheme: aggregatedScheme,
	})
}

// NewForConfigOrDie creates a new client for the given config and
// panics if there is an error in the config.
func NewForConfigOrDie(config *rest.Config) client.Client {
	c, err := NewForConfig(config)
	if err != nil {
		panic(err)
	}
	return c
}
