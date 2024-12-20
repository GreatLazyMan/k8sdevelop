package coschedule

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	clientscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/GreatLazyMan/simplescheduler/pkg/apis/config"
	simpleiov1 "github.com/GreatLazyMan/simplescheduler/pkg/apis/crd/simpleio/v1"
)

const Name = "CoschedulePlugin"

type CoschedulePlugin struct {
	handle framework.Handle
	client client.Client
}

func (s *CoschedulePlugin) Name() string {
	return Name
}

func (s *CoschedulePlugin) PreFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod) (*framework.PreFilterResult, *framework.Status) {
	klog.Infof("prefilter pod: %v", pod.Name)
	return nil, framework.NewStatus(framework.Success, "")
}

func (s *CoschedulePlugin) PreFilterExtensions() framework.PreFilterExtensions {
	klog.Infof("PreFilterExtensions")
	return nil
}

func (s *CoschedulePlugin) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	node := nodeInfo.Node()
	if node == nil {
		return framework.NewStatus(framework.Error, "node not found")
	}
	klog.Infof("filter pod: %v, node: %v", pod.Name, node.Name)
	return framework.NewStatus(framework.Success, "")
}

func (s *CoschedulePlugin) PreBind(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) *framework.Status {
	klog.Infof("prebind node info: %+v", nodeName)
	return framework.NewStatus(framework.Success, "")
}

// type PluginFactory = func(configuration *runtime.Unknown, f FrameworkHandle) (Plugin, error)
func New(_ context.Context, plArgs runtime.Object, f framework.Handle) (framework.Plugin, error) {
	args, ok := plArgs.(*config.SimplePluginArgs)
	if !ok {
		xx := plArgs.(*runtime.Unknown)
		return nil, fmt.Errorf("expected PluginConfig, got %T,raw: %s", xx, string(xx.Raw))
	}
	scheme := runtime.NewScheme()
	_ = clientscheme.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)
	_ = simpleiov1.Install(scheme)
	client, err := client.New(f.KubeConfig(), client.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}

	klog.Infof("get plugin config args: %+v", args)
	return &CoschedulePlugin{
		handle: f,
		client: client,
	}, nil
}
