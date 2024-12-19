package coschedule

import (
	"context"
	"fmt"
	"time"

	gocache "github.com/patrickmn/go-cache"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	informersv1 "k8s.io/client-go/informers/core/v1"
	clientscheme "k8s.io/client-go/kubernetes/scheme"
	"k8s.io/client-go/tools/cache"
	"k8s.io/klog/v2"
	"k8s.io/kubernetes/pkg/scheduler/framework"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/GreatLazyMan/simplescheduler/pkg/apis/config"
	simpleiov1 "github.com/GreatLazyMan/simplescheduler/pkg/apis/crd/simpleio/v1"
)

const (
	Name            = "CoschedulePlugin"
	AnnotationKey   = "simple.io/pod-group"
	LabelKey        = "simple.io/pod-group"
	PgBackoffTime   = 7 * time.Second
	permitStateKey  = "PermitCoscheduling"
	DefaultWaitTime = 3 * time.Second
)

type CoschedulePlugin struct {
	handle        framework.Handle
	client        client.Client
	podInformer   informersv1.PodInformer
	snapshoLister framework.SharedLister
	permittedPG   *gocache.Cache
	backedOffPG   *gocache.Cache
}

type PermitState struct {
	Activate bool
}

func (s *PermitState) Clone() framework.StateData {
	return &PermitState{Activate: s.Activate}
}

func (sp *CoschedulePlugin) Name() string {
	return Name
}

func (sp *CoschedulePlugin) PreFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod) (*framework.PreFilterResult, *framework.Status) {
	klog.Infof("prefilter pod: %v", pod.Name)
	pgFullName := sp.GetPodGroupFullName(pod)
	if _, exist := sp.backedOffPG.Get(pgFullName); exist {
		return nil, framework.NewStatus(framework.Unschedulable,
			fmt.Sprintf("podGroup %v failed recently", pgFullName))
	}

	pg, err := sp.GetPodGroup(ctx, pod)
	if err != nil {
		return nil, framework.NewStatus(framework.Unschedulable, err.Error())
	}
	if pg == nil {
		return nil, framework.NewStatus(framework.Unschedulable, "pg is nil")
	}
	if _, ok := sp.permittedPG.Get(pgFullName); ok {
		queue := &simpleiov1.Queue{}
		if pg.Spec.Queue == nil {
			err = sp.client.Get(ctx, types.NamespacedName{Name: "default"}, queue)
		} else {
			err = sp.client.Get(ctx, types.NamespacedName{Name: *pg.Spec.Queue}, queue)
		}
		if err != nil {
			klog.Errorf("get queue error: %v", err)
			return nil, framework.NewStatus(framework.UnschedulableAndUnresolvable, err.Error())
		}

		err = sp.CheckPodNum(pg, pod)
		if err != nil {
			klog.Errorf("check pod number err: %v", err)
			return nil, framework.NewStatus(framework.UnschedulableAndUnresolvable, err.Error())
		}
	} else {
		sp.permittedPG.Add(pgFullName, pg.Name, 4*time.Second)
	}

	return nil, framework.NewStatus(framework.Success, "")
}

func (sp *CoschedulePlugin) PreFilterExtensions() framework.PreFilterExtensions {
	klog.Infof("PreFilterExtensions")
	return nil
}

func (sp *CoschedulePlugin) Filter(ctx context.Context, state *framework.CycleState, pod *v1.Pod,
	nodeInfo *framework.NodeInfo) *framework.Status {
	node := nodeInfo.Node()
	if node == nil {
		return framework.NewStatus(framework.Error, "node not found")
	}
	klog.Infof("filter plugin check, pod: %v, node: %v", pod.Name, node.Name)
	return framework.NewStatus(framework.Success, "")
}

