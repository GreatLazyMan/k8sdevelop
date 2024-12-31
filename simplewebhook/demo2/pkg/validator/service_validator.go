/*
Copyright 2020 The Kubernetes Authors.

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

package validator

import (
	"context"
	"fmt"
	"net"
	"net/http"

	"github.com/prometheus/client_golang/prometheus"
	authenticationv1 "k8s.io/api/authentication/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/kubectl/pkg/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/metrics"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var (
	setupLog            = ctrl.Log.WithName("setup")
	failedRequestsCount = prometheus.NewCounter(
		prometheus.CounterOpts{
			Name: "webhook_failed_request_count",
			Help: "Number of failed requests count total",
		},
	)
)

// +kubebuilder:webhook:path=/validate-service,mutating=false,failurePolicy=Fail,groups="core",resources=services,verbs=create;update,versions=v1,name=validate-externalip.webhook.svc,sideEffects=None,admissionReviewVersions=v1beta1

type ServiceValidator struct {
	allowedExternalIPNets []*net.IPNet
	decoder               *admission.Decoder
}

func init() {
	// Register custom metrics with the global prometheus registry
	metrics.Registry.MustRegister(failedRequestsCount)
}

// NewServiceValidator validates the input list if any and returns ServiceValidator with list of valid external IPNets
func NewServiceValidator(allowedCIDRs []string) (*ServiceValidator, error) {
	var externalIPNets []*net.IPNet
	for _, allowedCIDR := range allowedCIDRs {
		_, ipNet, err := net.ParseCIDR(allowedCIDR)
		if err != nil {
			return nil, err
		}
		externalIPNets = append(externalIPNets, ipNet)
	}
	s := scheme.Scheme
	corev1.AddToScheme(s)
	decoder := admission.NewDecoder(s)

	return &ServiceValidator{allowedExternalIPNets: externalIPNets, decoder: &decoder}, nil
}

// Handle handles the /validate-service endpoint requests
func (sv *ServiceValidator) Handle(ctx context.Context, req admission.Request) admission.Response {

	svc := &corev1.Service{}

	err := (*sv.decoder).Decode(req, svc)
	if err != nil {
		return admission.Errored(http.StatusBadRequest, err)
	}

	if len(svc.Spec.ExternalIPs) > 0 && !sv.isValidUser(req.UserInfo) {
		failedRequestsCount.Inc()
		reason := fmt.Sprintf("user %s is not alllowed to specify externalIP", req.UserInfo.Username)
		response := admission.Denied(reason)
		response.AuditAnnotations = map[string]string{"error": reason}
		return response
	}

	return sv.validateExternalIPs(svc.Spec.ExternalIPs)
}

// validateExternalIPs validates if external IP specified in the service is allowed or not
func (sv *ServiceValidator) validateExternalIPs(externalIPsInSpec []string) admission.Response {

	for _, externalIP := range externalIPsInSpec {
		var found bool
		ip := net.ParseIP(externalIP)
		if ip == nil {
			failedRequestsCount.Inc()
			return buildDeniedResponse(externalIP, "externalIP specified is not valid")
		}

		for _, externalIPNet := range sv.allowedExternalIPNets {
			if externalIPNet.Contains(ip) {
				found = true
				break
			}
		}
		if !found {
			failedRequestsCount.Inc()
			return buildDeniedResponse(externalIP, "externalIP specified is not allowed to use")
		}
	}

	return admission.Allowed("passed svc spec validation")
}

func (sv *ServiceValidator) isValidUser(user authenticationv1.UserInfo) bool {
	// If both allowedUsernames and allowedGroups are empty, anyone is allowed
	setupLog.Info("UserInfo is %v", "user", user)
	return true
}

func buildDeniedResponse(externalIP string, reason string) admission.Response {
	response := admission.Denied(field.Invalid(
		field.NewPath("spec").Child("externalIPs"), externalIP, reason).Error())
	response.AuditAnnotations = map[string]string{
		"error": reason,
	}
	return response
}
