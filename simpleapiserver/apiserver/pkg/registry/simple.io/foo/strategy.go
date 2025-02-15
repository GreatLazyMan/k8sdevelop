package foo

import (
	"context"
	"fmt"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/fields"
	"k8s.io/apimachinery/pkg/labels"
	"k8s.io/apimachinery/pkg/runtime"
	durationutil "k8s.io/apimachinery/pkg/util/duration"
	"k8s.io/apimachinery/pkg/util/validation/field"
	"k8s.io/apiserver/pkg/registry/generic"
	"k8s.io/apiserver/pkg/storage"
	"k8s.io/apiserver/pkg/storage/names"
	"sigs.k8s.io/structured-merge-diff/v4/fieldpath"

	hello "github.com/greatlazyman/apiserver/pkg/apis/simple.io"
)

// NewStrategy creates and returns a fooStrategy instance
func NewStrategy(typer runtime.ObjectTyper) fooStrategy {
	return fooStrategy{typer, names.SimpleNameGenerator}
}

// GetAttrs returns labels.Set, fields.Set, and error in case the given runtime.Object is not a Foo
func GetAttrs(obj runtime.Object) (labels.Set, fields.Set, error) {
	apiserver, ok := obj.(*hello.Foo)
	if !ok {
		return nil, nil, fmt.Errorf("given object is not a Foo")
	}
	return labels.Set(apiserver.ObjectMeta.Labels), SelectableFields(apiserver), nil
}

// MatchFoo is the filter used by the generic etcd backend to watch events
// from etcd to clients of the apiserver only interested in specific labels/fields.
func MatchFoo(label labels.Selector, field fields.Selector) storage.SelectionPredicate {
	return storage.SelectionPredicate{
		Label:    label,
		Field:    field,
		GetAttrs: GetAttrs,
	}
}

// SelectableFields returns a field set that represents the object.
func SelectableFields(obj *hello.Foo) fields.Set {
	return generic.ObjectMetaFieldsSet(&obj.ObjectMeta, true)
}

type fooStrategy struct {
	runtime.ObjectTyper
	names.NameGenerator
}

func (fooStrategy) NamespaceScoped() bool {
	return true
}

// GetResetFields returns the set of fields that get reset by the strategy
// and should not be modified by the user. (only do reset in put/patch actions, not for create action)
func (fooStrategy) GetResetFields() map[fieldpath.APIVersion]*fieldpath.Set {
	fields := map[fieldpath.APIVersion]*fieldpath.Set{
		"simple.io/v1beta1": fieldpath.NewSet(
			fieldpath.MakePathOrDie("status"),
		),
	}

	return fields
}

func (fooStrategy) PrepareForCreate(ctx context.Context, obj runtime.Object) {
	foo := obj.(*hello.Foo)
	foo.Status = hello.FooStatus{
		Phase: hello.FooPhaseProcessing,
	}
}

func (fooStrategy) PrepareForUpdate(ctx context.Context, obj, old runtime.Object) {
}

func (fooStrategy) Validate(ctx context.Context, obj runtime.Object) field.ErrorList {
	foo := obj.(*hello.Foo)
	return hello.ValidateFoo(foo)
}

// WarningsOnCreate returns warnings for the creation of the given object.
func (fooStrategy) WarningsOnCreate(ctx context.Context, obj runtime.Object) []string { return nil }

func (fooStrategy) AllowCreateOnUpdate() bool {
	return false
}

func (fooStrategy) AllowUnconditionalUpdate() bool {
	return false
}

func (fooStrategy) Canonicalize(obj runtime.Object) {
}

func (fooStrategy) ValidateUpdate(ctx context.Context, obj, old runtime.Object) field.ErrorList {
	return field.ErrorList{}
}

// WarningsOnUpdate returns warnings for the given update.
func (fooStrategy) WarningsOnUpdate(ctx context.Context, obj, old runtime.Object) []string {
	return nil
}

func (fooStrategy) ConvertToTable(ctx context.Context, object runtime.Object, tableOptions runtime.Object) (*metav1.Table, error) {
	var table metav1.Table

	table.ColumnDefinitions = []metav1.TableColumnDefinition{
		{Name: "Name", Type: "string", Format: "name", Description: metav1.ObjectMeta{}.SwaggerDoc()["name"]},
		{Name: "Status", Type: "string", Format: "status", Description: "status of where the Foo is in its lifecycle"},
		{Name: "Age", Type: "string", Description: metav1.ObjectMeta{}.SwaggerDoc()["creationTimestamp"]},
		{Name: "Message", Type: "string", Format: "message", Description: "foo message", Priority: 1},        // kubectl -o wide
		{Name: "Message1", Type: "string", Format: "message1", Description: "foo message plus", Priority: 1}, // kubectl -o wide
	}

	switch t := object.(type) {
	case *hello.Foo:
		table.ResourceVersion = t.ResourceVersion
		addFoosToTable(&table, *t)
	case *hello.FooList:
		table.ResourceVersion = t.ResourceVersion
		table.Continue = t.Continue
		addFoosToTable(&table, t.Items...)
	default:
	}

	return &table, nil
}

func addFoosToTable(table *metav1.Table, foos ...hello.Foo) {
	for _, foo := range foos {
		ts := "<unknown>"
		if timestamp := foo.CreationTimestamp; !timestamp.IsZero() {
			ts = durationutil.HumanDuration(time.Since(timestamp.Time))
		}
		table.Rows = append(table.Rows, metav1.TableRow{
			Cells:  []interface{}{foo.Name, foo.Status.Phase, ts, foo.Spec.Config.Msg, foo.Spec.Config.Msg1},
			Object: runtime.RawExtension{Object: &foo},
		})
	}
}
