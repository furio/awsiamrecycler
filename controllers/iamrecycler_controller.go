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

	corev1 "k8s.io/api/core/v1"
	ctrl "sigs.k8s.io/controller-runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/log"

	awsv1alpha1 "github.com/furio/awsiamrecycler/api/v1alpha1"

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
}

//+kubebuilder:rbac:groups=aws.furio.me,resources=iamrecyclers,verbs=get;list;watch;create;update;patch;delete
//+kubebuilder:rbac:groups=aws.furio.me,resources=iamrecyclers/status,verbs=get;update;patch
//+kubebuilder:rbac:groups=aws.furio.me,resources=iamrecyclers/finalizers,verbs=update
//+kubebuilder:rbac:groups=core,resources=secrets,verbs=get;list;watch;create;update;patch;delete

// Reconcile is part of the main kubernetes reconciliation loop which aims to
// move the current state of the cluster closer to the desired state.
// TODO(user): Modify the Reconcile function to compare the state specified by
// the IAMRecycler object against the actual cluster state, and then
// perform operations to make the cluster state reflect the state specified by
// the user.
//
// For more details, check Reconcile and its Result here:
// - https://pkg.go.dev/sigs.k8s.io/controller-runtime@v0.9.2/pkg/reconcile
func (r *IAMRecyclerReconciler) Reconcile(ctx context.Context, req ctrl.Request) (ctrl.Result, error) {
	logger := log.FromContext(ctx)

	iamrecycler := &awsv1alpha1.IAMRecycler{}
	err := r.Get(ctx, req.NamespacedName, iamrecycler)
	if err != nil {
		logger.Error(err, "Error while getting the object")
		return ctrl.Result{}, err
	}

	logger.Info("Iam object found", "name", iamrecycler.Name)

	foundSecret := &corev1.Secret{}
	err = r.Get(ctx, types.NamespacedName{Name: iamrecycler.Spec.Secret, Namespace: req.NamespacedName.Namespace}, foundSecret)
	if err != nil {
		logger.Error(err, "Failed to get Secret", "secret", iamrecycler.Spec.Secret)
		return ctrl.Result{}, err
	}

	logger.Info("Secret object found", "secret", foundSecret, "data", foundSecret.Data)

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

	logger.Info("Iam results", "result", listKeys.AccessKeyMetadata)

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

	// TODO: Enable it to test
	if false {
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

		logger.Info("Newkeys", "result", newKey)

		// Update
		foundSecret.StringData["AWS_ACCESS_KEY_ID"] = *(newKey.AccessKey.AccessKeyId)
		foundSecret.StringData["AWS_SECRET_ACCESS_KEY"] = *(newKey.AccessKey.SecretAccessKey)

		// Update the secret
		err = r.Update(ctx, foundSecret)
		if err != nil {
			logger.Error(err, "Failed to update Secret", "secret", foundSecret.Name, "namespace", foundSecret.Namespace)
			return ctrl.Result{}, err
		}
	}

	return ctrl.Result{}, nil
}

// SetupWithManager sets up the controller with the Manager.
func (r *IAMRecyclerReconciler) SetupWithManager(mgr ctrl.Manager) error {
	return ctrl.NewControllerManagedBy(mgr).
		For(&awsv1alpha1.IAMRecycler{}).
		// Owns(&corev1.Secret{}).
		Complete(r)
}
