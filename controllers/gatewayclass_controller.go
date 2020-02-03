/*

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
	"fmt"

	"github.com/go-logr/logr"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/service-apis/api/v1alpha1"
)

// GatewayClassReconciler reconciles a GatewayClass object
type GatewayClassReconciler struct {
	client.Client
	Log logr.Logger
}

// +kubebuilder:rbac:groups=networking.x.k8s.io,resources=gatewayclasses,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=networking.x.k8s.io,resources=gatewayclasses/status,verbs=get;update;patch

// Reconcile the changes.
func (r *GatewayClassReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("gatewayclass", req.NamespacedName)

	// your logic here

	var gClass v1alpha1.GatewayClass
	if err := r.Get(ctx, req.NamespacedName, &gClass); err != nil {
		notfound := client.IgnoreNotFound(err)
		if notfound != nil {
			log.Info(fmt.Sprintf("Unable to fetch GatewayClass, %s", err))
			return ctrl.Result{}, notfound
		}
		log.Info("Would do a delete operation")
		return ctrl.Result{}, nil
	}

	log.Info("Resource exists, was either created or updated")

	return ctrl.Result{}, nil
}

// SetupWithManager wires up the controller.
func (r *GatewayClassReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&v1alpha1.GatewayClass{}).
		Complete(r)
}
