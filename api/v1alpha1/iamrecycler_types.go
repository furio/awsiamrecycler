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

package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// EDIT THIS FILE!  THIS IS SCAFFOLDING FOR YOU TO OWN!
// NOTE: json tags are required.  Any new fields you add must have json tags for the fields to be serialized.

// IAMRecyclerSpec defines the desired state of IAMRecycler
type IAMRecyclerSpec struct {
	// INSERT ADDITIONAL SPEC FIELDS - desired state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	//+kubebuilder:validation:MinLength=1
	Secret string `json:"secret"`
	//+kubebuilder:validation:MinLength=1
	DataKeyAccesskey string `json:"datakeyaccesskey"`
	//+kubebuilder:validation:MinLength=1
	DataKeySecretkey string `json:"datakeysecretkey"`
	//+kubebuilder:validation:MinLength=1
	IAMUser string `json:"iamuser"`
	//+kubebuilder:validation:Minimum=60
	Recycle int `json:"recycle"`
}

// IAMRecyclerStatus defines the observed state of IAMRecycler
type IAMRecyclerStatus struct {
	// INSERT ADDITIONAL STATUS FIELD - define observed state of cluster
	// Important: Run "make" to regenerate code after modifying this file

	// +optional
	LastRecycleTime *metav1.Time `json:"lastRecycleTime,omitempty"`
}

//+kubebuilder:object:root=true
//+kubebuilder:subresource:status

// IAMRecycler is the Schema for the iamrecyclers API
type IAMRecycler struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   IAMRecyclerSpec   `json:"spec,omitempty"`
	Status IAMRecyclerStatus `json:"status,omitempty"`
}

//+kubebuilder:object:root=true

// IAMRecyclerList contains a list of IAMRecycler
type IAMRecyclerList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []IAMRecycler `json:"items"`
}

func init() {
	SchemeBuilder.Register(&IAMRecycler{}, &IAMRecyclerList{})
}
