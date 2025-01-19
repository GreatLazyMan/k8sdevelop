package foo

import (
	"bytes"
	"context"
	"fmt"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"sync"

	"k8s.io/apimachinery/pkg/util/uuid"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	metainternalversion "k8s.io/apimachinery/pkg/apis/meta/internalversion"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/conversion"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/watch"
	genericapirequest "k8s.io/apiserver/pkg/endpoints/request"
	"k8s.io/apiserver/pkg/registry/rest"
)

/*
https://blog.gmem.cc/kubernetes-style-apiserver
调用NewStore即可创建一个rest.Storage。前面我们提到过存储后端有很多细节需要处理，对于上面这个样例，它没有：
1.发现正在删除中的资源，并在CRUD时作出适当响应
2.进行资源合法性校验。genericregistry.Store的做法是，调用strategy进行校验
3.自动填充某些元数据字段，包括creationTimestamp、selfLink等
*/

var _ rest.StandardStorage = &store{}
var _ rest.Scoper = &store{}
var _ rest.Storage = &store{}

/*
https://mp.weixin.qq.com/s/AHwIwWp-XC86R0zgcnkEug?poc_token=HJ8cjWejhS1GGyForK9nlBJtt76riyksT10JdMUs
https://github.com/acorn-io/mink
// Note: the rest.StandardStorage interface aggregates the common REST verbs
var _ rest.StandardStorage = &Store{}
var _ rest.Exporter = &Store{}
var _ rest.TableConvertor = &Store{}
*/
// NewStore instantiates a new file storage
func NewFileStore(groupResource schema.GroupResource, codec runtime.Codec, rootpath string, isNamespaced bool,
	newFunc func() runtime.Object, newListFunc func() runtime.Object, tc rest.TableConvertor) rest.Storage {
	objRoot := filepath.Join(rootpath, groupResource.Group, groupResource.Resource)
	if err := ensureDir(objRoot); err != nil {
		panic(fmt.Sprintf("unable to write data dir: %s", err))
	}
	rest := &store{
		defaultQualifiedResource: groupResource,
		TableConvertor:           tc,
		codec:                    codec,
		objRootPath:              objRoot,
		isNamespaced:             isNamespaced,
		newFunc:                  newFunc,
		newListFunc:              newListFunc,
		watchers:                 make(map[int]*yamlWatch, 10),
	}
	return rest
}

type store struct {
	rest.TableConvertor
	codec        runtime.Codec
	objRootPath  string
	isNamespaced bool

	muWatchers sync.RWMutex
	watchers   map[int]*yamlWatch

	newFunc                  func() runtime.Object
	newListFunc              func() runtime.Object
	defaultQualifiedResource schema.GroupResource
}

func (f *store) notifyWatchers(ev watch.Event) {
	f.muWatchers.RLock()
	for _, w := range f.watchers {
		w.ch <- ev
	}
	f.muWatchers.RUnlock()
}

func (f *store) New() runtime.Object {
	return f.newFunc()
}

func (f *store) Destroy() {
}

func (f *store) NewList() runtime.Object {
	return f.newListFunc()
}

func (f *store) NamespaceScoped() bool {
	return f.isNamespaced
}

func (f *store) Get(ctx context.Context, name string, options *metav1.GetOptions) (runtime.Object, error) {
	return read(f.codec, f.objectFileName(ctx, name), f.newFunc)
}

func (f *store) List(ctx context.Context, options *metainternalversion.ListOptions) (runtime.Object, error) {
	newListObj := f.NewList()
	v, err := getListPrt(newListObj)
	if err != nil {
		return nil, err
	}

	dirname := f.objectDirName(ctx)
	if err := visitDir(dirname, f.newFunc, f.codec, func(path string, obj runtime.Object) {
		appendItem(v, obj)
	}); err != nil {
		return nil, fmt.Errorf("failed walking filepath %v", dirname)
	}
	return newListObj, nil
}

