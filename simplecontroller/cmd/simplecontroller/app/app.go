package app

import (
	"context"
	"fmt"
	"os"
	"time"

	apisixv2 "github.com/apache/apisix-ingress-controller/pkg/kube/apisix/apis/config/v2"
	"github.com/spf13/cobra"
	corev1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/networking/v1"
	apiextensionsv1 "k8s.io/apiextensions-apiserver/pkg/apis/apiextensions/v1"
	"k8s.io/apimachinery/pkg/api/meta"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/util/uuid"
	"k8s.io/client-go/dynamic"
	"k8s.io/client-go/informers"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/rest"
	"k8s.io/client-go/tools/cache"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/leaderelection"
	"k8s.io/client-go/tools/leaderelection/resourcelock"
	cliflag "k8s.io/component-base/cli/flag"
	klog "k8s.io/klog/v2"
	controllerruntime "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/manager"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"

	"github.com/GreatLazyMan/simplecontroller/cmd/simplecontroller/options"
	simplecontrollerversioned "github.com/GreatLazyMan/simplecontroller/pkg/apis/client/clientset/versioned"
	"github.com/GreatLazyMan/simplecontroller/pkg/sharedcli/klogflag"
	"github.com/GreatLazyMan/simplecontroller/pkg/simplecontroller/controller"
	"github.com/GreatLazyMan/simplecontroller/pkg/simplecontroller/scheme"
)

func NewSimplecontrollerCommand(ctx context.Context) *cobra.Command {
	opts := options.NewOptions()

	cmd := &cobra.Command{
		Use:  "",
		Long: ``,
		RunE: func(cmd *cobra.Command, args []string) error {
			if errs := opts.Validate(); len(errs) != 0 {
				return errs.ToAggregate()
			}
			if err := leaderElectionRun(ctx, opts); err != nil {
				return err
			}
			return nil
		},
	}

	fss := cliflag.NamedFlagSets{}

	genericFlagSet := fss.FlagSet("generic")
	opts.AddFlags(genericFlagSet)

	logsFlagSet := fss.FlagSet("logs")
	klogflag.Add(logsFlagSet)
	klog.StartFlushDaemon(time.Second * 5)

	cmd.Flags().AddFlagSet(genericFlagSet)
	cmd.Flags().AddFlagSet(logsFlagSet)

	return cmd
}

func controllerruntimeRun(ctx context.Context, mgr manager.Manager, cherr chan error) {

	controllerNodeName := "Simplecontroller-Node-controller"
	SimplecontrollerNodeController := controller.NodeController{
		Name:          controllerNodeName,
		Client:        mgr.GetClient(),
		Config:        mgr.GetConfig(),
		EventRecorder: mgr.GetEventRecorderFor(controllerNodeName),
	}
	if err := SimplecontrollerNodeController.SetupWithManager(mgr); err != nil {
		cherr <- fmt.Errorf("error starting %s: %v", controllerNodeName, err)
	}

	controllerPodName := "Simplecontroller-Pod-controller"
	SimplecontrollerPodController := controller.PodController{
		Name:          controllerPodName,
		Client:        mgr.GetClient(),
		Config:        mgr.GetConfig(),
		EventRecorder: mgr.GetEventRecorderFor(controllerPodName),
	}
	if err := SimplecontrollerPodController.SetupWithManager(mgr); err != nil {
		cherr <- fmt.Errorf("error starting %s: %v", controllerPodName, err)
	}

	controllerClusterName := "Simplecontroller-Cluster-controller"
	SimplecontrollerClusterController := controller.ClusterController{
		Name:          controllerClusterName,
		Client:        mgr.GetClient(),
		Config:        mgr.GetConfig(),
		EventRecorder: mgr.GetEventRecorderFor(controllerClusterName),
	}
	if err := SimplecontrollerClusterController.SetupWithManager(mgr); err != nil {
		cherr <- fmt.Errorf("error starting %s: %v", controllerClusterName, err)
	}

	apisixRouteCRD := apisixv2.SchemeGroupVersion.WithKind("ApisixRoute")
	_, err := mgr.GetRESTMapper().RESTMapping(apisixRouteCRD.GroupKind(), apisixRouteCRD.Version)
	if err != nil {
		if !meta.IsNoMatchError(err) {
			cherr <- err
		}
		klog.Errorf("no match crd %s err: %v", apisixRouteCRD.String(), err)
	} else {
		controllerApisixName := "Simplecontroller-Apisix-controller"
		SimplecontrollerApisixController := controller.ApisixRouteController{
			Name:          controllerApisixName,
			Client:        mgr.GetClient(),
			Config:        mgr.GetConfig(),
			EventRecorder: mgr.GetEventRecorderFor(controllerApisixName),
		}
		if err := SimplecontrollerApisixController.SetupWithManager(mgr); err != nil {
			cherr <- fmt.Errorf("error starting %s: %v", controllerApisixName, err)
		}
	}

	controllerSimpleJobName := "Simplecontroller-SimpleJob-controller"
	SimplecontrollerSimpleJobController := controller.SimpleJobController{
		Name:          controllerSimpleJobName,
		Client:        mgr.GetClient(),
		Config:        mgr.GetConfig(),
		EventRecorder: mgr.GetEventRecorderFor(controllerSimpleJobName),
		Scheme:        mgr.GetScheme(),
	}
	if err := SimplecontrollerSimpleJobController.SetupWithManager(mgr); err != nil {
		cherr <- fmt.Errorf("error starting %s: %v", controllerSimpleJobName, err)
	}

	klog.Infof("is Elected: %v", mgr.Elected())
	if err := mgr.Start(ctx); err != nil {
		cherr <- fmt.Errorf("failed to start controller manager: %v", err)
	}

}

