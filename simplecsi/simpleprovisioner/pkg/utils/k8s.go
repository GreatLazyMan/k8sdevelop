package utils

import (
	"bytes"
	"context"
	"fmt"
	"text/template"
	"time"

	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/klog/v2"
	"sigs.k8s.io/yaml"
)

const CmdTimeoutCounts = 120

func GetKubernetesClient(kubeconfig string) (*rest.Config, *kubernetes.Clientset, error) {
	var config *rest.Config
	var err error
	if len(kubeconfig) == 0 {
		config, err = rest.InClusterConfig()
	} else {
		config, err = clientcmd.BuildConfigFromFlags("", kubeconfig)
	}
	if err != nil {
		return nil, nil, err
	}
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, nil, err
	}
	return config, clientset, nil
}

// pluginOptions
func defaultValue(value interface{}, defaultVal string) string {
	if str, ok := value.(string); ok && str != "" {
		return str
	}
	return defaultVal
}

// ParseTemplate validates and parses passed as argument template
func ParseTemplate(strtmpl string, obj interface{}) (string, error) {
	var buf bytes.Buffer
	tmpl := template.New("template").Funcs(template.FuncMap{
		"defaultValue": defaultValue,
	})
	tmpl, err := tmpl.Parse(strtmpl)
	if err != nil {
		return "", fmt.Errorf("error when parsing template, err: %w", err)
	}
	err = tmpl.Execute(&buf, obj)
	if err != nil {
		return "", fmt.Errorf("error when executing template, err: %w", err)
	}
	return buf.String(), nil
}

func CreatePodFromTeplate(ctx context.Context, clientSet *kubernetes.Clientset,
	podTemplate string, podStruct interface{}) (*v1.Pod, error) {
	initPodBytes, err := ParseTemplate(podTemplate, podStruct)
	if err != nil {
		klog.Errorf("ParseTemplate error: %v", err)
		return nil, err
	}

	// 将 YAML 文件解析为 unstructured.Unstructured 对象
	obj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal([]byte(initPodBytes), obj); err != nil {
		klog.Errorf("unstructured.Unstructured  yaml err: %v", err)
		return nil, err
	}

	// 将 unstructured.Unstructured 对象转化为 runtime.Object
	runtimeObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		klog.Errorf("unstructured.Unstructured  yaml err: %v", err)
		return nil, err
	}

	pod := &v1.Pod{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(runtimeObj, pod); err != nil {
		klog.Errorf("unstructured.Unstructured  yaml err: %v", err)
		return nil, err
	}

	// 创建 Pod
	newPod, err := clientSet.CoreV1().Pods("default").Create(ctx, pod, metav1.CreateOptions{})
	if err != nil && !errors.IsAlreadyExists(err) {
		klog.Errorf("create pods error: %v", err)
		return nil, err
	}
	return newPod, err
}

func ExitPod(ctx context.Context, clientSet *kubernetes.Clientset, hPod *v1.Pod) error {
	defer func() {
		e := clientSet.CoreV1().Pods(hPod.Namespace).Delete(ctx, hPod.Name, metav1.DeleteOptions{})
		if e != nil {
			klog.Errorf("unable to delete the helper pod: %v", e)
		}
	}()

	//Wait for the helper pod to complete it job and exit
	completed := false
	for i := 0; i < CmdTimeoutCounts; i++ {
		checkPod, err := clientSet.CoreV1().Pods(hPod.Namespace).Get(ctx, hPod.Name, metav1.GetOptions{})
		if err != nil {
			return err
		} else if checkPod.Status.Phase == v1.PodSucceeded {
			completed = true
			break
		}
		time.Sleep(1 * time.Second)
	}
	if !completed {
		return fmt.Errorf("create process timeout after %v seconds", CmdTimeoutCounts)
	}
	return nil
}
func GenePvFromTeplate(ctx context.Context, pTemplate string, pStruct interface{}) (*v1.PersistentVolume, error) {
	initPvBytes, err := ParseTemplate(pTemplate, pStruct)
	if err != nil {
		klog.Errorf("ParseTemplate error: %v", err)
		return nil, err
	}
	klog.Infof("initPvBytes is %v\n pStruct is %v", initPvBytes, pStruct)
	// 将 YAML 文件解析为 unstructured.Unstructured 对象
	obj := &unstructured.Unstructured{}
	if err := yaml.Unmarshal([]byte(initPvBytes), obj); err != nil {
		klog.Errorf("unstructured.Unstructured  yaml err: %v", err)
		return nil, err
	}

	// 将 unstructured.Unstructured 对象转化为 runtime.Object
	runtimeObj, err := runtime.DefaultUnstructuredConverter.ToUnstructured(obj)
	if err != nil {
		klog.Errorf("unstructured.Unstructured  yaml err: %v", err)
		return nil, err
	}

	pv := &v1.PersistentVolume{}
	if err := runtime.DefaultUnstructuredConverter.FromUnstructured(runtimeObj, pv); err != nil {
		klog.Errorf("unstructured.Unstructured  yaml err: %v", err)
		return nil, err
	}

	return pv, err
}
