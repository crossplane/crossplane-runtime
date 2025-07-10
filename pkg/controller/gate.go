package controller

import (
	"k8s.io/apimachinery/pkg/runtime/schema"
)

// A Gate is an interface to allow reconcilers to delay a callback until a set of GVKs are set to true inside the gate.
type Gate interface {
	// Register to call a callback function when all given GVKs are marked true. If the callback is unblocked, the
	// registration is removed.
	Register(callback func(), gvks ...schema.GroupVersionKind)
	// True marks a given gvk true, and then looks for unlocked callbacks. Returns if there was an update or not.
	True(gvk schema.GroupVersionKind) bool
	// False marks a given gvk false. Returns if there was an update or not.
	False(gkv schema.GroupVersionKind) bool
}
