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

package controller

import (
	"context"
	"fmt"

	batchv1 "k8s.io/api/batch/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/runtime/serializer"
	"k8s.io/client-go/rest"
	"k8s.io/kubectl/pkg/scheme"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	//kueuev1beta1 "sigs.k8s.io/kueue/admission-controller/api/v1beta1"
	kueuev1beta1 "sigs.k8s.io/kueue/apis/kueue/v1beta1"
)

// WorkloadReconciler reconciles a Workload object
type WorkloadReconciler struct {
	client.Client
	Scheme *runtime.Scheme
}

var JobResource = schema.GroupVersionResource{Group: batchv1.GroupName,
	Version: batchv1.SchemeGroupVersion.Version, Resource: "jobs"}

//+kubebuilder:rbac:groups=kueue.x-k8s.io,resources=workloads,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kueue.x-k8s.io,resources=workloads/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kueue.x-k8s.io,resources=workloads/finalizers,verbs=update

//+kubebuilder:rbac:groups=kueue.x-k8s.io,resources=admissionchecks,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=kueue.x-k8s.io,resources=admissionchecks/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=kueue.x-k8s.io,resources=admissionchecks/finalizers,verbs=update

//+kubebuilder:rbac:groups=batch,resources=jobs,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=batch,resources=jobs/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=batch,resources=jobs/finalizers,verbs=update

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the Workload object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.17.2/pkg/reconcile

// Reconcile: set the state of the workload to Ready, if the workload is pending and nodeSelector of its job is set to a velid node group
func (r *WorkloadReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	_ = log.FromContext(ctx)
	crdClient := &rest.RESTClient{}
	config := ctrl.GetConfigOrDie()
	crdConfig := *config

	crdConfig.ContentConfig.GroupVersion = &schema.GroupVersion{Group: JobResource.Group, Version: JobResource.Version}
	crdConfig.APIPath = "/apis"
	crdConfig.NegotiatedSerializer = serializer.NewCodecFactory(scheme.Scheme)
	crdConfig.UserAgent = rest.DefaultKubernetesUserAgent()
	crdClient, _ = rest.UnversionedRESTClientFor(&crdConfig)
	wl := &kueuev1beta1.Workload{}

	if err := r.Get(ctx, req.NamespacedName, wl); err != nil {
		log.Log.Error(err, "unable to fetch Workload")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}
	if len(wl.OwnerReferences) > 0 {
		jobName := wl.OwnerReferences[0].Name
		job := batchv1.Job{}

		err := crdClient.Get().Resource("jobs").Namespace(wl.Namespace).Name(jobName).Do(context.Background()).Into(&job)
		fmt.Println(err)
		if err == nil && len(job.Spec.Template.Spec.NodeSelector) > 0 &&
			len(job.Spec.Template.Spec.NodeSelector["carbon"]) > 0 && len(wl.Status.AdmissionChecks) > 0 && wl.Status.AdmissionChecks[0].State == "Pending" {

			wl.Status.AdmissionChecks[0].State = "Ready"

			if err := r.Status().Update(ctx, wl); err != nil {
				log.Log.Error(err, "Failed to update Workload status")
				return ctrl.Result{}, err
			}

		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *WorkloadReconciler) SetupWithManager(mgr ctrl.Manager) error {

	err := ctrl.NewControllerManagedBy(mgr).
		For(&kueuev1beta1.Workload{}).
		Complete(r)
	if err != nil {
		return err
	}

	acReconciler := &acReconciler{
		client: r.Client,
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&kueuev1beta1.AdmissionCheck{}).
		Complete(acReconciler)
}
