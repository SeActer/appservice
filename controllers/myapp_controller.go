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
	corev1 "k8s.io/api/core/v1"
	"k8s.io/apimachinery/pkg/api/errors"
	"k8s.io/apimachinery/pkg/util/json"
	"k8s.io/client-go/util/retry"
	"reflect"

	"github.com/go-logr/logr"
	appv1beta1 "github.com/seacter/appservice/api/v1beta1"
	appsv1 "k8s.io/api/apps/v1"
	"k8s.io/apimachinery/pkg/runtime"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
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

// +kubebuilder:rbac:groups=app.seacter.io,resources=myapps,verbs=get;list;watch;create;update;patch;delete
// +kubebuilder:rbac:groups=app.seacter.io,resources=myapps/status,verbs=get;update;patch

func (r *MyAppReconciler) Reconcile(req ctrl.Request) (ctrl.Result, error) {
	ctx := context.Background()
	_ = r.Log.WithValues("myapp", req.NamespacedName)

	//获取Myapp实例
	var myapp appv1beta1.MyApp

	err := r.Client.Get(ctx, req.NamespacedName, &myapp)
	if err != nil {
		if client.IgnoreNotFound(err) != nil {
			return ctrl.Result{}, err
		}
		//删除一个不存在的对象的时候，可能会报not-found的错误
		// 这种情况下不需要重新入队列修复
		return ctrl.Result{}, nil
	}
	//当前对象标记为删除
	if myapp.DeletionTimestamp != nil {
		return ctrl.Result{}, nil
	}

	//如果不存在关联的资源，是不是应该去创建
	//如果存在关联的资源，是不是要判断是否需要更新
	deploy := &appsv1.Deployment{}
	if err := r.Client.Get(ctx, req.NamespacedName, deploy); err != nil && errors.IsNotFound(err) {
		data, err := json.Marshal(myapp.Spec)
		if err != nil {
			return ctrl.Result{}, err
		}
		if myapp.Annotations != nil {
			myapp.Annotations[oldSpecAnnotations] = string(data)
		} else {
			myapp.Annotations = map[string]string{
				oldSpecAnnotations: string(data),
			}
		}
		//重新更新myapp
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			return r.Client.Update(ctx, &myapp)
		}); err != nil {
			return ctrl.Result{}, err
		}

		//deployment不存在,创建deployment
		newDeploy := NewDeploy(&myapp)
		if err := r.Client.Create(ctx, newDeploy); err != nil {
			return ctrl.Result{}, err
		}
		//直接创建svc
		newService := NewService(&myapp)
		if err := r.Client.Create(ctx, newService); err != nil {
			return ctrl.Result{}, err
		}

		//创建成功
		return ctrl.Result{}, nil

	}

	//TODO 更新,判断是否需要更新（yaml文件是不是变化了）
	//yaml-> old yaml 我们可以从annotations里面获取
	oldSpec := appv1beta1.MyAppSpec{}
	if err := json.Unmarshal([]byte(myapp.Annotations[oldSpecAnnotations]), &oldSpec); err != nil {
		return ctrl.Result{}, err
	}

	//和新旧对象进行比较
	if !reflect.DeepEqual(myapp.Spec, oldSpec) {
		//更新关联资源
		newDeploy := NewDeploy(&myapp)
		oldDeploy := &appsv1.Deployment{}
		if err := r.Client.Get(ctx, req.NamespacedName, oldDeploy); err != nil {
			return ctrl.Result{}, err
		}
		oldDeploy.Spec = newDeploy.Spec
		//正常直接去更新
		//一般情况下不会直接调用update更新
		//r.Client.Update(ctx,oldDeploy)
		if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
			return r.Client.Update(ctx, oldDeploy)
		}); err != nil {
			return ctrl.Result{}, err
		}
	}

	//Service

	newService := NewService(&myapp)
	oldService := &corev1.Service{}
	if err := r.Client.Get(ctx, req.NamespacedName, oldService); err != nil {
		return ctrl.Result{}, err
	}
	newService.Spec.ClusterIP = oldService.Spec.ClusterIP
	oldService.Spec = newService.Spec
	if err := retry.RetryOnConflict(retry.DefaultRetry, func() error {
		return r.Client.Update(ctx, oldService)
	}); err != nil {
		return ctrl.Result{}, err
	}

	return ctrl.Result{}, nil
}

func (r *MyAppReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&appv1beta1.MyApp{}).
		Complete(r)
}
