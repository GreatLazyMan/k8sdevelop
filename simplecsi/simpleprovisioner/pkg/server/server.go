package server

import (
	"context"
	"path"
	"path/filepath"
	"strings"

	"github.com/GreatLazyMan/simpleprovisioner/pkg/utils"
	v1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"k8s.io/klog/v2"
	pvController "sigs.k8s.io/sig-storage-lib-external-provisioner/v10/controller"
)

type SimpleProvisioner struct {
	ClientSet *kubernetes.Clientset
}

const provisionerName = "simplecsi.io/local"
const CmdTimeoutCounts = 120

func NewProvisioner(client *kubernetes.Clientset) SimpleProvisioner {
	return SimpleProvisioner{
		ClientSet: client,
	}
}

func (s *SimpleProvisioner) Start(ctx context.Context) {
	//Create an instance of the Dynamic Provisioner Controller
	// that has the reconciliation loops for PVC create and delete
	// events and invokes the Provisioner Handler.
	pc := pvController.NewProvisionController(
		klog.Logger{},
		s.ClientSet,
		provisionerName,
		s,
		pvController.LeaderElection(true),
	)

	klog.Info("Provisioner started")
	//Run the provisioner till a shutdown signal is received.
	pc.Run(ctx)
	klog.Info("Provisioner stopped")
}

// Provision is invoked by the PVC controller which expect the PV
//
//	to be provisioned and a valid PV spec returned.
func (p *SimpleProvisioner) Provision(ctx context.Context, opts pvController.ProvisionOptions) (*v1.PersistentVolume,
	pvController.ProvisioningState, error) {
	klog.V(10).Infof("sc is %v", *opts.StorageClass)
	klog.V(10).Infof("node is %v", *opts.SelectedNode)
	bashPath := opts.StorageClass.Annotations["simplecsi.io/basepath"]
	image := opts.StorageClass.Annotations["simplecsi.io/image"]
	commands := []string{"mkdir", "-m", "0777", "-p", path.Join(bashPath, opts.PVName)}
	for comIndex, command := range commands {
		commands[comIndex] = "\"" + command + "\""
	}
	podStruct := PodStruct{
		LocalPath:    bashPath,
		Image:        image,
		NodeName:     opts.SelectedNode.Name,
		PvName:       opts.PVName,
		SaName:       "simplecsi",
		Namespace:    opts.PVC.Namespace,
		CommandSlice: strings.Join(commands, ","),
	}
	klog.Infof("podStruct is %v", podStruct)
	pod, err := utils.CreatePodFromTeplate(ctx, p.ClientSet, InitPodTemplate, podStruct)
	if err != nil {
		klog.Errorf("create pods error: %v", err)
		return nil, pvController.ProvisioningFinished, err
	}
	if err := utils.ExitPod(ctx, p.ClientSet, pod); err != nil {
		klog.Errorf("clear pods error: %v", err)
		return nil, pvController.ProvisioningFinished, err
	}
	pvStruct := PvStruct{
		PvName:    opts.PVName,
		PvcRV:     opts.PVC.ResourceVersion,
		PvcName:   opts.PVC.Name,
		PvcCap:    opts.PVC.Spec.Resources.Requests.Storage().String(),
		PvcUID:    string(opts.PVC.UID),
		BasePath:  bashPath,
		ScName:    opts.StorageClass.Name,
		NodeName:  podStruct.NodeName,
		Namespace: opts.PVC.Namespace,
	}
	pv, err := utils.GenePvFromTeplate(ctx, PvTemplate, pvStruct)
	if err != nil {
		klog.Errorf("gen pv error: %v", err)
		return nil, pvController.ProvisioningFinished, err
	}
	return pv, pvController.ProvisioningFinished, nil
}

// Delete is invoked by the PVC controller to perform clean-up
//
//	activities before deleteing the PV object. If reclaim policy is
//	set to not-retain, then this function will create a helper pod
//	to delete the host path from the node.
func (p *SimpleProvisioner) Delete(ctx context.Context, pv *v1.PersistentVolume) (err error) {
	bashPath := pv.Spec.Local.Path
	sc, err := p.ClientSet.StorageV1().StorageClasses().Get(ctx, pv.Spec.StorageClassName, metav1.GetOptions{})
	if err != nil {
		return err
	}
	image := sc.Annotations["simplecsi.io/image"]
	commands := []string{"rm", "-rf", bashPath}
	for comIndex, command := range commands {
		commands[comIndex] = "\"" + command + "\""
	}
	podStruct := PodStruct{
		LocalPath:    filepath.Dir(bashPath),
		Image:        image,
		NodeName:     pv.Spec.NodeAffinity.Required.NodeSelectorTerms[0].MatchExpressions[0].Values[0],
		PvName:       pv.Name,
		SaName:       "simplecsi",
		Namespace:    pv.Spec.ClaimRef.Namespace,
		CommandSlice: strings.Join(commands, ","),
	}
	klog.Infof("podStruct is %v", podStruct)
	pod, err := utils.CreatePodFromTeplate(ctx, p.ClientSet, InitPodTemplate, podStruct)
	if err != nil {
		klog.Errorf("create pods error: %v", err)
		return nil
	}
	if err := utils.ExitPod(ctx, p.ClientSet, pod); err != nil {
		klog.Errorf("clear pods error: %v", err)
		return nil
	}
	return nil
}
