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

package controllers

import (
	"context"
	"testing"

	"github.com/go-logr/logr"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta2 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1beta2"

	ipamv1beta2 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
)

// The pool kind and API group are compared as plain strings in three places the
// compiler cannot check for us: the claim mapper, the pool controller's
// IPAddress mapper, and every ListAddressesInUse call. A rename that misses one
// of them keeps compiling and silently stops matching, so these tests pin the
// served kind and group behaviorally: the new pair is honored, the pre-rename
// kind and the pre-move group are not.

const (
	legacyPoolKind  = "UnifiIPPool"
	legacyPoolGroup = "ipam.cluster.x-k8s.io"
)

func kindTestScheme(t *testing.T) *runtime.Scheme {
	t.Helper()

	scheme := runtime.NewScheme()
	if err := v1beta2.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add v1beta2 to scheme: %v", err)
	}
	if err := ipamv1beta2.AddToScheme(scheme); err != nil {
		t.Fatalf("failed to add cluster-api ipam v1beta2 to scheme: %v", err)
	}
	return scheme
}

func claimReferencing(name, kind string) *ipamv1beta2.IPAddressClaim {
	return claimReferencingGroup(name, kind, v1beta2.GroupVersion.Group)
}

func claimReferencingGroup(name, kind, apiGroup string) *ipamv1beta2.IPAddressClaim {
	return &ipamv1beta2.IPAddressClaim{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test-ns"},
		Spec: ipamv1beta2.IPAddressClaimSpec{
			PoolRef: ipamv1beta2.IPPoolReference{
				Name:     "test-pool",
				Kind:     kind,
				APIGroup: apiGroup,
			},
		},
	}
}

func addressReferencing(name, kind string) *ipamv1beta2.IPAddress {
	return addressReferencingGroup(name, kind, v1beta2.GroupVersion.Group)
}

func addressReferencingGroup(name, kind, apiGroup string) *ipamv1beta2.IPAddress {
	return &ipamv1beta2.IPAddress{
		ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "test-ns"},
		Spec: ipamv1beta2.IPAddressSpec{
			Address: "192.168.1.10",
			ClaimRef: ipamv1beta2.IPAddressClaimReference{
				Name: name,
			},
			PoolRef: ipamv1beta2.IPPoolReference{
				Name:     "test-pool",
				Kind:     kind,
				APIGroup: apiGroup,
			},
		},
	}
}

// TestProviderAdapter_ipPoolToIPClaims_MatchesServedKind pins the claim-mapping
// path: a claim whose poolRef names the served kind in the served group is
// enqueued. A claim carrying the pre-rename kind is not, and neither is one that
// names the right kind in Cluster API's own group -- IPPool in
// ipam.cluster.x-k8s.io is somebody else's type, not ours.
func TestProviderAdapter_ipPoolToIPClaims_MatchesServedKind(t *testing.T) {
	scheme := kindTestScheme(t)
	c := fake.NewClientBuilder().WithScheme(scheme).WithObjects(
		claimReferencing("current-claim", v1beta2.IPPoolKind),
		claimReferencing("legacy-kind-claim", legacyPoolKind),
		claimReferencingGroup("legacy-group-claim", v1beta2.IPPoolKind, legacyPoolGroup),
	).Build()

	adapter := &UnifiProviderAdapter{Client: c}
	pool := &v1beta2.IPPool{ObjectMeta: metav1.ObjectMeta{Name: "test-pool", Namespace: "test-ns"}}

	got := adapter.ipPoolToIPClaims(context.Background(), pool)

	if len(got) != 1 {
		t.Fatalf("expected exactly one reconcile request, got %d: %v", len(got), got)
	}
	if got[0].Name != "current-claim" {
		t.Errorf("expected the claim with kind %q to be enqueued, got %q",
			v1beta2.IPPoolKind, got[0].Name)
	}
}

