package server

import (
	"context"

	"k8s.io/client-go/kubernetes"
)

type Server struct {
	ClientSet *kubernetes.Clientset
}

func NewServer(client *kubernetes.Clientset) Server {
	return Server{
		ClientSet: client,
	}
}

func (s *Server) Start(ctx context.Context) {

}
