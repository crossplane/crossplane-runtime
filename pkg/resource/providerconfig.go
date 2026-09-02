/*
Copyright 2020 The Crossplane Authors.

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
	"context"
	"os"

	xpv2 "github.com/crossplane/crossplane/apis/v2/core/v2"
	"github.com/spf13/afero"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	"sigs.k8s.io/controller-runtime/pkg/client"

	"github.com/crossplane/crossplane-runtime/v2/pkg/errors"
	"github.com/crossplane/crossplane-runtime/v2/pkg/meta"
)

const (
	errExtractEnv            = "cannot extract from environment variable when none specified"
	errExtractFs             = "cannot extract from filesystem when no path specified"
	errExtractSecretKey      = "cannot extract from secret key when none specified"
	errGetCredentialsSecret  = "cannot get credentials secret"
	errNoHandlerForSourceFmt = "no extraction handler registered for source: %s"
	errMissingPCRef          = "managed resource does not reference a ProviderConfig"
	errMissingPCRefKind      = "managed resource ProviderConfig reference has no Kind"
	errApplyPCU              = "cannot apply ProviderConfigUsage"
	errGetPCU                = "cannot get ProviderConfigUsage"
	errAddPCUFinalizer       = "cannot add ProviderConfigUsage finalizer"
	errRemovePCUFinalizer    = "cannot remove ProviderConfigUsage finalizer"
	// Shared by Protect and Untrack, so the wording names neither.
	errFmtPCUNotModern = "cannot resolve ProviderConfigUsage: %T is not a namespaced managed resource"
	errFmtPCUNotLegacy = "cannot resolve ProviderConfigUsage: %T is not a cluster-scoped managed resource"
)

// ProviderConfigUsageFinalizer keeps a ProviderConfigUsage alive until the
// managed resource that owns it has finished deleting its external resource.
const ProviderConfigUsageFinalizer = "finalizer.providerconfigusage.crossplane.io"

type missingRefError struct{ error }

func (m missingRefError) MissingReference() bool { return true }

// IsMissingReference returns true if an error indicates that a managed
// resource is missing a required reference..
func IsMissingReference(err error) bool {
	_, ok := err.(interface {
		MissingReference() bool
	})

	return ok
}

// EnvLookupFn looks up an environment variable.
type EnvLookupFn func(string) string

// ExtractEnv extracts credentials from an environment variable.
func ExtractEnv(_ context.Context, e EnvLookupFn, s xpv2.CommonCredentialSelectors) ([]byte, error) {
	if s.Env == nil {
		return nil, errors.New(errExtractEnv)
	}

	return []byte(e(s.Env.Name)), nil
}

// ExtractFs extracts credentials from the filesystem.
func ExtractFs(_ context.Context, fs afero.Fs, s xpv2.CommonCredentialSelectors) ([]byte, error) {
	if s.Fs == nil {
		return nil, errors.New(errExtractFs)
	}

	return afero.ReadFile(fs, s.Fs.Path)
}

// ExtractSecret extracts credentials from a Kubernetes secret.
func ExtractSecret(ctx context.Context, client client.Client, s xpv2.CommonCredentialSelectors) ([]byte, error) {
	if s.SecretRef == nil {
		return nil, errors.New(errExtractSecretKey)
	}

	secret := &corev1.Secret{}
	if err := client.Get(ctx, types.NamespacedName{Namespace: s.SecretRef.Namespace, Name: s.SecretRef.Name}, secret); err != nil {
		return nil, errors.Wrap(err, errGetCredentialsSecret)
	}

	return secret.Data[s.SecretRef.Key], nil
}

// CommonCredentialExtractor extracts credentials from common sources.
func CommonCredentialExtractor(ctx context.Context, source xpv2.CredentialsSource, client client.Client, selector xpv2.CommonCredentialSelectors) ([]byte, error) {
	switch source {
	case xpv2.CredentialsSourceEnvironment:
		return ExtractEnv(ctx, os.Getenv, selector)
	case xpv2.CredentialsSourceFilesystem:
		return ExtractFs(ctx, afero.NewOsFs(), selector)
	case xpv2.CredentialsSourceSecret:
		return ExtractSecret(ctx, client, selector)
	case xpv2.CredentialsSourceNone:
		return nil, nil
	case xpv2.CredentialsSourceInjectedIdentity:
		// There is no common injected identity extractor. Each provider must
		// implement their own.
		fallthrough
	default:
		return nil, errors.Errorf(errNoHandlerForSourceFmt, source)
	}
}

// A Tracker tracks managed resources.
type Tracker interface {
	// Track the supplied managed resource.
	Track(ctx context.Context, mg Managed) error
}

// A TrackerFn is a function that tracks managed resources.
type TrackerFn func(ctx context.Context, mg Managed) error

// Track the supplied managed resource.
func (fn TrackerFn) Track(ctx context.Context, mg Managed) error {
	return fn(ctx, mg)
}

// A LegacyTracker tracks legacy managed resources.
type LegacyTracker interface {
	// Track the supplied legacy managed resource.
	Track(ctx context.Context, mg LegacyManaged) error
}

// A LegacyTrackerFn is a function that tracks legacy managed resources.
type LegacyTrackerFn func(ctx context.Context, mg LegacyManaged) error

// Track the supplied legacy managed resource.
func (fn LegacyTrackerFn) Track(ctx context.Context, mg LegacyManaged) error {
	return fn(ctx, mg)
}

// A ModernTracker tracks modern managed resources.
type ModernTracker interface {
	// Track the supplied modern managed resource.
	Track(ctx context.Context, mg ModernManaged) error
}

// A ModernTrackerFn is a function that tracks modern managed resources.
type ModernTrackerFn func(ctx context.Context, mg ModernManaged) error

// Track the supplied modern managed resource.
func (fn ModernTrackerFn) Track(ctx context.Context, mg ModernManaged) error {
	return fn(ctx, mg)
}

// A ProviderConfigUsageCleaner protects a managed resource's
// ProviderConfigUsage while the resource still needs its ProviderConfig, and
// releases it once it doesn't.
//
// Both halves belong to the managed reconciler, so the finalizer is written and
// removed by the same component. A tracker that only creates usages - at a
// provider's connect site, which may build a fresh one per connection - never
// needs to know about the finalizer, and cannot get out of step with whatever
// removes it.
type ProviderConfigUsageCleaner interface {
	// Protect the managed resource's ProviderConfigUsage, so the garbage
	// collector can't take it while the resource still needs its
	// ProviderConfig.
	Protect(ctx context.Context, mg Managed) error

	// Untrack releases the managed resource's ProviderConfigUsage.
	Untrack(ctx context.Context, mg Managed) error
}

// ProviderConfigUsageCleanerFns protects and releases ProviderConfigUsages
// using the supplied functions.
type ProviderConfigUsageCleanerFns struct {
	ProtectFn func(ctx context.Context, mg Managed) error
	UntrackFn func(ctx context.Context, mg Managed) error
}

// Protect the supplied managed resource's ProviderConfigUsage.
func (fns ProviderConfigUsageCleanerFns) Protect(ctx context.Context, mg Managed) error {
	return fns.ProtectFn(ctx, mg)
}

// Untrack the supplied managed resource.
func (fns ProviderConfigUsageCleanerFns) Untrack(ctx context.Context, mg Managed) error {
	return fns.UntrackFn(ctx, mg)
}

// NewNopProviderConfigUsageCleaner returns a ProviderConfigUsageCleaner that
// does nothing. It is intended for managed resources that do not use a
// ProviderConfigUsage.
func NewNopProviderConfigUsageCleaner() ProviderConfigUsageCleaner {
	return ProviderConfigUsageCleanerFns{
		ProtectFn: func(context.Context, Managed) error { return nil },
		UntrackFn: func(context.Context, Managed) error { return nil },
	}
}

// preserveFinalizers copies the current object's finalizers onto the desired
// one. Apply updates with the desired object, so without this an update - a
// changed ProviderConfig reference is the only one Track makes - would strip
// the finalizer the reconciler put there.
func preserveFinalizers(_ context.Context, current, desired runtime.Object) error {
	c, ok := current.(metav1.Object)
	if !ok {
		return nil
	}

	d, ok := desired.(metav1.Object)
	if !ok {
		return nil
	}

	d.SetFinalizers(c.GetFinalizers())

	return nil
}

// A ProviderConfigUsageTracker tracks usages of a ProviderConfig by creating or
// updating the appropriate ProviderConfigUsage.
type ProviderConfigUsageTracker struct {
	client client.Client
	c      Applicator
	of     ProviderConfigUsage
}

// NewProviderConfigUsageTracker creates a ProviderConfigUsageTracker.
func NewProviderConfigUsageTracker(c client.Client, of TypedProviderConfigUsage) *ProviderConfigUsageTracker {
	return &ProviderConfigUsageTracker{client: c, c: NewAPIUpdatingApplicator(c), of: of}
}

// Track that the supplied Managed resource is using the ProviderConfig it
// references by creating or updating a ProviderConfigUsage. Track should be
// called _before_ attempting to use the ProviderConfig. This ensures the
// managed resource's usage is updated if the managed resource is updated to
// reference a misconfigured ProviderConfig. Track doesn't manage the
// ProviderConfigUsageFinalizer; the managed reconciler does, through Protect
// and Untrack.
func (u *ProviderConfigUsageTracker) Track(ctx context.Context, mg ModernManaged) error {
	//nolint:forcetypeassert // Will always be a PCU.
	pcu := u.of.DeepCopyObject().(TypedProviderConfigUsage)
	gvk := mg.GetObjectKind().GroupVersionKind()

	ref := mg.GetProviderConfigReference()
	if ref == nil {
		return missingRefError{errors.New(errMissingPCRef)}
	}

	if ref.Kind == "" {
		return missingRefError{errors.New(errMissingPCRefKind)}
	}

	pcu.SetName(string(mg.GetUID()))
	pcu.SetNamespace(mg.GetNamespace())
	pcu.SetLabels(map[string]string{xpv2.LabelKeyProviderName: ref.Name, xpv2.LabelKeyProviderKind: ref.Kind})
	pcu.SetOwnerReferences([]metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(mg, gvk))})
	pcu.SetProviderConfigReference(xpv2.ProviderConfigReference{Name: ref.Name, Kind: ref.Kind})
	pcu.SetResourceReference(xpv2.TypedReference{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		Name:       mg.GetName(),
	})
	err := u.c.Apply(ctx, pcu,
		MustBeControllableBy(mg.GetUID()),
		preserveFinalizers,
		AllowUpdateIf(func(current, _ runtime.Object) bool {
			//nolint:forcetypeassert // Will always be a PCU.
			return current.(TypedProviderConfigUsage).GetProviderConfigReference() != pcu.GetProviderConfigReference()
		}),
	)

	return errors.Wrap(Ignore(IsNotAllowed, err), errApplyPCU)
}

// Protect the supplied managed resource's ProviderConfigUsage, so the garbage
// collector can't take it while the resource still needs its ProviderConfig.
// It returns an error if the managed resource is not namespaced.
func (u *ProviderConfigUsageTracker) Protect(ctx context.Context, mg Managed) error {
	if _, ok := mg.(ModernManaged); !ok {
		return errors.Errorf(errFmtPCUNotModern, mg)
	}

	//nolint:forcetypeassert // Will always be a PCU.
	pcu := u.of.DeepCopyObject().(TypedProviderConfigUsage)
	pcu.SetName(string(mg.GetUID()))
	pcu.SetNamespace(mg.GetNamespace())

	if err := u.client.Get(ctx, client.ObjectKeyFromObject(pcu), pcu); err != nil {
		return errors.Wrap(IgnoreNotFound(err), errGetPCU)
	}

	return errors.Wrap(NewAPIFinalizer(u.client, ProviderConfigUsageFinalizer).AddFinalizer(ctx, pcu), errAddPCUFinalizer)
}

// Untrack releases the ProviderConfigUsage for the supplied managed resource.
// It returns an error if the managed resource is not namespaced.
func (u *ProviderConfigUsageTracker) Untrack(ctx context.Context, mg Managed) error {
	if _, ok := mg.(ModernManaged); !ok {
		return errors.Errorf(errFmtPCUNotModern, mg)
	}

	//nolint:forcetypeassert // Will always be a PCU.
	pcu := u.of.DeepCopyObject().(TypedProviderConfigUsage)
	pcu.SetName(string(mg.GetUID()))
	pcu.SetNamespace(mg.GetNamespace())

	if err := u.client.Get(ctx, client.ObjectKeyFromObject(pcu), pcu); err != nil {
		return errors.Wrap(IgnoreNotFound(err), errGetPCU)
	}
	return errors.Wrap(NewAPIFinalizer(u.client, ProviderConfigUsageFinalizer).RemoveFinalizer(ctx, pcu), errRemovePCUFinalizer)
}

// A LegacyProviderConfigUsageTracker tracks usages of a by creating or
// updating the appropriate LegacyProviderConfigUsage.
type LegacyProviderConfigUsageTracker struct {
	client client.Client
	c      Applicator
	of     LegacyProviderConfigUsage
}

// NewLegacyProviderConfigUsageTracker tracks usages of a by creating or
// updating the appropriate LegacyProviderConfigUsage.
func NewLegacyProviderConfigUsageTracker(c client.Client, of LegacyProviderConfigUsage) *LegacyProviderConfigUsageTracker {
	return &LegacyProviderConfigUsageTracker{client: c, c: NewAPIUpdatingApplicator(c), of: of}
}

// Track that the supplied LegacyManaged resource is using the ProviderConfig it
// references by creating or updating a ProviderConfigUsage. Track should be
// called _before_ attempting to use the ProviderConfig. This ensures the
// managed resource's usage is updated if the managed resource is updated to
// reference a misconfigured ProviderConfig. Track doesn't manage the
// ProviderConfigUsageFinalizer; the managed reconciler does, through Protect
// and Untrack.
func (u *LegacyProviderConfigUsageTracker) Track(ctx context.Context, mg LegacyManaged) error {
	//nolint:forcetypeassert // Will always be a legacy PCU.
	pcu := u.of.DeepCopyObject().(LegacyProviderConfigUsage)

	gvk := mg.GetObjectKind().GroupVersionKind()

	ref := mg.GetProviderConfigReference()
	if ref == nil {
		return missingRefError{errors.New(errMissingPCRef)}
	}

	pcu.SetName(string(mg.GetUID()))
	pcu.SetLabels(map[string]string{xpv2.LabelKeyProviderName: ref.Name})
	pcu.SetOwnerReferences([]metav1.OwnerReference{meta.AsController(meta.TypedReferenceTo(mg, gvk))})
	pcu.SetProviderConfigReference(xpv2.Reference{Name: ref.Name})
	pcu.SetResourceReference(xpv2.TypedReference{
		APIVersion: gvk.GroupVersion().String(),
		Kind:       gvk.Kind,
		Name:       mg.GetName(),
	})
	err := u.c.Apply(ctx, pcu,
		MustBeControllableBy(mg.GetUID()),
		preserveFinalizers,
		AllowUpdateIf(func(current, _ runtime.Object) bool {
			//nolint:forcetypeassert // Will always be a PCU.
			return current.(LegacyProviderConfigUsage).GetProviderConfigReference() != pcu.GetProviderConfigReference()
		}),
	)

	return errors.Wrap(Ignore(IsNotAllowed, err), errApplyPCU)
}

// Protect the supplied managed resource's ProviderConfigUsage, so the garbage
// collector can't take it while the resource still needs its ProviderConfig.
// It returns an error if the managed resource is not cluster scoped.
func (u *LegacyProviderConfigUsageTracker) Protect(ctx context.Context, mg Managed) error {
	if _, ok := mg.(LegacyManaged); !ok {
		return errors.Errorf(errFmtPCUNotLegacy, mg)
	}

	//nolint:forcetypeassert // Will always be a legacy PCU.
	pcu := u.of.DeepCopyObject().(LegacyProviderConfigUsage)
	pcu.SetName(string(mg.GetUID()))

	if err := u.client.Get(ctx, client.ObjectKeyFromObject(pcu), pcu); err != nil {
		return errors.Wrap(IgnoreNotFound(err), errGetPCU)
	}

	return errors.Wrap(NewAPIFinalizer(u.client, ProviderConfigUsageFinalizer).AddFinalizer(ctx, pcu), errAddPCUFinalizer)
}

// Untrack releases the ProviderConfigUsage for the supplied managed resource.
// It returns an error if the managed resource is not cluster scoped.
func (u *LegacyProviderConfigUsageTracker) Untrack(ctx context.Context, mg Managed) error {
	if _, ok := mg.(LegacyManaged); !ok {
		return errors.Errorf(errFmtPCUNotLegacy, mg)
	}

	//nolint:forcetypeassert // Will always be a legacy PCU.
	pcu := u.of.DeepCopyObject().(LegacyProviderConfigUsage)
	pcu.SetName(string(mg.GetUID()))

	if err := u.client.Get(ctx, client.ObjectKeyFromObject(pcu), pcu); err != nil {
		return errors.Wrap(IgnoreNotFound(err), errGetPCU)
	}
	return errors.Wrap(NewAPIFinalizer(u.client, ProviderConfigUsageFinalizer).RemoveFinalizer(ctx, pcu), errRemovePCUFinalizer)
}
