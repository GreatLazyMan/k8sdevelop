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

const Name = "SimplePlugin"

type SimplePlugin struct {
	handle framework.Handle
}

func (s *SimplePlugin) Name() string {
	return Name
}

func (s *SimplePlugin) PreFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod) (*framework.PreFilterResult, *framework.Status) {
	klog.Infof("prefilter pod: %v", pod.Name)
	return nil, framework.NewStatus(framework.Success, "")
}

func (s *SimplePlugin) PreFilterExtensions() framework.PreFilterExtensions {
	klog.Infof("PreFilterExtensions")
	return nil
}

func (s *SimplePlugin) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeInfo *framework.NodeInfo) *framework.Status {
	node := nodeInfo.Node()
	if node == nil {
		return framework.NewStatus(framework.Error, "node not found")
	}
	klog.Infof("filter pod: %v, node: %v", pod.Name, node.Name)
	return framework.NewStatus(framework.Success, "")
}

func (s *SimplePlugin) PreBind(ctx context.Context, state *framework.CycleState, pod *v1.Pod, nodeName string) *framework.Status {
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
	klog.Infof("get plugin config args: %+v", args)
	return &SimplePlugin{
		handle: f,
	}, nil
}
