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

package ipamutil

import (
	"context"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/builder"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	ipamv1beta2 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
)

// otherFinalizer stands in for a finalizer another controller (CAPT) keeps on
// the claim, so a deleting claim can exist in the fake client.
const otherFinalizer = "infrastructure.cluster.x-k8s.io/ipaddressclaim"

type stubHandler struct {
	released bool
	ensured  bool
}

func (h *stubHandler) FetchPool(context.Context) (client.Object, *ctrl.Result, error) {
	return nil, nil, nil
}

func (h *stubHandler) EnsureAddress(context.Context, *ipamv1beta2.IPAddress) (*ctrl.Result, error) {
	h.ensured = true
	return nil, nil
}

func (h *stubHandler) ReleaseAddress(context.Context) (*ctrl.Result, error) {
	h.released = true
	return nil, nil
}

type stubAdapter struct{ handler *stubHandler }

func (stubAdapter) SetupWithManager(context.Context, *builder.Builder) error { return nil }

func (a stubAdapter) ClaimHandlerFor(client.Client, *ipamv1beta2.IPAddressClaim) ClaimHandler {
	return a.handler
}

// filteringAdapter is a stubAdapter that serves no claim, like a provider
// handed another provider's claim.
type filteringAdapter struct{ stubAdapter }

func (filteringAdapter) ServesClaim(*ipamv1beta2.IPAddressClaim) bool { return false }

func newDeletingClaim(finalizers ...string) *ipamv1beta2.IPAddressClaim {
	now := metav1.Now()
	return &ipamv1beta2.IPAddressClaim{
		ObjectMeta: metav1.ObjectMeta{
			Name:              "claim",
			Namespace:         "default",
			Finalizers:        finalizers,
			DeletionTimestamp: &now,
		},
		Spec: ipamv1beta2.IPAddressClaimSpec{
			PoolRef: ipamv1beta2.IPPoolReference{APIGroup: "unifi.ipam.cluster.x-k8s.io", Kind: "IPPool", Name: "pool"},
		},
	}
}

func newOwnAddress() *ipamv1beta2.IPAddress {
	return &ipamv1beta2.IPAddress{
		ObjectMeta: metav1.ObjectMeta{
			Name:       "claim",
			Namespace:  "default",
			Finalizers: []string{ProtectAddressFinalizer},
		},
		Spec: ipamv1beta2.IPAddressSpec{
			ClaimRef: ipamv1beta2.IPAddressClaimReference{Name: "claim"},
			PoolRef:  ipamv1beta2.IPPoolReference{APIGroup: "unifi.ipam.cluster.x-k8s.io", Kind: "IPPool", Name: "pool"},
			Address:  "10.1.40.37",
		},
	}
}

func reconcileClaim(t *testing.T, objs ...client.Object) (client.Client, *stubHandler) {
	t.Helper()

	return reconcileClaimWith(t, func(h *stubHandler) ProviderAdapter { return stubAdapter{handler: h} }, objs...)
}

func reconcileClaimWith(t *testing.T, adapter func(*stubHandler) ProviderAdapter, objs ...client.Object) (client.Client, *stubHandler) {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := ipamv1beta2.AddToScheme(scheme); err != nil {
		t.Fatal(err)
	}

	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(objs...).Build()
	handler := &stubHandler{}
	r := &ClaimReconciler{Client: c, Scheme: scheme, Adapter: adapter(handler)}

	if _, err := r.Reconcile(context.Background(), ctrl.Request{NamespacedName: types.NamespacedName{Namespace: "default", Name: "claim"}}); err != nil {
		t.Fatalf("Reconcile() error: %v", err)
	}

	return c, handler
}