// PostFilter is used to reject a group of pods if a pod does not pass PreFilter or Filter.
func (sp *CoschedulePlugin) PostFilter(ctx context.Context, state *framework.CycleState, pod *v1.Pod,
	filteredNodeStatusMap framework.NodeToStatusMap) (*framework.PostFilterResult, *framework.Status) {
	lh := klog.FromContext(ctx)
	klog.Infof("PostFilter plugin for pod %s", pod.Name)
	for node, status := range filteredNodeStatusMap {
		klog.Infof("node is %s, status is %v", node, status)
	}
	pg, err := sp.GetPodGroup(ctx, pod)
	if err != nil {
		return &framework.PostFilterResult{}, framework.NewStatus(framework.Unschedulable, err.Error())
	}
	if len(pg.Name) > 0 {
		assignedPodNum := sp.CalculateAssignedPods(ctx, pg.Name, pod.Namespace)
		if assignedPodNum > int(pg.Spec.MinReplicas) {
			klog.Info("Assigned pods", "podGroup", klog.KObj(pg), "assigned", assignedPodNum)
			return &framework.PostFilterResult{}, framework.NewStatus(framework.Unschedulable)
		}
	}

	// It's based on an implicit assumption: if the nth Pod failed,
	// it's inferrable other Pods belonging to the same PodGroup would be very likely to fail.
	sp.handle.IterateOverWaitingPods(func(waitingPod framework.WaitingPod) {
		if waitingPod.GetPod().Namespace == pod.Namespace &&
			sp.GetPodGroupNameByLabel(waitingPod.GetPod()) == pg.Name {
			lh.Info("PostFilter rejects the pod", "podGroup", klog.KObj(pg), "pod",
				klog.KObj(waitingPod.GetPod()))
			waitingPod.Reject(sp.Name(), "optimistic rejection in PostFilter")
		}
	})
	// backoff
	pods, err := sp.handle.SharedInformerFactory().Core().V1().Pods().Lister().Pods(pod.Namespace).List(
		labels.SelectorFromSet(labels.Set{LabelKey: pg.Name}),
	)

	pgFullName := sp.GetPodGroupFullName(pod)
	if err == nil && len(pods) >= int(pg.Spec.MinReplicas) {
		sp.backedOffPG.Add(pgFullName, nil, PgBackoffTime)
	}

	sp.permittedPG.Delete(pgFullName)

	return &framework.PostFilterResult{}, framework.NewStatus(framework.Unschedulable)
}

func (sp *CoschedulePlugin) Permit(ctx context.Context, state *framework.CycleState,
	pod *v1.Pod, nodeName string) (*framework.Status, time.Duration) {
	klog.Infof("Permit plugin pods %s node %s", pod.Name, nodeName)
	pg, err := sp.GetPodGroup(ctx, pod)
	if err != nil {
		klog.Errorf("get pod group err: %v", err)
		return framework.NewStatus(framework.Unschedulable, err.Error()), 0
	}

	assignedPodNum := sp.CalculateAssignedPods(ctx, pg.Name, pod.Namespace)
	klog.Infof("assignedPodNum is %d", assignedPodNum)
	if assignedPodNum+1 >= int(pg.Spec.MinReplicas) {
		klog.Info("Assigned pods ", "podGroup:", klog.KObj(pg), "assigned: ", assignedPodNum)
		sp.handle.IterateOverWaitingPods(func(waitingPod framework.WaitingPod) {
			if sp.GetPodGroupFullName(waitingPod.GetPod()) == fmt.Sprintf("%v/%v", pod.Namespace, pg.Name) {
				klog.Info("Permit allows", "pod", klog.KObj(waitingPod.GetPod()))
				waitingPod.Allow(sp.Name())
			}
		})
		return framework.NewStatus(framework.Success), 0
	}
	if assignedPodNum == 0 {
		state.Write(permitStateKey, &PermitState{Activate: true})
	}
	// Only proceed if it's explicitly requested to activate sibling pods.
	if c, err := state.Read(permitStateKey); err != nil {
	} else if s, ok := c.(*PermitState); !ok || !s.Activate {
	}
	pods, err := sp.podInformer.Lister().Pods(pod.Namespace).List(
		labels.SelectorFromSet(labels.Set{LabelKey: pg.Name}),
	)
	if err != nil {
		klog.Error(err, "Failed to obtain pods belong to a PodGroup", "podGroup", pg.Name)
	}

	for i := range pods {
		if pods[i].UID == pod.UID {
			pods = append(pods[:i], pods[i+1:]...)
			break
		}
	}
	if len(pods) != 0 {
		if c, err := state.Read(framework.PodsToActivateKey); err == nil {
			if s, ok := c.(*framework.PodsToActivate); ok {
				s.Lock()
				for _, pod := range pods {
					namespacedName := sp.GetNamespacedName(pod)
					klog.Infof("PodsToActivate, pods is %s", pod.Name)
					s.Map[namespacedName] = pod
				}
				s.Unlock()
			}
		}
	}

	klog.Infof("pod %s need wait", pod.Name)
	return framework.NewStatus(framework.Wait, ""), DefaultWaitTime
}

