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

package unifi

import (
	"context"
	"reflect"
	"testing"

	"github.com/ubiquiti-community/go-unifi/unifi"

	v1beta2 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1beta2"

	ipamv1beta1 "sigs.k8s.io/cluster-api/exp/ipam/api/v1beta1"
)

func TestNewClient(t *testing.T) {
	type args struct {
		cfg Config
	}
	tests := []struct {
		name    string
		args    args
		want    *Client
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := NewClient(tt.args.cfg)
			if (err != nil) != tt.wantErr {
				t.Errorf("NewClient() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("NewClient() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_ValidateCredentials(t *testing.T) {
	type fields struct {
		client *unifi.Client
		site   string
	}
	type args struct{}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				client: tt.fields.client,
				site:   tt.fields.site,
			}
			if err := c.ValidateCredentials(context.Background()); (err != nil) != tt.wantErr {
				t.Errorf("Client.ValidateCredentials() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestClient_GetNetwork(t *testing.T) {
	type fields struct {
		client *unifi.Client
		site   string
	}
	type args struct {
		networkID string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *unifi.Network
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				client: tt.fields.client,
				site:   tt.fields.site,
			}
			got, err := c.GetNetwork(context.Background(), tt.args.networkID)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.GetNetwork() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client.GetNetwork() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_GetOrAllocateIP(t *testing.T) {
	type fields struct {
		client *unifi.Client
		site   string
	}
	type args struct {
		networkID      string
		macAddress     string
		hostname       string
		poolSpec       *v1beta2.SubnetSpec
		addressesInUse []ipamv1beta1.IPAddress
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    *IPAllocation
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				client: tt.fields.client,
				site:   tt.fields.site,
			}
			got, err := c.GetOrAllocateIP(context.Background(), tt.args.networkID, tt.args.macAddress, tt.args.hostname, tt.args.poolSpec, tt.args.addressesInUse)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.GetOrAllocateIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client.GetOrAllocateIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_allocateNextIP(t *testing.T) {
	type fields struct {
		client *unifi.Client
		site   string
	}
	type args struct {
		network        *unifi.Network
		subnetSpec     *v1beta2.SubnetSpec
		addressesInUse []ipamv1beta1.IPAddress
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    string
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				client: tt.fields.client,
				site:   tt.fields.site,
			}
			got, err := c.allocateNextIP(tt.args.network, tt.args.subnetSpec, tt.args.addressesInUse)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.allocateNextIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if got != tt.want {
				t.Errorf("Client.allocateNextIP() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestClient_ReleaseIP(t *testing.T) {
	type fields struct {
		client *unifi.Client
		site   string
	}
	type args struct {
		networkID  string
		ipAddress  string
		macAddress string
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				client: tt.fields.client,
				site:   tt.fields.site,
			}
			if err := c.ReleaseIP(context.Background(), tt.args.networkID, tt.args.ipAddress, tt.args.macAddress); (err != nil) != tt.wantErr {
				t.Errorf("Client.ReleaseIP() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
