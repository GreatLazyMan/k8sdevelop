// webhooks/service.go
package webhooks

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"

	"github.com/golang/glog"
	"k8s.io/api/admission/v1beta1"
	apiv1 "k8s.io/api/core/v1"
	v1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/types"
)

const (
	SVC_NS_NAME_LENGTH = 56
)

type ServiceValidator struct {
	Service *apiv1.Service
	uid     types.UID
}

type ServiceManager struct {
	service   *apiv1.Service
	pod       *v1.Pod
	user      string
	namespace string
	uid       types.UID
}

type PodManager struct {
	pod       *v1.Pod
	user      string
	namespace string
	uid       types.UID
}

func makePatchOperation(verb, path string) *patchOperation {
	return &patchOperation{
		Op:   verb,
		Path: path,
	}
}

func makePatchOperationWithValue(verb, path string, value any) *patchOperation {
	return &patchOperation{
		Op:    verb,
		Path:  path,
		Value: value,
	}
}

func NewServiceManager(req *v1beta1.AdmissionRequest) (*ServiceManager, error) {
	sv := &ServiceManager{
		service:   &apiv1.Service{},
		user:      req.UserInfo.Username,
		namespace: req.Namespace,
		uid:       req.UID,
	}
	if err := json.Unmarshal(req.Object.Raw, sv.service); err != nil {
		return nil, err
	}
	return sv, nil
}

func (sv *ServiceManager) injectRequiredLabels() []*patchOperation {
	patch := []*patchOperation{}

	labels := sv.service.ObjectMeta.Labels
	existLabels := make(map[string]string)
	for k, v := range labels {
		existLabels[k] = v
	}
	existLabels["enabled"] = "yes" // add enabled label and set yes as default value

	labelPath := makePatchOperation("add", "/metadata/labels")
	labelPath.Value = existLabels
	patch = append(patch, labelPath)

	return patch
}

func (sv *ServiceManager) Mutate() *v1beta1.AdmissionResponse {
	resp := &v1beta1.AdmissionResponse{
		Allowed: true,
		UID:     sv.uid,
	}

	operations := []*patchOperation{}
	operations = append(operations, sv.injectRequiredLabels()...)

	finalPatch, _ := json.Marshal(operations)
	glog.Infof("Adding patch %s for service %s in namespace %s", string(finalPatch), sv.service.ObjectMeta.Name, sv.namespace)
	resp.Patch = finalPatch
	resp.PatchType = func() *v1beta1.PatchType {
		pt := v1beta1.PatchTypeJSONPatch
		return &pt
	}()

	return resp
}

func NewPodManager(req *v1beta1.AdmissionRequest) (*PodManager, error) {
	sv := &PodManager{
		pod:       &v1.Pod{},
		user:      req.UserInfo.Username,
		namespace: req.Namespace,
		uid:       req.UID,
	}
	if err := json.Unmarshal(req.Object.Raw, sv.pod); err != nil {
		return nil, err
	}
	return sv, nil
}

func (sv *PodManager) injectRequiredEnvs() []*patchOperation {
	patch := []*patchOperation{}

	var envMap map[string]string = make(map[string]string)

	envMap["a"] = os.Getenv("KUBERNETES_SERVICE_HOST")
	containersNum := len(sv.pod.Spec.Containers)

	for i := 0; i < containersNum; i++ {
		for key, value := range envMap {
			if value != "" {
				glog.Infof("patch key %s value %s", key, value)
				patch = append(patch, makePatchOperationWithValue("replace",
					fmt.Sprintf("/spec/containers/%d/env/0/value", i),
					value))
			}
		}
	}
	return patch
}

func (sv *PodManager) Mutate() *v1beta1.AdmissionResponse {
	resp := &v1beta1.AdmissionResponse{
		Allowed: true,
		UID:     sv.uid,
	}

	operations := []*patchOperation{}
	operations = append(operations, sv.injectRequiredEnvs()...)

	finalPatch, _ := json.Marshal(operations)
	glog.Infof("uid is %s", resp.UID)
	glog.Infof("Adding patch %s for pod %s in namespace %s", string(finalPatch),
		sv.pod.ObjectMeta.Name, sv.namespace)
	resp.Patch = finalPatch
	resp.PatchType = func() *v1beta1.PatchType {
		pt := v1beta1.PatchTypeJSONPatch
		return &pt
	}()

	return resp
}

func NewServiceValidator(s *apiv1.Service, req *v1beta1.AdmissionRequest) *ServiceValidator {
	return &ServiceValidator{
		Service: s,
		uid:     req.UID,
	}
}

func (sv *ServiceValidator) hasProtocolPrefix(name string) bool {
	i := strings.IndexByte(name, '-')
	if i >= 0 {
		name = name[:i]
	}
	return true
}

func (sv *ServiceValidator) validatePorts() *v1beta1.AdmissionResponse {
	for _, sp := range sv.Service.Spec.Ports {
		switch sp.Protocol {
		case apiv1.ProtocolUDP: // skip udp validate
			break
		}
		if sp.Port == 443 {
			if !strings.HasPrefix(sp.Name, "https") {
				return MakeAdmissionResponse(false,
					"You cannot configure a non-HTTPs service with 443 port", sv.uid)
			}
		}
	}
	return MakeAdmissionResponse(true, "Allowed", sv.uid)
}

func (sv *ServiceValidator) validate() *v1beta1.AdmissionResponse {
	if r := sv.validatePorts(); r != nil {
		return r
	}
	return nil
}