func (sp *CoschedulePlugin) PreBind(ctx context.Context, state *framework.CycleState,
	pod *v1.Pod, nodeName string) *framework.Status {
	klog.Infof("prebind node info: %+v", nodeName)
	if state != nil {
		data, err := state.Read(permitStateKey)
		if err != nil {
			klog.Errorf("read data error: %v", err)
		} else {
			klog.Infof("pod is %s, state is %v", pod.Name, data)
		}
	}
	return framework.NewStatus(framework.Success, "")
}

// Reserve is the functions invoked by the framework at "reserve" extension point.
func (sp *CoschedulePlugin) Reserve(ctx context.Context, state *framework.CycleState,
	pod *v1.Pod, nodeName string) *framework.Status {
	klog.Infof("Reserve plugin pods %s node %s", pod.Name, nodeName)
	return nil
}

// Unreserve rejects all other Pods in the PodGroup when one of the pods in the group times out.
func (sp *CoschedulePlugin) Unreserve(ctx context.Context, state *framework.CycleState,
	pod *v1.Pod, nodeName string) {
	klog.Infof("Unreserve plugin pods %s node %s", pod.Name, nodeName)
	lh := klog.FromContext(ctx)
	pg, err := sp.GetPodGroup(ctx, pod)
	if err == nil {
		return
	}
	sp.handle.IterateOverWaitingPods(func(waitingPod framework.WaitingPod) {
		if waitingPod.GetPod().Namespace == pod.Namespace && waitingPod.GetPod().Labels[LabelKey] == pg.Name {
			lh.V(3).Info("Unreserve rejects", "pod", klog.KObj(waitingPod.GetPod()), "podGroup", klog.KObj(pg))
			waitingPod.Reject(sp.Name(), "rejection in Unreserve")
		}
	})
	sp.permittedPG.Delete(pg.Name)
}

// type PluginFactory = func(configuration *runtime.Unknown, f FrameworkHandle) (Plugin, error)
func New(_ context.Context, plArgs runtime.Object, f framework.Handle) (framework.Plugin, error) {
	args, ok := plArgs.(*config.CoschedulePluginArgs)
	if !ok {
		if plArgs == nil {
			klog.Warning("plArgs is nil")
		} else {
			xx := plArgs.(*runtime.Unknown)
			return nil, fmt.Errorf("expected PluginConfig, got %T,raw: %s", xx, string(xx.Raw))
		}
	} else if ok {
		klog.Infof("get plugin config args: %s", args.GetArgs())
	}
	scheme := runtime.NewScheme()
	_ = clientscheme.AddToScheme(scheme)
	_ = v1.AddToScheme(scheme)
	_ = simpleiov1.Install(scheme)
	client, err := client.New(f.KubeConfig(), client.Options{Scheme: scheme})
	if err != nil {
		return nil, err
	}
	f.SharedInformerFactory().Core().V1().Pods().Informer().AddIndexers(
		cache.Indexers{cache.NamespaceIndex: cache.MetaNamespaceIndexFunc},
	)

	return &CoschedulePlugin{
		handle:        f,
		client:        client,
		podInformer:   f.SharedInformerFactory().Core().V1().Pods(),
		snapshoLister: f.SnapshotSharedLister(),
		permittedPG:   gocache.New(3*time.Second, 3*time.Second),
		backedOffPG:   gocache.New(10*time.Second, 10*time.Second),
	}, nil
}
