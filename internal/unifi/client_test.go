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

// TestClient_GetOrAllocateIP tests GetOrAllocateIP function.
// TODO: Update test to match new signature with pool and claim parameters
/*
func TestClient_GetOrAllocateIP(t *testing.T) {
	type fields struct {
		client *unifi.Client
		site   string
	}
	type args struct {
		pool           *v1beta2.UnifiIPPool
		claim          *ipamv1beta2.IPAddressClaim
		networkID      string
		macAddress     string
		hostname       string
		addressesInUse []ipamv1beta2.IPAddress
	}
	tests := []struct {
		name    string
		fields  args    want    *IPAllocation
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
			got, err := c.GetOrAllocateIP(context.Background(), tt.args.pool, tt.args.claim, tt.args.networkID, tt.args.macAddress, tt.args.hostname, tt.args.addressesInUse)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.GetOrAllocateIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("Client.GetOrAllocateIP() = %v, want %v", got, tt.want)
			}
		})
	}
*/

// TestClient_allocateNextIP tests the allocateNextIP function.
// TODO: Update test to match new signature with context, pool, claim, and 4 return values (ip, prefix, gateway, error)
/*
func TestClient_allocateNextIP(t *testing.T) {
	type fields struct {
		client *unifi.Client
		site   string
	}
	type args struct {
		ctx            context.Context
		pool           *v1beta2.UnifiIPPool
		claim          *ipamv1beta2.IPAddressClaim
		network        *unifi.Network
		addressesInUse []ipamv1beta2.IPAddress
	}
	tests := []struct {
		name        string
		fields      args        wantIP      string
		wantPrefix  int32
		wantGateway string
		wantErr     bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			c := &Client{
				client: tt.fields.client,
				site:   tt.fields.site,
			}
			gotIP, gotPrefix, gotGateway, err := c.allocateNextIP(tt.args.ctx, tt.args.pool, tt.args.claim, tt.args.network, tt.args.addressesInUse)
			if (err != nil) != tt.wantErr {
				t.Errorf("Client.allocateNextIP() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if gotIP != tt.wantIP {
				t.Errorf("Client.allocateNextIP() gotIP = %v, want %v", gotIP, tt.wantIP)
			}
			if gotPrefix != tt.wantPrefix {
				t.Errorf("Client.allocateNextIP() gotPrefix = %v, want %v", gotPrefix, tt.wantPrefix)
			}
			if gotGateway != tt.wantGateway {
				t.Errorf("Client.allocateNextIP() gotGateway = %v, want %v", gotGateway, tt.wantGateway)
			}
		})
	}
*/

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
