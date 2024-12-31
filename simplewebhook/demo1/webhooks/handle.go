// webhooks/handle.go
package webhooks

import (
	"encoding/json"
	"fmt"
	"strings"

	"github.com/golang/glog"

	"k8s.io/api/admission/v1beta1"
	apiv1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
)

type patchOperation struct {
	Op    string      `json:"op"`
	Path  string      `json:"path"`
	Value interface{} `json:"value,omitempty"`
}

func Validate(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	req := ar.Request
	var resp *v1beta1.AdmissionResponse
	switch req.Kind.Kind {
	case "Service":
		switch req.Kind.Group {
		case "":
			s := &apiv1.Service{}
			if err := json.Unmarshal(req.Object.Raw, s); err != nil {
				glog.Errorf("Could not unmarshal raw object %v", err)
				return UnmarshalError(err, req.UID)
			}
			sv := NewServiceValidator(s, req)
			resp = sv.validate()
		}
		// case "Pod": ....
	}
	if resp == nil {
		resp = &v1beta1.AdmissionResponse{
			Allowed: true,
		}
	}
	return resp
}

func Mutate(ar *v1beta1.AdmissionReview) *v1beta1.AdmissionResponse {
	req := ar.Request
	resp := &v1beta1.AdmissionResponse{}
	switch req.Kind.Kind {
	case "Service":
		sv, err := NewServiceManager(req)
		if err != nil {
			return UnmarshalError(err, req.UID)
		}
		glog.Infof("req uid is %s", req.UID)
		resp = sv.Mutate()
	case "Pod":
		sv, err := NewPodManager(req)
		if err != nil {
			return UnmarshalError(err, req.UID)
		}
		glog.Infof("req uid is %s", req.UID)
		resp = sv.Mutate()
	default:
		glog.Infof("unknown kind %s", req.Kind.Kind)
	}
	return resp
}

func MakeAdmissionResponse(allowed bool, reason metav1.StatusReason, uid types.UID) *v1beta1.AdmissionResponse {
	resp := new(v1beta1.AdmissionResponse)
	resp.Allowed = allowed
	resp.Result = &metav1.Status{
		Reason: reason,
	}
	resp.UID = uid
	return resp
}

func UnmarshalError(err error, uid types.UID) *v1beta1.AdmissionResponse {
	errMsg := fmt.Sprintf("Cannot unmarshal raw objects from API server, %v", err)
	if strings.Contains(err.Error(), "AnalysisMessageBase_Level") {
		errMsg = "Cannot unmarshal the object due to Istio API issue. If you are creating a virtual service, please check if you provide the correct gateway in the manifest."
	}
	return MakeAdmissionResponse(false, metav1.StatusReason(errMsg), uid)
}
