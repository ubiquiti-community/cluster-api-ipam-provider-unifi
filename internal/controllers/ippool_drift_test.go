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
	"strings"
	"testing"

	"github.com/go-logr/logr"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta2 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1beta2"
)

func TestConfigurationDrift(t *testing.T) {
	prefix16 := int32(16)
	prefix24 := int32(24)
	network := &v1beta2.SubnetSpec{CIDR: "10.1.0.0/16", Gateway: "10.1.0.1"}

	tests := []struct {
		name string
		spec v1beta2.IPPoolSpec
		want []string
	}{
		{
			name: "range with pool-level prefix and gateway",
			spec: v1beta2.IPPoolSpec{Subnets: []v1beta2.SubnetSpec{{Start: "10.1.40.32", End: "10.1.40.62"}}, Prefix: &prefix16, Gateway: "10.1.0.1"},
		},
		{
			name: "subnet-level overrides",
			spec: v1beta2.IPPoolSpec{Subnets: []v1beta2.SubnetSpec{{CIDR: "10.1.40.0/24", Prefix: &prefix16, Gateway: "10.1.0.1"}}, Gateway: "10.1.0.99"},
		},
		{
			name: "no gateway is not drift",
			spec: v1beta2.IPPoolSpec{Subnets: []v1beta2.SubnetSpec{{Start: "10.1.40.32", End: "10.1.40.62"}}, Prefix: &prefix16},
		},
		{
			name: "gateway differs",
			spec: v1beta2.IPPoolSpec{Subnets: []v1beta2.SubnetSpec{{Start: "10.1.40.32", End: "10.1.40.62"}}, Prefix: &prefix16, Gateway: "10.1.0.254"},
			want: []string{"gateway 10.1.0.254, the network 10.1.0.1"},
		},
		{
			name: "prefix differs",
			spec: v1beta2.IPPoolSpec{Subnets: []v1beta2.SubnetSpec{{CIDR: "10.1.40.0/24"}}, Gateway: "10.1.0.1"},
			want: []string{"prefix /24, the network /16"},
		},
		{
			name: "default prefix differs",
			spec: v1beta2.IPPoolSpec{Subnets: []v1beta2.SubnetSpec{{Start: "10.1.40.32", End: "10.1.40.62"}}, Gateway: "10.1.0.1"},
			want: []string{"prefix /24, the network /16"},
		},
		{
			name: "range outside the network",
			spec: v1beta2.IPPoolSpec{Subnets: []v1beta2.SubnetSpec{{Start: "10.2.0.10", End: "10.2.0.20"}}, Prefix: &prefix16, Gateway: "10.1.0.1"},
			want: []string{"is outside network 10.1.0.0/16"},
		},
		{
			name: "every subnet is checked",
			spec: v1beta2.IPPoolSpec{Subnets: []v1beta2.SubnetSpec{
				{Start: "10.1.40.32", End: "10.1.40.62"},
				{CIDR: "10.1.41.0/24", Prefix: &prefix24},
			}, Prefix: &prefix16, Gateway: "10.1.0.1"},
			want: []string{"subnet 10.1.41.0/24 has prefix /24"},
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := configurationDrift(&v1beta2.IPPool{Spec: tt.spec}, network)
			if len(got) != len(tt.want) {
				t.Fatalf("configurationDrift() = %q, want %d differences matching %q", got, len(tt.want), tt.want)
			}
			for i, want := range tt.want {
				if !strings.Contains(got[i], want) {
					t.Errorf("difference %d = %q, want it to contain %q", i, got[i], want)
				}
			}
		})
	}
}

// TestIPPoolReconciler_SyncFindsNoDriftOnMatchingRangePool reproduces the
// NetworkSynced=False the lab's pool reported: a start/end range with the
// pool-level /16 and gateway 10.1.0.1 on a UniFi network whose ip_subnet is
// 10.1.0.1/16. The sync compared the subnet's own empty CIDR and gateway with
// the raw ip_subnet and a gateway computed as 10.1.0.2.
func TestIPPoolReconciler_SyncFindsNoDriftOnMatchingRangePool(t *testing.T) {
	c, scheme, instance, pool := reKeyedPoolFixture(t, reKeyedController(t).URL)
	r := &IPPoolReconciler{Client: c, Scheme: scheme}

	ctx := context.Background()
	stored := &v1beta2.IPPool{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(pool), stored); err != nil {
		t.Fatal(err)
	}
	if err := r.syncWithUnifi(ctx, stored, instance, nil, logr.Discard()); err != nil {
		t.Fatalf("syncWithUnifi() error = %v", err)
	}

	got := &v1beta2.IPPool{}
	if err := c.Get(ctx, client.ObjectKeyFromObject(pool), got); err != nil {
		t.Fatal(err)
	}
	if !conditionTrue(got.Status.Conditions, ConditionNetworkSynced) {
		t.Errorf("conditions = %+v, want %s True", got.Status.Conditions, ConditionNetworkSynced)
	}
	observed := got.Status.ObservedNetworkConfiguration
	if observed == nil || observed.CIDR != "10.1.0.0/16" || observed.Gateway != "10.1.0.1" {
		t.Errorf("status.observedNetworkConfig = %+v, want 10.1.0.0/16 via 10.1.0.1", observed)
	}
}
