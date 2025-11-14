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

package main

import (
	"context"
	"flag"
	"fmt"
	"os"

	"k8s.io/apimachinery/pkg/runtime"
	utilruntime "k8s.io/apimachinery/pkg/util/runtime"
	clientgoscheme "k8s.io/client-go/kubernetes/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/healthz"
	"sigs.k8s.io/controller-runtime/pkg/log/zap"
	metricsserver "sigs.k8s.io/controller-runtime/pkg/metrics/server"
	"sigs.k8s.io/controller-runtime/pkg/webhook"

	v1beta2 "github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/api/v1beta2"
	"github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/internal/controllers"
	"github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/internal/webhooks"
	"github.com/ubiquiti-community/cluster-api-ipam-provider-unifi/pkg/ipamutil"

	clusterv1beta2 "sigs.k8s.io/cluster-api/api/core/v1beta2"
	ipamv1beta2 "sigs.k8s.io/cluster-api/api/ipam/v1beta2"
)

var (
	scheme   = runtime.NewScheme()
	setupLog = ctrl.Log.WithName("setup")
)

func init() {
	utilruntime.Must(clientgoscheme.AddToScheme(scheme))
	utilruntime.Must(v1beta2.AddToScheme(scheme))
	utilruntime.Must(ipamv1beta2.AddToScheme(scheme))
	utilruntime.Must(clusterv1beta2.AddToScheme(scheme))
}

type managerConfig struct {
	metricsAddr          string
	probeAddr            string
	enableWebhook        bool
	webhookPort          int
	webhookCertDir       string
	enableLeaderElection bool
	watchFilterValue     string
}

func parseFlags() *managerConfig {
	config := &managerConfig{}

	flag.StringVar(&config.metricsAddr, "metrics-bind-address", ":8080", "The address the metric endpoint binds to.")
	flag.StringVar(&config.probeAddr, "health-probe-bind-address", ":8081", "The address the probe endpoint binds to.")
	flag.BoolVar(&config.enableWebhook, "enable-webhook", false,
		"Enable webhook server for validation and mutation. "+
			"When disabled, the provider works without admission webhooks (recommended for CAPI Operator).")
	flag.IntVar(&config.webhookPort, "webhook-port", 9443, "The port the webhook server binds to (only used when --enable-webhook=true).")
	flag.StringVar(&config.webhookCertDir, "webhook-cert-dir", "/tmp/k8s-webhook-server/serving-certs",
		"The directory that contains the webhook server key and certificate (only used when --enable-webhook=true).")
	flag.BoolVar(&config.enableLeaderElection, "leader-elect", false,
		"Enable leader election for controller manager. "+
			"Enabling this will ensure there is only one active controller manager.")
	flag.StringVar(&config.watchFilterValue, "watch-filter", "",
		"Label value that the controller watches to reconcile cluster-api objects. "+
			"Label key is always "+clusterv1beta2.WatchLabel+". If unspecified, the controller watches for all cluster-api objects.")

	opts := zap.Options{Development: true}
	opts.BindFlags(flag.CommandLine)
	flag.Parse()

	ctrl.SetLogger(zap.New(zap.UseFlagOptions(&opts)))

	return config
}

func main() {
	config := parseFlags()

	// Build manager options
	mgrOptions := ctrl.Options{
		Scheme:                  scheme,
		Metrics:                 metricsserver.Options{BindAddress: config.metricsAddr},
		HealthProbeBindAddress:  config.probeAddr,
		LeaderElection:          config.enableLeaderElection,
		LeaderElectionID:        "unifi-ipam-controller-leader-election",
		LeaderElectionNamespace: "",
	}

	// Conditionally enable webhook server
	if config.enableWebhook {
		setupLog.Info("webhook server enabled", "port", config.webhookPort, "certDir", config.webhookCertDir)
		mgrOptions.WebhookServer = webhook.NewServer(webhook.Options{
			Port:    config.webhookPort,
			CertDir: config.webhookCertDir,
		})
	} else {
		setupLog.Info("webhook server disabled - running without admission webhooks")
	}

	mgr, err := ctrl.NewManager(ctrl.GetConfigOrDie(), mgrOptions)
	if err != nil {
		setupLog.Error(err, "unable to start manager")
		os.Exit(1)
	}

	ctx := ctrl.SetupSignalHandler()
	if err := setupControllers(mgr, config, ctx); err != nil {
		setupLog.Error(err, "unable to setup controllers")
		os.Exit(1)
	}

	setupLog.Info("starting manager")
	if err := mgr.Start(ctx); err != nil {
		setupLog.Error(err, "problem running manager")
		os.Exit(1)
	}
}

func setupControllers(mgr ctrl.Manager, config *managerConfig, ctx context.Context) error {
	// Setup UnifiInstance controller.
	if err := (&controllers.UnifiInstanceReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller UnifiInstance: %w", err)
	}

	// Setup UnifiIPPool controller.
	if err := (&controllers.UnifiIPPoolReconciler{
		Client: mgr.GetClient(),
		Scheme: mgr.GetScheme(),
	}).SetupWithManager(mgr); err != nil {
		return fmt.Errorf("unable to create controller UnifiIPPool: %w", err)
	}

	// Setup IPAddressClaim controller with UnifiProviderAdapter.
	if err := (&ipamutil.ClaimReconciler{
		Client:           mgr.GetClient(),
		Scheme:           mgr.GetScheme(),
		WatchFilterValue: config.watchFilterValue,
		Adapter:          &controllers.UnifiProviderAdapter{Client: mgr.GetClient()},
	}).SetupWithManager(ctx, mgr); err != nil {
		return fmt.Errorf("unable to create controller IPAddressClaim: %w", err)
	}

	// Setup webhooks if enabled.
	if config.enableWebhook {
		setupLog.Info("setting up webhooks")
		if err := (&webhooks.UnifiIPPoolWebhook{}).SetupWebhookWithManager(mgr); err != nil {
			return fmt.Errorf("unable to create webhook UnifiIPPool: %w", err)
		}
		if err := (&webhooks.UnifiInstanceWebhook{}).SetupWebhookWithManager(mgr); err != nil {
			return fmt.Errorf("unable to create webhook UnifiInstance: %w", err)
		}
	}

	// Add health checks.
	if err := mgr.AddHealthzCheck("healthz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up health check: %w", err)
	}
	if err := mgr.AddReadyzCheck("readyz", healthz.Ping); err != nil {
		return fmt.Errorf("unable to set up ready check: %w", err)
	}

	return nil
}
