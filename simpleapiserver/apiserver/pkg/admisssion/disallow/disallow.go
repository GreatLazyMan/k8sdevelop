package disallow

import (
	"context"
	"fmt"
	"io"

	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apiserver/pkg/admission"

	simpleiov1beta1 "github.com/greatlazyman/apiserver/pkg/apis/simple.io/v1beta1"
)

// Register registers a plugin
func Register(plugins *admission.Plugins) {
	plugins.Register("DisallowFoo", func(config io.Reader) (admission.Interface, error) {
		return New()
	})
}

func New() (*DisallowFoo, error) {
	return &DisallowFoo{
		Handler: *admission.NewHandler(admission.Create),
	}, nil
}

var _ admission.ValidationInterface = &DisallowFoo{}

type DisallowFoo struct {
	admission.Handler
}

func (d *DisallowFoo) Validate(ctx context.Context, a admission.Attributes, o admission.ObjectInterfaces) (err error) {
	if a.GetKind().GroupKind() != simpleiov1beta1.SchemeGroupVersion.WithKind("Foo").GroupKind() {
		return nil
	}

	metaAccessor, err := meta.Accessor(a.GetObject())
	if err != nil {
		return err
	}
	fooNamespace := metaAccessor.GetNamespace()

	if fooNamespace == "kube-system" {
		return errors.NewForbidden(
			a.GetResource().GroupResource(),
			fmt.Sprintf("%s/%s", a.GetNamespace(), a.GetName()),
			fmt.Errorf("namespace/%s is not permitted, please change the resource namespace", fooNamespace),
		)
	}

	return nil
}
