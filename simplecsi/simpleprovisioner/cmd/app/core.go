package app

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/GreatLazyMan/simpleprovisioner/cmd/options"
	"github.com/GreatLazyMan/simpleprovisioner/pkg/server"
	"github.com/GreatLazyMan/simpleprovisioner/pkg/utils"
	"k8s.io/klog/v2"
)

// Health check handler function
func healthCheckHandler(w http.ResponseWriter, r *http.Request) {
	// Here, you can add any logic to determine the health of your application
	// For simplicity, we'll always return "OK"
	w.WriteHeader(http.StatusOK)
	fmt.Fprintln(w, "OK")
}

func Start(ctx context.Context, wg *sync.WaitGroup, opt *options.Options) {
	_, clientSet, err := utils.GetKubernetesClient(opt.KubeconfigPath)
	if err != nil {
		klog.Fatalf("init kubeclientset error: %v", err)
	}
	s := server.NewProvisioner(clientSet)
	// TODO: do some work
	go run(ctx)
	go s.Start(ctx)

	// Register the health check handler with the "/healthz" endpoint
	http.HandleFunc("/healthz", healthCheckHandler)

	// Start the HTTP server on port 8080
	fmt.Println("Starting server on :8080")
	if err := http.ListenAndServe(":8080", nil); err != nil {
		klog.Fatalf("Error starting server: %s\n", err)
	}
}

func run(ctx context.Context) {
	ticker := time.NewTicker(1 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			// TODO: do some real work
		case <-ctx.Done():
			return
		}
	}
}
