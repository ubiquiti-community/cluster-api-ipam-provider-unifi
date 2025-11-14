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

package poolutil

import (
	"context"
	"reflect"
	"testing"

	"go4.org/netipx"
	"sigs.k8s.io/controller-runtime/pkg/client"

	v1beta2 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1beta2"

	ipamv1beta1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1beta1"
)

func TestListAddressesInUse(t *testing.T) {
	type args struct {
		c            client.Client
		namespace    string
		poolName     string
		poolKind     string
		poolAPIGroup string
	}
	tests := []struct {
		name    string
		args    args
		want    []ipamv1beta1.IPAddress
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ListAddressesInUse(context.Background(), tt.args.c, tt.args.namespace, tt.args.poolName, tt.args.poolKind, tt.args.poolAPIGroup)
			if (err != nil) != tt.wantErr {
				t.Errorf("ListAddressesInUse() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ListAddressesInUse() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestAddressesToIPSet(t *testing.T) {
	type args struct {
		addresses []string
	}
	tests := []struct {
		name    string
		args    args
		want    *netipx.IPSet
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := AddressesToIPSet(tt.args.addresses)
			if (err != nil) != tt.wantErr {
				t.Errorf("AddressesToIPSet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("AddressesToIPSet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestPoolSpecToIPSet(t *testing.T) {
	type args struct {
		poolSpec *v1beta2.SubnetSpec
	}
	tests := []struct {
		name    string
		args    args
		want    *netipx.IPSet
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := PoolSpecToIPSet(tt.args.poolSpec)
			if (err != nil) != tt.wantErr {
				t.Errorf("PoolSpecToIPSet() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("PoolSpecToIPSet() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFindNextAvailableIP(t *testing.T) {
	type args struct {
		poolIPSet  *netipx.IPSet
		inUseIPSet *netipx.IPSet
	}
	tests := []struct {
		name    string
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := FindNextAvailableIP(tt.args.poolIPSet, tt.args.inUseIPSet)
			if (err != nil) != tt.wantErr {
				t.Errorf("FindNextAvailableIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("FindNextAvailableIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestComputePoolStatus(t *testing.T) {
	type args struct {
		poolIPSet      *netipx.IPSet
		addressesInUse []ipamv1beta1.IPAddress
		poolNamespace  string
	}
	tests := []struct {
		name string
		args args
		want *v1beta2.IPAddressStatusSummary
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ComputePoolStatus(tt.args.poolIPSet, tt.args.addressesInUse, tt.args.poolNamespace); !reflect.DeepEqual(got, tt.want) {
				t.Errorf("ComputePoolStatus() = %v, want %v", got, tt.want)
			}
		})
	}
}
