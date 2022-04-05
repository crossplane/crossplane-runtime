/*
Copyright 2022 The Crossplane Authors.

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

package webhook

import (
	"context"

	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"
)

var _ admission.CustomValidator = &Validator{}

// NewValidator returns a new Validator with no-op defaults.
func NewValidator() *Validator {
	vc := &Validator{
		CreationChain: []ValidateCreateFn{NopValidateCreate},
		UpdateChain:   []ValidateUpdateFn{NopValidateUpdate},
		DeletionChain: []ValidateDeleteFn{NopValidateDelete},
	}
	return vc
}

// ValidateCreateFn is function type for creation validation.
type ValidateCreateFn func(ctx context.Context, obj runtime.Object) error

// ValidateUpdateFn is function type for update validation.
type ValidateUpdateFn func(ctx context.Context, oldObj, newObj runtime.Object) error

// ValidateDeleteFn is function type for deletion validation.
type ValidateDeleteFn func(ctx context.Context, obj runtime.Object) error

// No-op validator functions.
var (
	NopValidateCreate ValidateCreateFn = func(_ context.Context, _ runtime.Object) error {
		return nil
	}
	NopValidateUpdate ValidateUpdateFn = func(_ context.Context, _, _ runtime.Object) error {
		return nil
	}
	NopValidateDelete ValidateDeleteFn = func(_ context.Context, _ runtime.Object) error {
		return nil
	}
)

// Validator runs the given validation chains in order.
type Validator struct {
	CreationChain []ValidateCreateFn
	UpdateChain   []ValidateUpdateFn
	DeletionChain []ValidateDeleteFn
}

// ValidateCreate runs functions in creation chain in order.
func (vc *Validator) ValidateCreate(ctx context.Context, obj runtime.Object) error {
	for _, f := range vc.CreationChain {
		if err := f(ctx, obj); err != nil {
			return err
		}
	}
	return nil
}

// ValidateUpdate runs functions in update chain in order.
func (vc *Validator) ValidateUpdate(ctx context.Context, oldObj, newObj runtime.Object) error {
	for _, f := range vc.UpdateChain {
		if err := f(ctx, oldObj, newObj); err != nil {
			return err
		}
	}
	return nil
}

// ValidateDelete runs functions in deletion chain in order.
func (vc *Validator) ValidateDelete(ctx context.Context, obj runtime.Object) error {
	for _, f := range vc.DeletionChain {
		if err := f(ctx, obj); err != nil {
			return err
		}
	}
	return nil
}