// A claim being deleted that no longer carries the release finalizer must not
// get it back: the API server rejects the patch, so the reconcile would fail
// for as long as anything else holds the claim.
func TestReconcileDoesNotAddReleaseFinalizerToDeletingClaim(t *testing.T) {
	c, handler := reconcileClaim(t, newDeletingClaim(otherFinalizer), newOwnAddress())

	claim := &ipamv1beta2.IPAddressClaim{}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "claim"}, claim); err != nil {
		t.Fatalf("getting claim: %v", err)
	}
	if controllerutil.ContainsFinalizer(claim, ReleaseAddressFinalizer) {
		t.Fatalf("release finalizer was added to a deleting claim: %v", claim.Finalizers)
	}
	if !controllerutil.ContainsFinalizer(claim, otherFinalizer) {
		t.Fatalf("another controller's finalizer was removed: %v", claim.Finalizers)
	}
	if handler.released || handler.ensured {
		t.Fatalf("handler called for a claim released already: released=%v ensured=%v", handler.released, handler.ensured)
	}

	// The claim may not be ours at all, so neither is an address left with it.
	address := &ipamv1beta2.IPAddress{}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "claim"}, address); err != nil {
		t.Fatalf("expected the IPAddress to be left alone, got %v", err)
	}
	if !controllerutil.ContainsFinalizer(address, ProtectAddressFinalizer) {
		t.Fatalf("the IPAddress lost its finalizer: %v", address.Finalizers)
	}
}

func TestReconcileReleasesDeletingClaimWithReleaseFinalizer(t *testing.T) {
	c, handler := reconcileClaim(t, newDeletingClaim(ReleaseAddressFinalizer, otherFinalizer), newOwnAddress())

	if !handler.released {
		t.Fatal("expected the address to be released")
	}

	claim := &ipamv1beta2.IPAddressClaim{}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "claim"}, claim); err != nil {
		t.Fatalf("getting claim: %v", err)
	}
	if controllerutil.ContainsFinalizer(claim, ReleaseAddressFinalizer) {
		t.Fatalf("expected the release finalizer to be removed: %v", claim.Finalizers)
	}

	err := c.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "claim"}, &ipamv1beta2.IPAddress{})
	if !apierrors.IsNotFound(err) {
		t.Fatalf("expected the IPAddress to be deleted, got %v", err)
	}
}

func getClaim(t *testing.T, c client.Client) *ipamv1beta2.IPAddressClaim {
	t.Helper()

	claim := &ipamv1beta2.IPAddressClaim{}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "claim"}, claim); err != nil {
		t.Fatalf("getting claim: %v", err)
	}

	return claim
}

func filtering(h *stubHandler) ProviderAdapter { return filteringAdapter{stubAdapter{handler: h}} }

// Another provider's claim gets no finalizer, and the handler is not asked to
// allocate for it.
func TestReconcileLeavesClaimsTheProviderDoesNotServe(t *testing.T) {
	claim := newDeletingClaim()
	claim.DeletionTimestamp = nil
	c, handler := reconcileClaimWith(t, filtering, claim)

	if got := getClaim(t, c); controllerutil.ContainsFinalizer(got, ReleaseAddressFinalizer) {
		t.Fatalf("release finalizer added to a claim the provider does not serve: %v", got.Finalizers)
	}
	if handler.ensured || handler.released {
		t.Fatalf("handler called for a claim the provider does not serve: ensured=%v released=%v", handler.ensured, handler.released)
	}
}

// Earlier versions put the release finalizer on every claim. On another
// provider's claim it comes off, deleting or not, and nothing else is touched.
func TestReconcileRemovesReleaseFinalizerFromClaimsTheProviderDoesNotServe(t *testing.T) {
	c, handler := reconcileClaimWith(t, filtering, newDeletingClaim(ReleaseAddressFinalizer, otherFinalizer), newOwnAddress())

	got := getClaim(t, c)
	if controllerutil.ContainsFinalizer(got, ReleaseAddressFinalizer) || !controllerutil.ContainsFinalizer(got, otherFinalizer) {
		t.Fatalf("finalizers = %v, want only %s", got.Finalizers, otherFinalizer)
	}
	if handler.released {
		t.Fatal("released an address for a claim the provider does not serve")
	}

	address := &ipamv1beta2.IPAddress{}
	if err := c.Get(context.Background(), types.NamespacedName{Namespace: "default", Name: "claim"}, address); err != nil || !controllerutil.ContainsFinalizer(address, ProtectAddressFinalizer) {
		t.Fatalf("expected the other provider's IPAddress untouched, got %+v, %v", address.Finalizers, err)
	}
}