// TestIPPoolReconciler_ipAddressToIPPool_MatchesServedKind pins the pool
// controller's IPAddress mapper against the same silent-failure class, on both
// axes: an address must name the served kind AND the served group to enqueue its
// pool. IPPool in ipam.cluster.x-k8s.io belongs to somebody else.
func TestIPPoolReconciler_ipAddressToIPPool_MatchesServedKind(t *testing.T) {
	r := &IPPoolReconciler{}

	for _, tt := range []struct {
		name     string
		address  *ipamv1beta2.IPAddress
		wantReqs int
	}{
		{
			name:     "served kind in the served group enqueues the pool",
			address:  addressReferencing("current", v1beta2.IPPoolKind),
			wantReqs: 1,
		},
		{
			name:     "pre-rename kind is ignored",
			address:  addressReferencing("legacy-kind", legacyPoolKind),
			wantReqs: 0,
		},
		{
			name:     "served kind in Cluster API's group is ignored",
			address:  addressReferencingGroup("legacy-group", v1beta2.IPPoolKind, legacyPoolGroup),
			wantReqs: 0,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			got := r.ipAddressToIPPool(context.Background(), tt.address)
			if len(got) != tt.wantReqs {
				t.Errorf("got %d reconcile requests, want %d (poolRef %s/%s): %v",
					len(got), tt.wantReqs,
					tt.address.Spec.PoolRef.APIGroup, tt.address.Spec.PoolRef.Kind, got)
			}
		})
	}
}

// TestIPPoolReconciler_handleDeletion_CountsServedKind pins the
// deletion-protection path: the finalizer is only released once no IPAddress
// naming the served kind in the served group remains. An address left over from
// the old group or the old kind must not keep the pool alive.
func TestIPPoolReconciler_handleDeletion_CountsServedKind(t *testing.T) {
	for _, tt := range []struct {
		name            string
		addressKind     string
		addressGroup    string
		wantRequeue     bool
		wantFinalizerOn bool
	}{
		{
			name:         "served kind in the served group holds the pool",
			addressKind:  v1beta2.IPPoolKind,
			addressGroup: v1beta2.GroupVersion.Group,
			wantRequeue:  true, wantFinalizerOn: true,
		},
		{
			name:         "pre-rename kind does not",
			addressKind:  legacyPoolKind,
			addressGroup: v1beta2.GroupVersion.Group,
			wantRequeue:  false, wantFinalizerOn: false,
		},
		{
			name:         "served kind in Cluster API's group does not",
			addressKind:  v1beta2.IPPoolKind,
			addressGroup: legacyPoolGroup,
			wantRequeue:  false, wantFinalizerOn: false,
		},
	} {
		t.Run(tt.name, func(t *testing.T) {
			scheme := kindTestScheme(t)
			now := metav1.Now()
			pool := &v1beta2.IPPool{
				ObjectMeta: metav1.ObjectMeta{
					Name:              "test-pool",
					Namespace:         "test-ns",
					DeletionTimestamp: &now,
					Finalizers:        []string{ProtectPoolFinalizer},
				},
			}
			c := fake.NewClientBuilder().WithScheme(scheme).
				WithObjects(pool, addressReferencingGroup("addr", tt.addressKind, tt.addressGroup)).Build()

			r := &IPPoolReconciler{Client: c, Scheme: scheme}
			res, err := r.handleDeletion(context.Background(), pool, logr.Discard())
			if err != nil {
				t.Fatalf("handleDeletion returned an error: %v", err)
			}
			if got := res.RequeueAfter > 0; got != tt.wantRequeue {
				t.Errorf("requeued = %v, want %v (addresses of %s/%s)",
					got, tt.wantRequeue, tt.addressGroup, tt.addressKind)
			}
			if got := len(pool.Finalizers) > 0; got != tt.wantFinalizerOn {
				t.Errorf("finalizer retained = %v, want %v (addresses of %s/%s)",
					got, tt.wantFinalizerOn, tt.addressGroup, tt.addressKind)
			}
		})
	}
}