func (f *store) Create(ctx context.Context, obj runtime.Object, createValidation rest.ValidateObjectFunc,
	options *metav1.CreateOptions) (runtime.Object, error) {
	if createValidation != nil {
		if err := createValidation(ctx, obj); err != nil {
			return nil, err
		}
	}
	if f.isNamespaced {
		ns, ok := genericapirequest.NamespaceFrom(ctx)
		if !ok {
			return nil, apierrors.NewBadRequest("namespace required")
		}
		if err := ensureDir(filepath.Join(f.objRootPath, ns)); err != nil {
			return nil, err
		}
	}

	accessor, err := meta.Accessor(obj)
	if err != nil {
		return nil, err
	}
	if accessor.GetUID() == "" {
		accessor.SetUID(uuid.NewUUID())
	}

	name := accessor.GetName()
	filename := f.objectFileName(ctx, name)
	qualifiedResource := f.qualifiedResourceFromContext(ctx)
	if exists(filename) {
		return nil, apierrors.NewAlreadyExists(qualifiedResource, name)
	}

	if err := write(f.codec, filename, obj); err != nil {
		return nil, apierrors.NewInternalError(err)
	}

	f.notifyWatchers(watch.Event{
		Type:   watch.Added,
		Object: obj,
	})

	return obj, nil
}

func (f *store) Update(ctx context.Context, name string, objInfo rest.UpdatedObjectInfo,
	createValidation rest.ValidateObjectFunc, updateValidation rest.ValidateObjectUpdateFunc,
	forceAllowCreate bool, options *metav1.UpdateOptions) (runtime.Object, bool, error) {
	isCreate := false
	oldObj, err := f.Get(ctx, name, nil)
	if err != nil {
		if !forceAllowCreate {
			return nil, false, err
		}
		isCreate = true
	}

	if f.isNamespaced {
		// ensures namespace dir
		ns, ok := genericapirequest.NamespaceFrom(ctx)
		if !ok {
			return nil, false, apierrors.NewBadRequest("namespace required")
		}
		if err := ensureDir(filepath.Join(f.objRootPath, ns)); err != nil {
			return nil, false, err
		}
	}

	updatedObj, err := objInfo.UpdatedObject(ctx, oldObj)
	if err != nil {
		return nil, false, err
	}
	filename := f.objectFileName(ctx, name)

	if isCreate {
		if createValidation != nil {
			if err := createValidation(ctx, updatedObj); err != nil {
				return nil, false, err
			}
		}
		if err := write(f.codec, filename, updatedObj); err != nil {
			return nil, false, err
		}
		f.notifyWatchers(watch.Event{
			Type:   watch.Added,
			Object: updatedObj,
		})
		return updatedObj, true, nil
	}

	if updateValidation != nil {
		if err := updateValidation(ctx, updatedObj, oldObj); err != nil {
			return nil, false, err
		}
	}
	if err := write(f.codec, filename, updatedObj); err != nil {
		return nil, false, err
	}
	f.notifyWatchers(watch.Event{
		Type:   watch.Modified,
		Object: updatedObj,
	})
	return updatedObj, false, nil
}

func (f *store) Delete(ctx context.Context, name string, deleteValidation rest.ValidateObjectFunc,
	options *metav1.DeleteOptions) (runtime.Object, bool, error) {
	filename := f.objectFileName(ctx, name)
	qualifiedResource := f.qualifiedResourceFromContext(ctx)
	if !exists(filename) {
		return nil, false, apierrors.NewNotFound(qualifiedResource, name)
	}

	oldObj, err := f.Get(ctx, name, nil)
	if err != nil {
		return nil, false, err
	}
	if deleteValidation != nil {
		if err := deleteValidation(ctx, oldObj); err != nil {
			return nil, false, err
		}
	}

	if err := os.Remove(filename); err != nil {
		return nil, false, err
	}
	f.notifyWatchers(watch.Event{
		Type:   watch.Deleted,
		Object: oldObj,
	})
	return oldObj, true, nil
}

func (f *store) DeleteCollection(ctx context.Context, deleteValidation rest.ValidateObjectFunc,
	options *metav1.DeleteOptions, listOptions *metainternalversion.ListOptions) (runtime.Object, error) {
	newListObj := f.NewList()
	v, err := getListPrt(newListObj)
	if err != nil {
		return nil, err
	}
	dirname := f.objectDirName(ctx)
	if err := visitDir(dirname, f.newFunc, f.codec, func(path string, obj runtime.Object) {
		_ = os.Remove(path)
		appendItem(v, obj)
	}); err != nil {
		return nil, fmt.Errorf("failed walking filepath %v", dirname)
	}
	return newListObj, nil
}

