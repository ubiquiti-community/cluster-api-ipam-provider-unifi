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
	"reflect"
	"testing"

	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
)

func TestUnifiInstanceReconciler_Reconcile(t *testing.T) {
	type fields struct {
		Client client.Client
		Scheme *runtime.Scheme
	}
	type args struct {
		req ctrl.Request
	}
	tests := []struct {
		name    string
		fields  fields
		args    args
		want    ctrl.Result
		wantErr bool
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			r := &UnifiInstanceReconciler{
				Client: tt.fields.Client,
				Scheme: tt.fields.Scheme,
			}
			got, err := r.Reconcile(context.Background(), tt.args.req)
			if (err != nil) != tt.wantErr {
				t.Errorf("UnifiInstanceReconciler.Reconcile() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Errorf("UnifiInstanceReconciler.Reconcile() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestUnifiInstanceReconciler_SetupWithManager(t *testing.T) {
	type fields struct {
		Client client.Client
		Scheme *runtime.Scheme
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
			r := &UnifiInstanceReconciler{
				Client: tt.fields.Client,
				Scheme: tt.fields.Scheme,
			}
			if err := r.SetupWithManager(tt.args.mgr); (err != nil) != tt.wantErr {
				t.Errorf("UnifiInstanceReconciler.SetupWithManager() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func Test_ptr(t *testing.T) {
	type args struct {
		s string
	}
	tests := []struct {
		name string
		args args
		want *string
	}{
		// TODO: Add test cases.
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := ptr(tt.args.s); got != tt.want {
				t.Errorf("ptr() = %v, want %v", got, tt.want)
			}
		})
	}
}
