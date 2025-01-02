/*
Copyright 2024.

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

package v1

import (
	"context"
	"fmt"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
	"sigs.k8s.io/controller-runtime/pkg/webhook"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	simpleiov1 "github.com/greatlazyman/simplewebhook/api/v1"
)

// nolint:unused
// log is for logging in this package.
var applog = logf.Log.WithName("app-resource")

// SetupAppWebhookWithManager registers the webhook for App in the manager.
func SetupAppWebhookWithManager(mgr ctrl.Manager) error {
	return ctrl.NewWebhookManagedBy(mgr).For(&simpleiov1.App{}).
		WithValidator(&AppCustomValidator{}).
		WithDefaulter(&AppCustomDefaulter{}).
		Complete()
}

// TODO(user): EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!

// +kubebuilder:webhook:path=/mutate-simple-io-github-com-v1-app,mutating=true,failurePolicy=fail,sideEffects=None,groups=simple.io.github.com,resources=apps,verbs=create;update,versions=v1,name=mapp-v1.kb.io,admissionReviewVersions=v1

// AppCustomDefaulter struct is responsible for setting default values on the custom resource of the
// Kind App when those are created or updated.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as it is used only for temporary operations and does not need to be deeply copied.
type AppCustomDefaulter struct {
	// TODO(user): Add more fields as needed for defaulting
}

var _ webhook.CustomDefaulter = &AppCustomDefaulter{}

// Default implements webhook.CustomDefaulter so a webhook will be registered for the Kind App.
func (d *AppCustomDefaulter) Default(ctx context.Context, obj runtime.Object) error {
	app, ok := obj.(*simpleiov1.App)

	if !ok {
		return fmt.Errorf("expected an App object but got %T", obj)
	}
	applog.Info("Defaulting for App", "name", app.GetName())
	if len(app.Spec.Foo) == 0 {
		app.Spec.Foo = "app"
	}

	// TODO(user): fill in your defaulting logic.

	return nil
}

// TODO(user): change verbs to "verbs=create;update;delete" if you want to enable deletion validation.
// NOTE: The 'path' attribute must follow a specific pattern and should not be modified directly here.
// Modifying the path for an invalid path can cause API server errors; failing to locate the webhook.
// +kubebuilder:webhook:path=/validate-simple-io-github-com-v1-app,mutating=false,failurePolicy=fail,sideEffects=None,groups=simple.io.github.com,resources=apps,verbs=create;update,versions=v1,name=vapp-v1.kb.io,admissionReviewVersions=v1

// AppCustomValidator struct is responsible for validating the App resource
// when it is created, updated, or deleted.
//
// NOTE: The +kubebuilder:object:generate=false marker prevents controller-gen from generating DeepCopy methods,
// as this struct is used only for temporary operations and does not need to be deeply copied.
type AppCustomValidator struct {
	//TODO(user): Add more fields as needed for validation
}

var _ webhook.CustomValidator = &AppCustomValidator{}

// ValidateCreate implements webhook.CustomValidator so a webhook will be registered for the type App.
func (v *AppCustomValidator) ValidateCreate(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	app, ok := obj.(*simpleiov1.App)
	if !ok {
		return nil, fmt.Errorf("expected a App object but got %T", obj)
	}
	applog.Info("Validation for App upon creation", "name", app.GetName())

	// TODO(user): fill in your validation logic upon object creation.

	return nil, nil
}

// ValidateUpdate implements webhook.CustomValidator so a webhook will be registered for the type App.
func (v *AppCustomValidator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) (admission.Warnings, error) {
	app, ok := newObj.(*simpleiov1.App)
	if !ok {
		return nil, fmt.Errorf("expected a App object for the newObj but got %T", newObj)
	}
	applog.Info("Validation for App upon update", "name", app.GetName())

	// TODO(user): fill in your validation logic upon object update.

	return nil, nil
}

// ValidateDelete implements webhook.CustomValidator so a webhook will be registered for the type App.
func (v *AppCustomValidator) ValidateDelete(ctx context.Context, obj runtime.Object) (admission.Warnings, error) {
	app, ok := obj.(*simpleiov1.App)
	if !ok {
		return nil, fmt.Errorf("expected a App object but got %T", obj)
	}
	applog.Info("Validation for App upon deletion", "name", app.GetName())

	// TODO(user): fill in your validation logic upon object deletion.

	return nil, nil
}