func (f *store) objectFileName(ctx context.Context, name string) string {
	if f.isNamespaced {
		// FIXME: return error if namespace is not found
		ns, _ := genericapirequest.NamespaceFrom(ctx)
		return filepath.Join(f.objRootPath, ns, name+".yaml")
	}
	return filepath.Join(f.objRootPath, name+".yaml")
}

func (f *store) objectDirName(ctx context.Context) string {
	if f.isNamespaced {
		// FIXME: return error if namespace is not found
		ns, _ := genericapirequest.NamespaceFrom(ctx)
		return filepath.Join(f.objRootPath, ns)
	}
	return filepath.Join(f.objRootPath)
}

func write(encoder runtime.Encoder, filepath string, obj runtime.Object) error {
	buf := new(bytes.Buffer)
	if err := encoder.Encode(obj, buf); err != nil {
		return err
	}
	return os.WriteFile(filepath, buf.Bytes(), 0600)
}

func read(decoder runtime.Decoder, path string, newFunc func() runtime.Object) (runtime.Object, error) {
	content, err := os.ReadFile(filepath.Clean(path))
	if err != nil {
		return nil, err
	}
	newObj := newFunc()
	decodedObj, _, err := decoder.Decode(content, nil, newObj)
	if err != nil {
		return nil, err
	}
	return decodedObj, nil
}

func exists(filepath string) bool {
	_, err := os.Stat(filepath)
	return err == nil
}

func ensureDir(dirname string) error {
	if !exists(dirname) {
		return os.MkdirAll(dirname, 0700)
	}
	return nil
}

func visitDir(dirname string, newFunc func() runtime.Object, codec runtime.Decoder,
	visitFunc func(string, runtime.Object)) error {
	return filepath.Walk(dirname, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		if !strings.HasSuffix(info.Name(), ".yaml") {
			return nil
		}
		newObj, err := read(codec, path, newFunc)
		if err != nil {
			return err
		}
		visitFunc(path, newObj)
		return nil
	})
}

func appendItem(v reflect.Value, obj runtime.Object) {
	v.Set(reflect.Append(v, reflect.ValueOf(obj).Elem()))
}

func getListPrt(listObj runtime.Object) (reflect.Value, error) {
	listPtr, err := meta.GetItemsPtr(listObj)
	if err != nil {
		return reflect.Value{}, err
	}
	v, err := conversion.EnforcePtr(listPtr)
	if err != nil || v.Kind() != reflect.Slice {
		return reflect.Value{}, fmt.Errorf("need ptr to slice: %v", err)
	}
	return v, nil
}

func (f *store) Watch(ctx context.Context, options *metainternalversion.ListOptions) (watch.Interface, error) {
	yw := &yamlWatch{
		id: len(f.watchers),
		f:  f,
		ch: make(chan watch.Event, 10),
	}
	// On initial watch, send all the existing objects
	list, err := f.List(ctx, options)
	if err != nil {
		return nil, err
	}

	danger := reflect.ValueOf(list).Elem()
	items := danger.FieldByName("Items")

	for i := 0; i < items.Len(); i++ {
		obj := items.Index(i).Addr().Interface().(runtime.Object)
		yw.ch <- watch.Event{
			Type:   watch.Added,
			Object: obj,
		}
	}

	f.muWatchers.Lock()
	f.watchers[yw.id] = yw
	f.muWatchers.Unlock()

	return yw, nil
}

type yamlWatch struct {
	f  *store
	id int
	ch chan watch.Event
}

func (w *yamlWatch) Stop() {
	w.f.muWatchers.Lock()
	delete(w.f.watchers, w.id)
	w.f.muWatchers.Unlock()
}

func (w *yamlWatch) ResultChan() <-chan watch.Event {
	return w.ch
}

func (f *store) ConvertToTable(ctx context.Context, object runtime.Object,
	tableOptions runtime.Object) (*metav1.Table, error) {
	return f.TableConvertor.ConvertToTable(ctx, object, tableOptions)
}
func (f *store) qualifiedResourceFromContext(ctx context.Context) schema.GroupResource {
	if info, ok := genericapirequest.RequestInfoFrom(ctx); ok {
		return schema.GroupResource{Group: info.APIGroup, Resource: info.Resource}
	}
	// some implementations access storage directly and thus the context has no RequestInfo
	return f.defaultQualifiedResource
}
