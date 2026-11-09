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
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-logr/logr"
	"github.com/ubiquiti-community/go-unifi/unifi"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	v1beta2 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1beta2"
)

const (
	rediscoveryNamespace = "tinkerbell"
	oldNetworkID         = "6531fc98540608444132dd19"
	newNetworkID         = "1991ed24-36b6-49d8-b8c6-c5e2ed76990b"
)

// reKeyedController serves the network list of a UniFi controller that has
// re-keyed the pool's network to newNetworkID.
func reKeyedController(t *testing.T) *httptest.Server {
	t.Helper()

	mux := http.NewServeMux()
	mux.HandleFunc("/proxy/network/api/s/default/rest/networkconf", func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		if err := json.NewEncoder(w).Encode(map[string]any{
			"meta": map[string]string{"rc": "ok"},
			"data": []unifi.Network{{
				ID:       newNetworkID,
				Name:     new("Internal"),
				Purpose:  unifi.PurposeCorporate,
				IPSubnet: new("10.1.0.1/16"),
				VLAN:     new(int64(10)),
			}},
		}); err != nil {
			t.Errorf("encoding networks: %v", err)
		}
	})
	srv := httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv
}

// reKeyedPoolFixture returns a Kubernetes client holding an Instance for host,
// its credentials, and a pool that discovered its network as oldNetworkID.
func reKeyedPoolFixture(t *testing.T, host string) (client.Client, *runtime.Scheme, *v1beta2.Instance, *v1beta2.IPPool) {
	t.Helper()

	scheme := runtime.NewScheme()
	for _, add := range []func(*runtime.Scheme) error{v1beta2.AddToScheme, corev1.AddToScheme} {
		if err := add(scheme); err != nil {
			t.Fatal(err)
		}
	}

	instance := &v1beta2.Instance{
		ObjectMeta: metav1.ObjectMeta{Name: "unifi", Namespace: rediscoveryNamespace},
		Spec:       v1beta2.InstanceSpec{Host: host, CredentialsRef: corev1.LocalObjectReference{Name: "unifi-credentials"}},
	}
	secret := &corev1.Secret{
		ObjectMeta: metav1.ObjectMeta{Name: "unifi-credentials", Namespace: rediscoveryNamespace},
		Data:       map[string][]byte{"apiKey": []byte("test-key")},
	}
	prefix := int32(16)
	pool := &v1beta2.IPPool{
		ObjectMeta: metav1.ObjectMeta{Name: "talos-nodes", Namespace: rediscoveryNamespace},
		Spec: v1beta2.IPPoolSpec{
			InstanceRef: corev1.ObjectReference{Name: "unifi", Namespace: rediscoveryNamespace},
			Subnets:     []v1beta2.SubnetSpec{{Start: "10.1.40.32", End: "10.1.40.62"}},
			Prefix:      &prefix,
			Gateway:     "10.1.0.1",
		},
		Status: v1beta2.IPPoolStatus{DiscoveredNetworkID: oldNetworkID},
	}
	c := fake.NewClientBuilder().WithScheme(scheme).
		WithObjects(instance, secret, pool).
		WithStatusSubresource(pool).
		Build()
	return c, scheme, instance, pool
}

func conditionTrue(conditions []metav1.Condition, conditionType string) bool {
	for _, cond := range conditions {
		if cond.Type == conditionType && cond.Status == metav1.ConditionTrue {
			return true
		}
	}
	return false
}

// TestIPPoolReconciler_SyncRediscoversReKeyedNetwork reproduces the outage after
// a UniFi Network 11 upgrade re-keyed every network to a UUID: the pool had
// discovered its network under the old ID, and sync failed on every reconcile
// with "network ... not found". The sync must discover the network again and
// record its new ID, which claims then allocate with.
func TestIPPoolReconciler_SyncRediscoversReKeyedNetwork(t *testing.T) {
	c, scheme, instance, pool := reKeyedPoolFixture(t, reKeyedController(t).URL)
	r := &IPPoolReconciler{Client: c, Scheme: scheme}

	ctx := context.Background()
	stored := &v1beta2.IPPool{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(pool), stored); err != nil {
		t.Fatal(err)
	}
	if err := r.syncWithUnifi(ctx, stored, instance, logr.Discard()); err != nil {
		t.Fatalf("syncWithUnifi() error = %v", err)
	}

	got := &v1beta2.IPPool{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(pool), got); err != nil {
		t.Fatal(err)
	}
	if got.Status.DiscoveredNetworkID != newNetworkID {
		t.Errorf("status.discoveredNetworkID = %q, want %q", got.Status.DiscoveredNetworkID, newNetworkID)
	}
	if got.Status.NetworkInfo == nil || got.Status.NetworkInfo.Name != "Internal" {
		t.Errorf("status.networkInfo = %+v, want the Internal network", got.Status.NetworkInfo)
	}
	if !conditionTrue(got.Status.Conditions, "NetworkDiscovered") {
		t.Errorf("conditions = %+v, want NetworkDiscovered True", got.Status.Conditions)
	}
}
