/*
Copyright 2019 The Crossplane Authors.

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

package resource

import (
	"bytes"
	"context"

	jsonpatch "github.com/evanphx/json-patch"
	kerrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer/json"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/pkg/errors"
	"github.com/crossplane/crossplane-runtime/pkg/meta"
)

// Error strings.
const (
	errUpdateObject = "cannot update object"

	// taken from k8s.io/apiserver. Not crucial to match, but for uniformity it
	// better should.
	// TODO(sttts): import from k8s.io/apiserver/pkg/registry/generic/registry when
	//              kube has updated otel dependencies post-1.28.
	errOptimisticLock = "the object has been modified; please apply your changes to the latest version and try again"
)

// An APIPatchingApplicator applies changes to an object by either creating or
// patching it in a Kubernetes API server.
type APIPatchingApplicator struct {
	client client.Client
}

// NewAPIPatchingApplicator returns an Applicator that applies changes to an
// object by either creating or patching it in a Kubernetes API server.
func NewAPIPatchingApplicator(c client.Client) *APIPatchingApplicator {
	return &APIPatchingApplicator{client: c}
}

// Apply changes to the supplied object. The object will be created if it does
// not exist, or patched if it does. If the object does exist, it will only be
// patched if the passed object has the same or an empty resource version.
func (a *APIPatchingApplicator) Apply(ctx context.Context, obj client.Object, ao ...ApplyOption) error {
	if obj.GetName() == "" && obj.GetGenerateName() != "" {
		return a.client.Create(ctx, obj)
	}

	current := obj.DeepCopyObject().(client.Object)
	err := a.client.Get(ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, current)
	if kerrors.IsNotFound(err) {
		// TODO(negz): Apply ApplyOptions here too?
		return a.client.Create(ctx, obj)
	}
	if err != nil {
		return err
	}

	// Note: this check would ideally not be necessary if the Apply signature
	// had a current object that we could use for the diff. But we have no
	// current and for consistency of the patch it matters that the object we
	// get above is the one that was originally used.
	if obj.GetResourceVersion() != "" && obj.GetResourceVersion() != current.GetResourceVersion() {
		gvr, err := groupResource(a.client, obj)
		if err != nil {
			return err
		}
		return kerrors.NewConflict(gvr, current.GetName(), errors.New(errOptimisticLock))
	}

	for _, fn := range ao {
		if err := fn(ctx, current, obj); err != nil {
			return errors.Wrapf(err, "apply option failed for %s", HumanReadableReference(a.client, obj))
		}
	}

	return a.client.Patch(ctx, obj, client.MergeFromWithOptions(current, client.MergeFromWithOptimisticLock{}))
}

func groupResource(c client.Client, o client.Object) (schema.GroupResource, error) {
	gvk, err := c.GroupVersionKindFor(o)
	if err != nil {
		return schema.GroupResource{}, errors.Wrapf(err, "cannot determine group version kind of %T", o)
	}
	m, err := c.RESTMapper().RESTMapping(gvk.GroupKind(), gvk.Version)
	if err != nil {
		return schema.GroupResource{}, errors.Wrapf(err, "cannot determine group resource of %v", gvk)
	}
	return m.Resource.GroupResource(), nil
}

var emptyScheme = runtime.NewScheme() // no need to recognize any types
var jsonSerializer = json.NewSerializerWithOptions(json.DefaultMetaFactory, emptyScheme, emptyScheme, json.SerializerOptions{})

// AdditiveMergePatchApplyOption returns an ApplyOption that makes
// the Apply additive in the sense of a merge patch without null values. This is
// the old behavior of the APIPatchingApplicator.
//
// This only works with a desired object of type *unstructured.Unstructured.
//
// Deprecated: replace with Server Side Apply.
func AdditiveMergePatchApplyOption(_ context.Context, current, desired runtime.Object) error {
	// set GVK uniformly to the desired object to make serializer happy
	currentGVK, desiredGVK := current.GetObjectKind().GroupVersionKind(), desired.GetObjectKind().GroupVersionKind()
	if !desiredGVK.Empty() && currentGVK != desiredGVK {
		return errors.Errorf("cannot apply %v to %v", desired.GetObjectKind().GroupVersionKind(), current.GetObjectKind().GroupVersionKind())
	}
	desired.GetObjectKind().SetGroupVersionKind(currentGVK)

	// merge `desired` additively with `current`
	var currentBytes, desiredBytes bytes.Buffer
	if err := jsonSerializer.Encode(current, &currentBytes); err != nil {
		return errors.Wrapf(err, "cannot marshal current %s", HumanReadableReference(nil, current))
	}
	if err := jsonSerializer.Encode(desired, &desiredBytes); err != nil {
		return errors.Wrapf(err, "cannot marshal desired %s", HumanReadableReference(nil, desired))
	}
	mergedBytes, err := jsonpatch.MergePatch(currentBytes.Bytes(), desiredBytes.Bytes())
	if err != nil {
		return errors.Wrapf(err, "cannot merge patch to %s", HumanReadableReference(nil, desired))
	}

	// write merged object back to `desired`
	if _, _, err := jsonSerializer.Decode(mergedBytes, nil, desired); err != nil {
		return errors.Wrapf(err, "cannot unmarshal merged patch to %s", HumanReadableReference(nil, desired))
	}

	// restore empty GVK for typed objects
	if _, isUnstructured := desired.(runtime.Unstructured); !isUnstructured {
		desired.GetObjectKind().SetGroupVersionKind(schema.GroupVersionKind{})
	}

	return nil
}

// An APIUpdatingApplicator applies changes to an object by either creating or
// updating it in a Kubernetes API server.
type APIUpdatingApplicator struct {
	client client.Client
}

// NewAPIUpdatingApplicator returns an Applicator that applies changes to an
// object by either creating or updating it in a Kubernetes API server.
//
// Deprecated: Use NewAPIPatchingApplicator instead. The updating applicator
// can lead to data-loss if the Golang types in this process are not up-to-date.
func NewAPIUpdatingApplicator(c client.Client) *APIUpdatingApplicator {
	return &APIUpdatingApplicator{client: c}
}

// Apply changes to the supplied object. The object will be created if it does
// not exist, or updated if it does.
func (a *APIUpdatingApplicator) Apply(ctx context.Context, obj client.Object, ao ...ApplyOption) error {
	if obj.GetName() == "" && obj.GetGenerateName() != "" {
		return a.client.Create(ctx, obj)
	}

	current := obj.DeepCopyObject().(client.Object)
	err := a.client.Get(ctx, types.NamespacedName{Name: obj.GetName(), Namespace: obj.GetNamespace()}, current)
	if kerrors.IsNotFound(err) {
		// TODO(negz): Apply ApplyOptions here too?
		return a.client.Create(ctx, obj)
	}
	if err != nil {
		return err
	}

	for _, fn := range ao {
		if err := fn(ctx, current, obj); err != nil {
			return errors.Wrapf(err, "apply option failed for %s", HumanReadableReference(a.client, obj))
		}
	}

	return a.client.Update(ctx, obj)
}

// An APIFinalizer adds and removes finalizers to and from a resource.
type APIFinalizer struct {
	client    client.Client
	finalizer string
}

// NewNopFinalizer returns a Finalizer that does nothing.
func NewNopFinalizer() Finalizer { return nopFinalizer{} }

type nopFinalizer struct{}

func (f nopFinalizer) AddFinalizer(_ context.Context, _ Object) error {
	return nil
}
func (f nopFinalizer) RemoveFinalizer(_ context.Context, _ Object) error {
	return nil
}

// NewAPIFinalizer returns a new APIFinalizer.
func NewAPIFinalizer(c client.Client, finalizer string) *APIFinalizer {
	return &APIFinalizer{client: c, finalizer: finalizer}
}

// AddFinalizer to the supplied Managed resource.
func (a *APIFinalizer) AddFinalizer(ctx context.Context, obj Object) error {
	if meta.FinalizerExists(obj, a.finalizer) {
		return nil
	}
	meta.AddFinalizer(obj, a.finalizer)
	return errors.Wrap(a.client.Update(ctx, obj), errUpdateObject)
}

// RemoveFinalizer from the supplied Managed resource.
func (a *APIFinalizer) RemoveFinalizer(ctx context.Context, obj Object) error {
	if !meta.FinalizerExists(obj, a.finalizer) {
		return nil
	}
	meta.RemoveFinalizer(obj, a.finalizer)
	return errors.Wrap(IgnoreNotFound(a.client.Update(ctx, obj)), errUpdateObject)
}

// A FinalizerFns satisfy the Finalizer interface.
type FinalizerFns struct {
	AddFinalizerFn    func(ctx context.Context, obj Object) error
	RemoveFinalizerFn func(ctx context.Context, obj Object) error
}

// AddFinalizer to the supplied resource.
func (f FinalizerFns) AddFinalizer(ctx context.Context, obj Object) error {
	return f.AddFinalizerFn(ctx, obj)
}

// RemoveFinalizer from the supplied resource.
func (f FinalizerFns) RemoveFinalizer(ctx context.Context, obj Object) error {
	return f.RemoveFinalizerFn(ctx, obj)
}
