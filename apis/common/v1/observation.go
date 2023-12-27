/*
Copyright 2024 The Crossplane Authors.

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

// ObservedStatus contains the recent reconciliation stats
type ObservedStatus struct {
	// ObservedGeneration is the most recent resource metadata.generation
	// observed by the reconciler
	// +optional
	ObservedGeneration *int64 `json:"observedGeneration,omitempty"`

	// ObservedLabels are the most recent resource metadata.labels
	// observed by the reconciler
	// +optional
	ObservedLabels map[string]string `json:"observedLabels,omitempty"`

	// ObservedLabels are the most recent resource metadata.annotations
	// observed by the reconciler
	// +optional
	ObservedAnnotations map[string]string `json:"observedAnnotations,omitempty"`
}

// SetObservedGeneration sets the generation of the main resource
// during the last reconciliation.
func (s *ObservedStatus) SetObservedGeneration(generation int64) {
	s.ObservedGeneration = &generation
}

// GetObservedGeneration returns the last observed generation of the main resource.
func (s *ObservedStatus) GetObservedGeneration() *int64 {
	return s.ObservedGeneration
}

// SetObservedLabels set the labels observed on the main resource
// during the last reconciliation.
func (s *ObservedStatus) SetObservedLabels(labels map[string]string) {
	s.ObservedLabels = make(map[string]string)
	for k, v := range labels {
		s.ObservedLabels[k] = v
	}
}

// GetObservedLabels returns the last observed labels of the main resource.
func (s *ObservedStatus) GetObservedLabels() map[string]string {
	if s.ObservedLabels == nil {
		return nil
	}
	r := make(map[string]string)
	for k, v := range s.ObservedLabels {
		r[k] = v
	}
	return r
}

// SetObservedAnnotations set the annotations observed on the main resource
// during the last reconciliation.
func (s *ObservedStatus) SetObservedAnnotations(annotations map[string]string) {
	s.ObservedAnnotations = make(map[string]string)
	for k, v := range annotations {
		s.ObservedAnnotations[k] = v
	}
}

// GetObservedAnnotations returns the last observed annotations of the main resource.
func (s *ObservedStatus) GetObservedAnnotations() map[string]string {
	if s.ObservedAnnotations == nil {
		return nil
	}
	r := make(map[string]string)
	for k, v := range s.ObservedAnnotations {
		r[k] = v
	}
	return r
}