func informerRun(ctx context.Context, cherr chan error, restConfig *rest.Config) {

	// 初始化 client
	clientset, err := kubernetes.NewForConfig(restConfig)
	if err != nil {
		klog.Errorf("Init kubernetes client err: %v", err)
		cherr <- err
	}
	dsController := controller.NewDsReconsiler(ctx, clientset)
	go dsController.Start()

	versionClientset, err := simplecontrollerversioned.NewForConfig(restConfig)
	if err != nil {
		klog.Errorf("Init versiond kubernetes client err: %v", err)
		cherr <- err
	}
	rcController := controller.NewRClusterReconsiler(ctx, versionClientset)
	go rcController.Start()

	dynamiClientset, err := dynamic.NewForConfig(restConfig)
	if err != nil {
		klog.Errorf("Init kubernetes client err: %v", err)
		cherr <- err
	}
	ippoolController := controller.NewIPPoolReconsiler(ctx, dynamiClientset)
	if ippoolController != nil {
		go ippoolController.Start()
	}

	kubeInformerFactory := informers.NewSharedInformerFactoryWithOptions(clientset, 0,
		informers.WithTweakListOptions(func(lo *metav1.ListOptions) {
			lo.FieldSelector = fields.OneTermEqualSelector("metadata.namespace", "default").String()
		}))

	controller := controller.NewController(clientset,
		kubeInformerFactory.Core().V1().Services(),
		kubeInformerFactory.Core().V1().Endpoints(),
	)
	kubeInformerFactory.Start(ctx.Done())
	synced := []cache.InformerSynced{
		kubeInformerFactory.Core().V1().Services().Informer().HasSynced,
		kubeInformerFactory.Core().V1().Endpoints().Informer().HasSynced,
	}
	if err := controller.Run(ctx.Done(), synced...); err != nil {
		klog.Errorf("Error running controller: %s", err.Error())
	}
}

func leaderElectionRun(ctx context.Context, opts *options.Options) error {

	// init
	config, err := clientcmd.BuildConfigFromFlags(opts.KubernetesOptions.Master, opts.KubernetesOptions.KubeConfig)
	if err != nil {
		panic(err)
	}
	config.QPS, config.Burst = opts.KubernetesOptions.QPS, opts.KubernetesOptions.Burst

	newscheme := scheme.NewSchema()
	err = apiextensionsv1.AddToScheme(newscheme)
	if err != nil {
		return err
	}
	err = corev1.AddToScheme(newscheme)
	if err != nil {
		return err
	}
	err = v1.AddToScheme(newscheme)
	if err != nil {
		return err
	}

	mgr, err := controllerruntime.NewManager(config, controllerruntime.Options{
		Logger:         klog.Background(),
		Scheme:         newscheme,
		LeaderElection: false,
		Metrics: metricsserver.Options{
			BindAddress: "0",
		},
		HealthProbeBindAddress: "0",
	})
	if err != nil {
		panic(fmt.Errorf("failed to build controller manager: %v", err))
	}
	id, err := os.Hostname()
	if err != nil {
		return err
	}

	// leader election
	id += "_" + string(uuid.NewUUID())

	rl, err := resourcelock.NewFromKubeconfig(
		opts.LeaderElection.ResourceLock,
		opts.LeaderElection.ResourceNamespace,
		opts.LeaderElection.ResourceName,
		resourcelock.ResourceLockConfig{
			Identity: id,
		},
		mgr.GetConfig(),
		opts.LeaderElection.RenewDeadline.Duration,
	)
	if err != nil {
		return err
	}

	leaderelection.RunOrDie(ctx, leaderelection.LeaderElectionConfig{
		Lock:          rl,
		Name:          opts.LeaderElection.ResourceName,
		LeaseDuration: opts.LeaderElection.LeaseDuration.Duration,
		RenewDeadline: opts.LeaderElection.RenewDeadline.Duration,
		RetryPeriod:   opts.LeaderElection.RetryPeriod.Duration,

		Callbacks: leaderelection.LeaderCallbacks{
			OnStartedLeading: func(ctx context.Context) {
				klog.Warning("leader-election got, simplecontroller is awaking")
				_ = run(ctx, mgr)
				os.Exit(0)
			},
			OnStoppedLeading: func() {
				klog.Warning("leader-election lost, simplecontroller is dying")
				os.Exit(0)
			},
		},
	})

	return nil
}
func run(ctx context.Context, mgr manager.Manager) error {

	cherr := make(chan error)
	//controller-runtime style
	go controllerruntimeRun(ctx, mgr, cherr)

	//informer style
	go informerRun(ctx, cherr, mgr.GetConfig())

	return <-cherr
}
