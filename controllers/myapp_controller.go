/*
Copyright 2021 seacter.

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
	"github.com/go-logr/logr"
	appv1beta1 "github.com/seacter/appservice/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"
)

var (
	oldSpecAnnotations = "old/spec"
)

// MyAppReconciler reconciles a MyApp object
type MyAppReconciler struct {
	client.Client
	Log    logr.Logger
	Scheme *runtime.Scheme
}

// +kubebuilder:rbac:groups=apps,resources=deployments,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups="",resources=services,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=app.seacter.io,resources=myapps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=app.seacter.io,resources=myapps/status,verbs=get;update;patch

func (r *MyAppReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	log := r.Log.WithValues("myapp", req.NamespacedName)

	//获取Myapp实例
	var myapp appv1beta1.MyApp

	if err := r.Client.Get(ctx, req.NamespacedName, &myapp); err != nil {
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	//调谐，获取当前状态和期望状态对比

	// CreateOrUpdate Deployment
	var deploy appsv1.Deployment
	deploy.Name = myapp.Name
	deploy.Namespace = myapp.Namespace

	or, err := ctrl.CreateOrUpdate(ctx, r, &deploy, func() error {
		//调谐
		MutateDeployment(&myapp, &deploy)
		return controllerutil.SetControllerReference(&myapp, &deploy, r.Scheme)
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	log.Info("CreateOrUpdate", "Deployment", or)

	// CreateOrUpdate Service
	var svc corev1.Service
	svc.Name = myapp.Name
	svc.Namespace = myapp.Namespace

	or, err = ctrl.CreateOrUpdate(ctx, r, &svc, func() error {
		//调谐
		MutateService(&myapp, &svc)
		return controllerutil.SetControllerReference(&myapp, &svc, r.Scheme)
	})
	if err != nil {
		return ctrl.Result{}, err
	}
	log.Info("CreateOrUpdate", "Service", or)

	return ctrl.Result{}, nil
}

func (r *MyAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1beta1.MyApp{}).
		Owns(&appsv1.Deployment{}).
		Owns(&corev1.Service{}).
		Complete(r)
}
