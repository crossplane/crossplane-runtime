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

package unstructured_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/google/go-cmp/cmp"
	corev1 "k8s.io/api/core/v1"
	apierrors "k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	applycorev1 "k8s.io/client-go/applyconfigurations/core/v1"
	"k8s.io/client-go/rest"
	"sigs.k8s.io/controller-runtime/pkg/client"

	xpunstructured "github.com/crossplane/crossplane-runtime/pkg/resource/unstructured"
)

func TestApplyConfigurationsReachKubernetes(t *testing.T) {
	for _, tc := range []struct {
		name   string
		status bool
	}{
		{name: "Resource"},
		{name: "Status", status: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			path := "/api/v1/namespaces/default/pods/example"
			if tc.status {
				path += "/status"
			}
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				if r.Method != http.MethodPatch || r.URL.Path != path {
					t.Errorf("request = %s %s; want PATCH %s", r.Method, r.URL.Path, path)
				}
				if r.URL.Query().Get("fieldManager") != "compat-test" || r.URL.Query().Get("force") != "true" {
					t.Errorf("apply options lost: %s", r.URL.RawQuery)
				}
				var body map[string]any
				if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
					t.Errorf("decode apply configuration: %v", err)
				}
				want := map[string]any{
					"apiVersion": "v1", "kind": "Pod",
					"metadata": map[string]any{"name": "example", "namespace": "default"},
					"status":   map[string]any{"phase": "Pending"},
				}
				if diff := cmp.Diff(want, body); diff != "" {
					t.Errorf("apply payload (-want +got):\n%s", diff)
				}
				w.Header().Set("Content-Type", "application/json")
				_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Pod","metadata":{"name":"example","namespace":"default"},"status":{"phase":"Running"}}`))
			}))
			t.Cleanup(server.Close)
			c := kubernetesWrapper(t, server.URL)
			config := applycorev1.Pod("example", "default").WithStatus(applycorev1.PodStatus().WithPhase(corev1.PodPending))
			var err error
			if tc.status {
				err = c.Status().Apply(t.Context(), config, client.FieldOwner("compat-test"), client.ForceOwnership)
			} else {
				err = c.Apply(t.Context(), config, client.FieldOwner("compat-test"), client.ForceOwnership)
			}
			if err != nil {
				t.Fatalf("apply configuration: %v", err)
			}
			if config.Status == nil || config.Status.Phase == nil || *config.Status.Phase != corev1.PodRunning {
				t.Errorf("server response was not reflected in apply configuration: %+v", config.Status)
			}
		})
	}
}

func TestApplyPreservesKubernetesConflict(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusConflict)
		_, _ = w.Write([]byte(`{"apiVersion":"v1","kind":"Status","status":"Failure","reason":"Conflict","message":"field ownership conflict","code":409}`))
	}))
	t.Cleanup(server.Close)
	c := kubernetesWrapper(t, server.URL)
	for _, status := range []bool{false, true} {
		config := applycorev1.Pod("example", "default")
		var err error
		if status {
			err = c.Status().Apply(t.Context(), config, client.FieldOwner("compat-test"))
		} else {
			err = c.Apply(t.Context(), config, client.FieldOwner("compat-test"))
		}
		if !apierrors.IsConflict(err) {
			t.Errorf("status=%v: error = %v; want Kubernetes conflict", status, err)
		}
	}
}

func TestStatusApplyRejectsLegacyUnstructuredWrapper(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		t.Error("unsupported wrapper reached Kubernetes")
		w.WriteHeader(http.StatusInternalServerError)
	}))
	t.Cleanup(server.Close)
	c := kubernetesWrapper(t, server.URL)
	err := c.Status().Apply(t.Context(), &wrappedApplyConfiguration{PodApplyConfiguration: applycorev1.Pod("example", "default")})
	if err == nil || !strings.Contains(err.Error(), "cannot apply status for unstructured") {
		t.Errorf("error = %v; want unsupported unstructured status apply", err)
	}
}

type wrappedApplyConfiguration struct {
	*applycorev1.PodApplyConfiguration
}

// GetUnstructured identifies this configuration as a legacy unstructured wrapper.
func (*wrappedApplyConfiguration) GetUnstructured() *unstructured.Unstructured {
	return nil
}

func kubernetesWrapper(t *testing.T, host string) *xpunstructured.WrapperClient {
	t.Helper()
	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}
	mapper := meta.NewDefaultRESTMapper([]schema.GroupVersion{corev1.SchemeGroupVersion})
	mapper.Add(corev1.SchemeGroupVersion.WithKind("Pod"), meta.RESTScopeNamespace)
	c, err := client.New(&rest.Config{Host: host}, client.Options{Scheme: scheme, Mapper: mapper})
	if err != nil {
		t.Fatal(err)
	}
	return xpunstructured.NewClient(c)
}
