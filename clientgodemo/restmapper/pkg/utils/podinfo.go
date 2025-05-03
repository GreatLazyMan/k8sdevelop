package utils

import (
	"context"
	"fmt"

	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
)

type PodLogEventService struct {
	client *kubernetes.Clientset
}

func NewPodLogEventService(client *kubernetes.Clientset) *PodLogEventService {
	return &PodLogEventService{client: client}
}

func (this *PodLogEventService) GetLogs(ns, podname string, tailLine int64) *rest.Request {
	req := this.client.CoreV1().Pods(ns).GetLogs(podname, &v1.PodLogOptions{Follow: false, Container: "example", TailLines: &tailLine})

	return req
}

func (this *PodLogEventService) GetEvents(ns, podname string) ([]string, error) {
	events, err := this.client.CoreV1().Events(ns).List(context.TODO(), metav1.ListOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to list events: %w", err)
	}

	var podEvents []string
	for _, event := range events.Items {
		if event.InvolvedObject.Kind == "Pod" && event.InvolvedObject.Name == podname && event.Type == "Warning" {
			podEvents = append(podEvents, event.Message)
		}
	}

	return podEvents, nil
}
