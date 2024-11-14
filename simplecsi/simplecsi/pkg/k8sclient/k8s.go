package k8sclient

import (
	"context"
	"fmt"
	"time"

	corev1 "k8s.io/api/core/v1"

	v1 "k8s.io/api/core/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/klog/v2"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	apierror "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/util/wait"
)

const DefaultKubeletPath = "/var/lib/kubelet"

func GetKubernetesClient() (*kubernetes.Clientset, error) {
	config, err := rest.InClusterConfig()
	if err != nil {
		return nil, err
	}
	// 创建 clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, err
	}
	return clientset, nil
}

func CreateJob(ctx context.Context, clientset *kubernetes.Clientset,
	name, namespace, mountpath, nodeName, image string,
	cmd []string) (string, error) {
	time2Del := int32(120)
	useid := int64(0)
	privileged := true
	cmdNsenter := []string{
		"nsenter", "--target", "1", "--mount", "--uts", "--ipc", "--net", "--pid", "--",
	}
	cmdNsenter = append(cmdNsenter, cmd...)
	var lastErr error = nil
	job := &batchv1.Job{
		ObjectMeta: metav1.ObjectMeta{
			Name:      name,
			Namespace: namespace,
		},
		Spec: batchv1.JobSpec{
			TTLSecondsAfterFinished: &time2Del,
			Template: v1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{
					Labels: map[string]string{
						"app": name,
					},
				},
				Spec: v1.PodSpec{
					Containers: []v1.Container{
						{
							Name:    "linux",
							Image:   image,
							Command: cmdNsenter,
							SecurityContext: &corev1.SecurityContext{
								RunAsUser:  &useid,
								Privileged: &privileged,
							},
						},
					},
					NodeName:      nodeName,
					RestartPolicy: v1.RestartPolicyNever,
					HostNetwork:   true,
					HostPID:       true,
				},
			},
		},
	}

	jobsClient := clientset.BatchV1().Jobs(namespace)
	_, err := jobsClient.Create(ctx, job, metav1.CreateOptions{})
	if err != nil {
		return "", err
	}
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()

	for i := 0; i < 10; i += 1 {
		select {
		case <-ticker.C:
			jobNow, err := jobsClient.Get(ctx, name, metav1.GetOptions{})
			if err != nil {
				klog.Warningf("get job %s errorL %v", name, err)
				lastErr = err
				continue
			}
			if len(jobNow.Status.Conditions) > 0 &&
				jobNow.Status.Conditions[len(jobNow.Status.Conditions)-1].Type == batchv1.JobComplete {
				labelSelector := fmt.Sprintf("job-name=%s", name)
				pods, err := clientset.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
					LabelSelector: labelSelector,
				})
				if err != nil {
					klog.Warningf("Error listing pods: %v", err)
					lastErr = err
					continue
				}
				for _, pod := range pods.Items {
					if pod.Status.Phase == v1.PodSucceeded {
						nodeName = pod.Spec.NodeName
						//err := jobsClient.Delete(ctx, name, metav1.DeleteOptions{})
						//if err != nil {
						//	klog.Errorf("delete job %s err: %v", name, err)
						//}
						return nodeName, nil
					}
				}
			}
		case <-ctx.Done():
			break
		}
	}

	return nodeName, lastErr
}

// WaitPodReady wait pod ready.
func WaitPodReady(ctx context.Context, c kubernetes.Interface, namespace, selector string, timeout int) error {
	// Wait 3 second
	time.Sleep(1 * time.Second)
	pods, err := c.CoreV1().Pods(namespace).List(context.TODO(), metav1.ListOptions{
		LabelSelector: fmt.Sprintf("app=%s", selector)})
	if err != nil {
		return err
	}

	if len(pods.Items) == 0 {
		return fmt.Errorf("no pods in %s with selector %s", namespace, selector)
	}

	for _, pod := range pods.Items {
		if err = waitPodReady(ctx, c, namespace, pod.Name, time.Duration(timeout)*time.Second); err != nil {
			return err
		}
	}

	return nil
}

