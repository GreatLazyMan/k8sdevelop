package plugins

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"

	"github.com/GreatLazyMan/simplescheduler/pkg/apis/config"
)

const Name = "FooPlugin"

type FooPlugin struct {
	handle framework.Handle
}

func (s *FooPlugin) Name() string {
	return Name
}

func (s *FooPlugin) PreFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod) (*framework.PreFilterResult, *framework.Status) {
	klog.V(3).Infof("prefilter pod: %v", pod.Name)
	return nil, framework.NewStatus(framework.Success, "")
}

func (s *FooPlugin) PreFilterExtensions() framework.PreFilterExtensions {
	return nil
}

func (s *FooPlugin) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	node := nodeInfo.Node()
	if node == nil {
		return framework.NewStatus(framework.Error, "node not found")
	}
	klog.V(3).Infof("filter pod: %v, node: %v", pod.Name, node.Name)
	return framework.NewStatus(framework.Success, "")
}

func (s *FooPlugin) PreBind(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) *framework.Status {
	klog.V(3).Infof("prebind node info: %+v", nodeName)
	return framework.NewStatus(framework.Success, "")
}

// type PluginFactory = func(configuration *runtime.Unknown, f FrameworkHandle) (Plugin, error)
func New(_ context.Context, plArgs runtime.Object, f framework.Handle) (framework.Plugin, error) {
	args, ok := plArgs.(*config.FooPluginArgs)
	if !ok {
		xx := plArgs.(*runtime.Unknown)
		return nil, fmt.Errorf("expected PluginConfig, got %T,raw: %s", xx, string(xx.Raw))
	}
	klog.V(3).Infof("get plugin config args: %+v", args)
	return &FooPlugin{
		handle: f,
	}, nil
}
