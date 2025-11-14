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
	"testing"

	"github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1beta2"
)

func TestGetIPAddress_CIDR(t *testing.T) {
	tests := []struct {
		name       string
		subnet     v1beta2.SubnetSpec
		index      int
		wantIP     string
		wantErr    bool
	}{
		{
			name:    "first IP in /24",
			subnet:  v1beta2.SubnetSpec{CIDR: "192.168.1.0/24"},
			index:   0,
			wantIP:  "192.168.1.1",
			wantErr: false,
		},
		{
			name:    "tenth IP in /24",
			subnet:  v1beta2.SubnetSpec{CIDR: "192.168.1.0/24"},
			index:   9,
			wantIP:  "192.168.1.10",
			wantErr: false,
		},
		{
			name:    "out of range",
			subnet:  v1beta2.SubnetSpec{CIDR: "192.168.1.0/30"},
			index:   10,
			wantIP:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetIPAddress(tt.subnet, 24, tt.index)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetIPAddress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.String() != tt.wantIP {
				t.Errorf("GetIPAddress() = %v, want %v", got.String(), tt.wantIP)
			}
		})
	}
}

func TestGetIPAddress_Range(t *testing.T) {
	tests := []struct {
		name    string
		subnet  v1beta2.SubnetSpec
		index   int
		wantIP  string
		wantErr bool
	}{
		{
			name:    "first IP in range",
			subnet:  v1beta2.SubnetSpec{Start: "10.1.40.10", End: "10.1.40.20"},
			index:   0,
			wantIP:  "10.1.40.10",
			wantErr: false,
		},
		{
			name:    "fifth IP in range",
			subnet:  v1beta2.SubnetSpec{Start: "10.1.40.10", End: "10.1.40.20"},
			index:   5,
			wantIP:  "10.1.40.15",
			wantErr: false,
		},
		{
			name:    "last IP in range",
			subnet:  v1beta2.SubnetSpec{Start: "10.1.40.10", End: "10.1.40.20"},
			index:   10,
			wantIP:  "10.1.40.20",
			wantErr: false,
		},
		{
			name:    "out of range",
			subnet:  v1beta2.SubnetSpec{Start: "10.1.40.10", End: "10.1.40.20"},
			index:   11,
			wantIP:  "",
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := GetIPAddress(tt.subnet, 24, tt.index)
			if (err != nil) != tt.wantErr {
				t.Errorf("GetIPAddress() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !tt.wantErr && got.String() != tt.wantIP {
				t.Errorf("GetIPAddress() = %v, want %v", got.String(), tt.wantIP)
			}
		})
	}
}

func TestIPInSubnets(t *testing.T) {
	subnets := []v1beta2.SubnetSpec{
		{CIDR: "10.1.40.0/24"},
		{Start: "10.1.50.10", End: "10.1.50.20"},
	}

	tests := []struct {
		name string
		ip   string
		want bool
	}{
		{"IP in first subnet", "10.1.40.15", true},
		{"IP in second subnet", "10.1.50.15", true},
		{"IP not in any subnet", "10.1.60.15", false},
		{"invalid IP", "not-an-ip", false},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := IPInSubnets(tt.ip, subnets, 24); got != tt.want {
				t.Errorf("IPInSubnets() = %v, want %v", got, tt.want)
			}
		})
	}
}
