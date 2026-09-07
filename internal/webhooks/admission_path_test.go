/*
Copyright 2024.

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

package webhooks

import (
	"context"
	"encoding/json"
	"testing"

	admissionv1 "k8s.io/api/admission/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1beta2 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1beta2"
)

// These tests drive the real admission path: they build the handler exactly the
// way builder.WebhookBuilder does -- admission.WithDefaulter/WithValidator over
// the webhook value itself -- and feed it a genuine AdmissionRequest.
//
// That matters because the generic parameter of those constructors is inferred
// from the webhook's own method set, which is the same thing that fixes T at the
// ctrl.NewWebhookManagedBy[T] call site. Registering at an interface type (T =
// runtime.Object) compiles and registers cleanly, but the generated new() does
// reflect.TypeOf on a nil interface and panics on EVERY request, which
// Webhook.Handle turns into a 500 and failurePolicy=fail turns into a rejected
// create. Tests that only call SetupWebhookWithManager cannot see any of that.

func testScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := corev1.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add corev1 to scheme: %v", err)
	}
	if err := v1beta2.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add v1beta2 to scheme: %v", err)
	}
	return scheme
}

func createRequest(t *testing.T, obj runtime.Object) admission.Request {
	t.Helper()

	raw, err := json.Marshal(obj)
	if err != nil {
		t.Fatalf("failed to marshal object: %v", err)
	}
	return admission.Request{
		AdmissionRequest: admissionv1.AdmissionRequest{
			Operation: admissionv1.Create,
			Object:    runtime.RawExtension{Raw: raw},
		},
	}
}

// patchedPaths lists the JSON patch paths a defaulting response asks for.
func patchedPaths(resp admission.Response) []string {
	paths := make([]string, 0, len(resp.Patches))
	for _, p := range resp.Patches {
		paths = append(paths, p.Path)
	}
	return paths
}

func validUnifiInstance() *v1beta2.UnifiInstance {
	return &v1beta2.UnifiInstance{
		ObjectMeta: metav1.ObjectMeta{Name: "test-instance", Namespace: "test-ns"},
		Spec: v1beta2.UnifiInstanceSpec{
			Host:           "https://unifi.example.com",
			CredentialsRef: corev1.LocalObjectReference{Name: "unifi-creds"},
		},
	}
}

func credentialsSecret() *corev1.Secret {
	return &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "unifi-creds", Namespace: "test-ns"},
		Data:       map[string][]byte{"apiKey": []byte("test-key")},
	}
}

func TestUnifiInstanceWebhook_DefaultingAdmissionRequest(t *testing.T) {
	wh := admission.WithDefaulter(testScheme(t), &UnifiInstanceWebhook{})

	resp := wh.Handle(context.Background(), createRequest(t, validUnifiInstance()))

	if !resp.Allowed {
		t.Fatalf("defaulting webhook rejected a valid create: code=%d result=%+v",
			resp.Result.Code, resp.Result)
	}
	if got := patchedPaths(resp); len(got) == 0 {
		t.Errorf("expected a patch defaulting spec.site, got no patches")
	} else {
		found := false
		for _, p := range got {
			if p == "/spec/site" {
				found = true
			}
		}
		if !found {
			t.Errorf("expected a patch for /spec/site, got %v", got)
		}
	}
}

func TestUnifiInstanceWebhook_ValidatingAdmissionRequest(t *testing.T) {
	scheme := testScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(credentialsSecret()).Build()
	wh := admission.WithValidator(scheme, &UnifiInstanceWebhook{Client: c})

	resp := wh.Handle(context.Background(), createRequest(t, validUnifiInstance()))

	if !resp.Allowed {
		t.Fatalf("validating webhook rejected a valid create: code=%d result=%+v",
			resp.Result.Code, resp.Result)
	}
}

// TestUnifiInstanceWebhook_ValidatingAdmissionRequestDenies proves the handler
// reaches the validation logic rather than merely surviving the call.
func TestUnifiInstanceWebhook_ValidatingAdmissionRequestDenies(t *testing.T) {
	scheme := testScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).Build()
	wh := admission.WithValidator(scheme, &UnifiInstanceWebhook{Client: c})

	instance := validUnifiInstance()
	instance.Spec.Host = "ftp://unifi.example.com" // scheme must be http or https

	resp := wh.Handle(context.Background(), createRequest(t, instance))

	if resp.Allowed {
		t.Error("validating webhook allowed a create with an invalid host scheme")
	}
	if resp.Result != nil && resp.Result.Code >= 500 {
		t.Errorf("expected a denial, got a server error: code=%d result=%+v",
			resp.Result.Code, resp.Result)
	}
}

func validUnifiIPPool() *v1beta2.UnifiIPPool {
	return &v1beta2.UnifiIPPool{
		ObjectMeta: metav1.ObjectMeta{Name: "test-pool", Namespace: "test-ns"},
		Spec: v1beta2.UnifiIPPoolSpec{
			InstanceRef: corev1.ObjectReference{Name: "test-instance", Namespace: "test-ns"},
			Subnets:     []v1beta2.SubnetSpec{{CIDR: "192.168.1.0/24"}},
		},
	}
}

func TestUnifiIPPoolWebhook_DefaultingAdmissionRequest(t *testing.T) {
	wh := admission.WithDefaulter(testScheme(t), &UnifiIPPoolWebhook{})

	pool := validUnifiIPPool()
	pool.Spec.InstanceRef.Namespace = "" // the defaulter fills this from metadata.namespace

	resp := wh.Handle(context.Background(), createRequest(t, pool))

	if !resp.Allowed {
		t.Fatalf("defaulting webhook rejected a valid create: code=%d result=%+v",
			resp.Result.Code, resp.Result)
	}
	found := false
	for _, p := range patchedPaths(resp) {
		if p == "/spec/instanceRef/namespace" {
			found = true
		}
	}
	if !found {
		t.Errorf("expected a patch for /spec/instanceRef/namespace, got %v", patchedPaths(resp))
	}
}

func TestUnifiIPPoolWebhook_ValidatingAdmissionRequest(t *testing.T) {
	scheme := testScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(validUnifiInstance()).Build()
	wh := admission.WithValidator(scheme, &UnifiIPPoolWebhook{Client: c})

	resp := wh.Handle(context.Background(), createRequest(t, validUnifiIPPool()))

	if !resp.Allowed {
		t.Fatalf("validating webhook rejected a valid create: code=%d result=%+v",
			resp.Result.Code, resp.Result)
	}
}

// TestUnifiIPPoolWebhook_ValidatingAdmissionRequestDenies proves the handler
// reaches the validation logic rather than merely surviving the call.
func TestUnifiIPPoolWebhook_ValidatingAdmissionRequestDenies(t *testing.T) {
	scheme := testScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(validUnifiInstance()).Build()
	wh := admission.WithValidator(scheme, &UnifiIPPoolWebhook{Client: c})

	pool := validUnifiIPPool()
	pool.Spec.Subnets = nil // at least one subnet is required

	resp := wh.Handle(context.Background(), createRequest(t, pool))

	if resp.Allowed {
		t.Error("validating webhook allowed a create with no subnets")
	}
	if resp.Result != nil && resp.Result.Code >= 500 {
		t.Errorf("expected a denial, got a server error: code=%d result=%+v",
			resp.Result.Code, resp.Result)
	}
}
