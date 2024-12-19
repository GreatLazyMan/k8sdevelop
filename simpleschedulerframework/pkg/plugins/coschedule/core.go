package coschedule

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/klog/v2"

	simpleiov1 "github.com/GreatLazyMan/simplescheduler/pkg/apis/crd/simpleio/v1"
)

func (sp *CoschedulePlugin) CheckPodNum(pg *simpleiov1.PodGroup, pod *v1.Pod) error {
	pgPods, err := sp.podInformer.Lister().Pods(pg.Namespace).List(
		labels.SelectorFromSet(labels.Set{LabelKey: pg.Name}),
	)
	if err != nil {
		return err
	}
	if len(pgPods) < int(pg.Spec.MinReplicas) {
		klog.Warningf("pg has %d pods, less than %d ", len(pgPods), pg.Spec.MinReplicas)
		return fmt.Errorf("pg has %d pods, less than %d ", len(pgPods), pg.Spec.MinReplicas)
	} else {
		pgNumber := len(pgPods)
		for _, pgPod := range pgPods {
			if pgPod.GetDeletionTimestamp() != nil {
				pgNumber -= 1
				klog.Infof("pod %s is deleteing", pgPod.Name)
			}
		}
		if pgNumber < int(pg.Spec.MinReplicas) {
			klog.Warningf("pg has %d pods, less than %d ", pgNumber, pg.Spec.MinReplicas)
			return fmt.Errorf("pg has %d pods, less than %d ", pgNumber, pg.Spec.MinReplicas)
		}
	}
	return nil
}

func (sp *CoschedulePlugin) GetPodGroupNameByLabel(pod *v1.Pod) string {
	if pod.Labels != nil {
		if pgName, ok := pod.Labels[LabelKey]; ok {
			return pgName
		}
	}
	return ""
}

func (sp *CoschedulePlugin) GetPodGroupFullName(pod *v1.Pod) string {
	pgName := sp.GetPodGroupNameByLabel(pod)
	if len(pgName) == 0 {
		return ""
	}
	return fmt.Sprintf("%v/%v", pod.Namespace, pgName)
}

// GetNamespacedName returns the namespaced name.
func (sp *CoschedulePlugin) GetNamespacedName(obj metav1.Object) string {
	return fmt.Sprintf("%v/%v", obj.GetNamespace(), obj.GetName())
}

func (sp *CoschedulePlugin) GetPodGroup(ctx context.Context, pod *v1.Pod) (*simpleiov1.PodGroup, error) {
	pg := &simpleiov1.PodGroup{}
	if pod.Labels != nil {
		if pgName, ok := pod.Labels[LabelKey]; ok {
			if len(pgName) == 0 {
				return nil, fmt.Errorf("pod group name is null")
			}
			pgTypes := types.NamespacedName{
				Name:      pgName,
				Namespace: pod.Namespace,
			}

			err := sp.client.Get(ctx, pgTypes, pg)
			if err != nil {
				klog.Errorf("pods is %s/%s, get pg %s error: %v", pod.Namespace, pod.Name,
					pgName, err)
				if errors.IsNotFound(err) {
					return nil, fmt.Errorf("pod not found")
				} else {
					return nil, err
				}
			}
		}
	}
	return pg, nil
}

// CalculateAssignedPods returns the number of pods that has been assigned nodes: assumed or bound.
func (sp *CoschedulePlugin) CalculateAssignedPods(ctx context.Context, podGroupName, namespace string) int {
	lh := klog.FromContext(ctx)
	nodeInfos, err := sp.snapshoLister.NodeInfos().List()
	if err != nil {
		lh.Error(err, "Cannot get nodeInfos from frameworkHandle")
		return 0
	}
	var count int
	for _, nodeInfo := range nodeInfos {
		for _, podInfo := range nodeInfo.Pods {
			pod := podInfo.Pod
			if pod.Labels != nil && pod.Labels[LabelKey] == podGroupName {
				if pod.Namespace == namespace && pod.Spec.NodeName != "" {
					count++
				}
			}
		}
	}
	return count
}
