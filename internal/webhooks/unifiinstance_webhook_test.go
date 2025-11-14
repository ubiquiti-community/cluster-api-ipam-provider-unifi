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
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/webhook/admission"

	v1beta2 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1beta2"
)

func TestUnifiInstanceWebhook_SetupWebhookWithManager(t *testing.T) {
	type fields struct {
		Client client.Client
	}
	type args struct {
		mgr ctrl.Manager
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
			w := &UnifiInstanceWebhook{
				Client: tt.fields.Client,
			}
			if err := w.SetupWebhookWithManager(tt.args.mgr); (err != nil) != tt.wantErr {
				t.Errorf("UnifiInstanceWebhook.SetupWebhookWithManager() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUnifiInstanceWebhook_Default(t *testing.T) {
	type fields struct {
		Client client.Client
	}
	type args struct {
		obj runtime.Object
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
			w := &UnifiInstanceWebhook{
				Client: tt.fields.Client,
			}
			if err := w.Default(context.Background(), tt.args.obj); (err != nil) != tt.wantErr {
				t.Errorf("UnifiInstanceWebhook.Default() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func TestUnifiInstanceWebhook_ValidateCreate(t *testing.T) {
	type fields struct {
		Client client.Client
	}
	type args struct {
		obj runtime.Object
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    admission.Warnings
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &UnifiInstanceWebhook{
				Client: tt.fields.Client,
			}
			got, err := w.ValidateCreate(context.Background(), tt.args.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnifiInstanceWebhook.ValidateCreate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnifiInstanceWebhook.ValidateCreate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnifiInstanceWebhook_ValidateUpdate(t *testing.T) {
	type fields struct {
		Client client.Client
	}
	type args struct {
		oldObj runtime.Object
		newObj runtime.Object
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    admission.Warnings
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &UnifiInstanceWebhook{
				Client: tt.fields.Client,
			}
			got, err := w.ValidateUpdate(context.Background(), tt.args.oldObj, tt.args.newObj)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnifiInstanceWebhook.ValidateUpdate() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnifiInstanceWebhook.ValidateUpdate() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnifiInstanceWebhook_ValidateDelete(t *testing.T) {
	type fields struct {
		Client client.Client
	}
	type args struct {
		obj runtime.Object
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    admission.Warnings
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			w := &UnifiInstanceWebhook{
				Client: tt.fields.Client,
			}
			got, err := w.ValidateDelete(context.Background(), tt.args.obj)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnifiInstanceWebhook.ValidateDelete() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnifiInstanceWebhook.ValidateDelete() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnifiInstanceWebhook_validate(t *testing.T) {
	type fields struct {
		Client client.Client
	}
	type args struct {
		instance *v1beta2.UnifiInstance
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
			w := &UnifiInstanceWebhook{
				Client: tt.fields.Client,
			}
			if err := w.validate(context.Background(), tt.args.instance); (err != nil) != tt.wantErr {
				t.Errorf("UnifiInstanceWebhook.validate() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}
