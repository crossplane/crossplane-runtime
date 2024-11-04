package reference

// A Claim is a reference to a claim.
type Claim struct {
	// APIVersion of the referenced claim.
	APIVersion string `json:"apiVersion"`

	// Kind of the referenced claim.
	Kind string `json:"kind"`

	// Name of the referenced claim.
	Name string `json:"name"`

	// Namespace of the referenced claim.
	Namespace string `json:"namespace"`
}

// A Composite is a reference to a composite.
type Composite struct {
	// APIVersion of the referenced composite.
	APIVersion string `json:"apiVersion"`

	// Kind of the referenced composite.
	Kind string `json:"kind"`

	// Name of the referenced composite.
	Name string `json:"name"`
}