// waitPodReady  Poll up to timeout seconds for pod to enter running state.
// Returns an error if the pod never enters the running state.
func waitPodReady(ctx context.Context, c kubernetes.Interface, namespaces, podName string, timeout time.Duration) error {
	tmCtx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	return wait.PollUntilContextCancel(tmCtx, time.Second, true, isPodReady(c, namespaces, podName))
	// return wait.PollImmediate(time.Second, timeout, isPodReady(c, namespaces, podName))
}

func podStatus(pod *corev1.Pod) string {
	for _, value := range pod.Status.ContainerStatuses {
		if pod.Status.Phase == corev1.PodSucceeded {
			if value.State.Waiting != nil {
				return value.State.Waiting.Reason
			}
			if value.State.Waiting == nil {
				return string(corev1.PodSucceeded)
			}
			return "Error"
		}
		if pod.ObjectMeta.DeletionTimestamp != nil {
			return "Terminating"
		}
	}
	return pod.Status.ContainerStatuses[0].State.Waiting.Reason
}

func isPodReady(c kubernetes.Interface, n, p string) wait.ConditionWithContextFunc {
	return func(ctx context.Context) (done bool, err error) {
		select {
		case <-ctx.Done():
			return false, fmt.Errorf("timeout")
		default:
			pod, err := c.CoreV1().Pods(n).Get(context.TODO(), p, metav1.GetOptions{})
			if err != nil {
				return false, err
			}

			if pod.Status.Phase == corev1.PodPending && len(pod.Status.ContainerStatuses) == 0 {
				klog.Warningf("Pod: %s not ready. status: %v", pod.Name, corev1.PodPending)
				return false, nil
			}

			for _, v := range pod.Status.Conditions {
				switch v.Type {
				case corev1.PodReady:
					if v.Status == corev1.ConditionTrue {
						klog.Infof("pod: %s is ready. status: %v", pod.Name, podStatus(pod))
						return true, nil
					}
					klog.Warningf("Pod: %s not ready. status: %v", pod.Name, podStatus(pod))
					return false, nil
				default:
					continue
				}
			}
		}
		return false, err
	}
}

func CreateOrUpdateConfigmap(ctx context.Context, clientset *kubernetes.Clientset,
	name, namespace, key, data string) error {
	var cm *corev1.ConfigMap
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil {
		if apierror.IsNotFound(err) {
			// 创建 ConfigMap 对象
			cm = &corev1.ConfigMap{
				ObjectMeta: metav1.ObjectMeta{
					Name:      name,
					Namespace: namespace,
				},
				Data: map[string]string{
					key: string(data),
				},
			}
			_, err := clientset.CoreV1().ConfigMaps(cm.Namespace).Create(context.TODO(), cm,
				metav1.CreateOptions{})
			if err != nil {
				klog.Errorf("Create config err: %v", err)
				return err
			}
		}
	} else {
		if cm.Data == nil {
			cm.Data = make(map[string]string)
		}
		cm.Data[key] = string(data)
		_, err := clientset.CoreV1().ConfigMaps(cm.Namespace).Update(context.TODO(), cm,
			metav1.UpdateOptions{})
		if err != nil {
			klog.Errorf("Update config err: %v", err)
			return err
		}
	}
	return nil
}

func GetConfigmapData(ctx context.Context, clientset *kubernetes.Clientset,
	name, namespace, key string) (string, error) {
	var cm *corev1.ConfigMap
	cm, err := clientset.CoreV1().ConfigMaps(namespace).Get(context.TODO(), name, metav1.GetOptions{})
	if err != nil && !errors.IsNotFound(err) {
		klog.Errorf("get config error: %v", err)
		return "", err
	}
	if cm.Data == nil {
		return "", nil
	}
	return string(cm.Data[key]), nil
}
