package app

import (
	"context"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/GreatLazyMan/simpledeviceplugin/cmd/options"
	"github.com/GreatLazyMan/simpledeviceplugin/pkg/server"
	"github.com/fsnotify/fsnotify"
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
	s := server.NewServer(opt.DevicePluginPath)
	// TODO: do some work
	KubeletSocket := opt.DevicePluginPath + server.KubeletSock

	klog.Info("try to start server")
	var restart bool
	go run(ctx)
	s.Serve(ctx)
	for {
		if restart {
			if s.GrpcServer != nil {
				s.GrpcServer.Stop()
			}
			s.Serve(ctx)
			restart = false
		}
		select {
		case event := <-s.Watcher.Events:
			if event.Name == KubeletSocket && event.Op&fsnotify.Create == fsnotify.Create {
				klog.Infof("%s is created, restartring", KubeletSocket)
				restart = true
			}
		case err := <-s.Watcher.Errors:
			klog.Errorf("watch ereor: %v", err)
		case <-ctx.Done():
			return
		}
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
