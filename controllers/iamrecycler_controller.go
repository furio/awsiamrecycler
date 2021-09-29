/*
Copyright 2021.

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
	"sort"
	"time"

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	awsv1alpha1 "github.com/furio/awsiamrecycler/api/v1alpha1"

	v1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/types"

	"github.com/aws/aws-sdk-go/aws"
	"github.com/aws/aws-sdk-go/aws/awserr"
	"github.com/aws/aws-sdk-go/aws/session"
	"github.com/aws/aws-sdk-go/service/iam"
)

// IAMRecyclerReconciler reconciles a IAMRecycler object
type IAMRecyclerReconciler struct {
	client.Client
	Scheme *runtime.Scheme
	Clock
}

/*
We'll mock out the clock to make it easier to jump around in time while testing,
the "real" clock just calls `time.Now`.
*/
type realClock struct{}

func (_ realClock) Now() time.Time { return time.Now() }

// clock knows how to get the current time.
// It can be used to fake out timing for testing.
type Clock interface {
	Now() time.Time
}

// +kubebuilder:docs-gen:collapse=Clock

//+kubebuilder:rbac:groups=aws.furio.me,resources=iamrecyclers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=aws.furio.me,resources=iamrecyclers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=aws.furio.me,resources=iamrecyclers/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;update;patch

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *IAMRecyclerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	iamrecycler := &awsv1alpha1.IAMRecycler{}
	err := r.Get(ctx, req.NamespacedName, iamrecycler)
	if err != nil {
		logger.Error(err, "Error while getting the object")
		return ctrl.Result{}, client.IgnoreNotFound(err)
	}

	logger.Info("IAMRecycler found", "name", iamrecycler.Name)

	if iamrecycler.Status.LastRecycleTime != nil {
		nextRun := iamrecycler.Status.LastRecycleTime.Time.Add(time.Minute * time.Duration(iamrecycler.Spec.Recycle))

		if nextRun.After(r.Now()) {
			return ctrl.Result{RequeueAfter: nextRun.Sub(r.Now())}, nil
		}
	}

	foundSecret := &corev1.Secret{}
	err = r.Get(ctx, types.NamespacedName{Name: iamrecycler.Spec.Secret, Namespace: req.NamespacedName.Namespace}, foundSecret)
	if err != nil {
		logger.Error(err, "Failed to get Secret", "secret", iamrecycler.Spec.Secret)
		return ctrl.Result{}, err
	}

	logger.Info("Secret object found", "name", foundSecret.Name)

	if foundSecret.Immutable != nil && *(foundSecret.Immutable) {
		logger.Error(err, "Secret is immutable", "secret", iamrecycler.Spec.Secret)
		return ctrl.Result{}, err
	}

	mySession := session.Must(session.NewSession())
	// Create a IAM client from just a session.
	svc := iam.New(mySession)
	input := &iam.ListAccessKeysInput{
		UserName: aws.String(iamrecycler.Spec.IAMUser),
	}

	listKeys, err := svc.ListAccessKeys(input)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			logger.Error(err, aerr.Error())
		} else {
			logger.Error(err, err.Error())
		}
		return ctrl.Result{}, err
	}

	sort.SliceStable(listKeys.AccessKeyMetadata, func(i, j int) bool {
		return listKeys.AccessKeyMetadata[i].CreateDate.Before(*listKeys.AccessKeyMetadata[j].CreateDate)
	})

	// logger.Info("AWS iam list keys", "result", listKeys.AccessKeyMetadata)

	keysLen := len(listKeys.AccessKeyMetadata)
	if keysLen == 2 {
		input := &iam.DeleteAccessKeyInput{
			AccessKeyId: listKeys.AccessKeyMetadata[0].AccessKeyId,
			UserName:    aws.String(iamrecycler.Spec.IAMUser),
		}
		_, err := svc.DeleteAccessKey(input)
		if err != nil {
			if aerr, ok := err.(awserr.Error); ok {
				logger.Error(err, aerr.Error())
			} else {
				logger.Error(err, err.Error())
			}
			return ctrl.Result{}, err
		}
	}

	newKeyInput := &iam.CreateAccessKeyInput{
		UserName: aws.String(iamrecycler.Spec.IAMUser),
	}

	newKey, err := svc.CreateAccessKey(newKeyInput)
	if err != nil {
		if aerr, ok := err.(awserr.Error); ok {
			logger.Error(err, aerr.Error())
		} else {
			logger.Error(err, err.Error())
		}
		return ctrl.Result{}, err
	}

	// Update
	data := make(map[string]string)
	data[iamrecycler.Spec.DataKeyAccesskey] = *(newKey.AccessKey.AccessKeyId)
	data[iamrecycler.Spec.DataKeySecretkey] = *(newKey.AccessKey.SecretAccessKey)

	foundSecret.StringData = data

	// Update the secret
	err = r.Update(ctx, foundSecret)
	if err != nil {
		logger.Error(err, "Failed to update Secret", "secret", foundSecret.Name, "namespace", foundSecret.Namespace)
		return ctrl.Result{}, err
	}

	// Update the status
	nextTime := v1.Now()
	iamrecycler.Status.LastRecycleTime = &nextTime
	if err := r.Status().Update(ctx, iamrecycler); err != nil {
		logger.Error(err, "unable to update IAMRecycler status")
		return ctrl.Result{}, err
	}

	logger.Info("Updated Secret and IAMRecycler", "namespace", foundSecret.Namespace, "secret", foundSecret.Name, "iamrecycler", iamrecycler.Name)

	return ctrl.Result{RequeueAfter: time.Minute * time.Duration(iamrecycler.Spec.Recycle)}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IAMRecyclerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	if r.Clock == nil {
		r.Clock = realClock{}
	}

	return ctrl.NewControllerManagedBy(mgr).
		For(&awsv1alpha1.IAMRecycler{}).
		// Owns(&corev1.Secret{}).
		Complete(r)
}
