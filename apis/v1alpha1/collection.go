// Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
//
// Licensed under the Apache License, Version 2.0 (the "License"). You may
// not use this file except in compliance with the License. A copy of the
// License is located at
//
//     http://aws.amazon.com/apache2.0/
//
// or in the "license" file accompanying this file. This file is distributed
// on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
// express or implied. See the License for the specific language governing
// permissions and limitations under the License.

// Code generated by ack-generate. DO NOT EDIT.

package v1alpha1

import (
	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// CollectionSpec defines the desired state of Collection.
type CollectionSpec struct {

	// Description of the collection.
	Description *string `json:"description,omitempty"`
	// Name of the collection.
	//
	// Regex Pattern: `^[a-z][a-z0-9-]+$`
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable once set"
	// +kubebuilder:validation:Required
	Name *string `json:"name"`
	// Indicates whether standby replicas should be used for a collection.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable once set"
	StandbyReplicas *string `json:"standbyReplicas,omitempty"`
	// An arbitrary set of tags (key–value pairs) to associate with the OpenSearch
	// Serverless collection.
	Tags []*Tag `json:"tags,omitempty"`
	// The type of collection.
	// +kubebuilder:validation:XValidation:rule="self == oldSelf",message="Value is immutable once set"
	Type *string `json:"type,omitempty"`
}

// CollectionStatus defines the observed state of Collection
type CollectionStatus struct {
	// All CRs managed by ACK have a common `Status.ACKResourceMetadata` member
	// that is used to contain resource sync state, account ownership,
	// constructed ARN for the resource
	// +kubebuilder:validation:Optional
	ACKResourceMetadata *ackv1alpha1.ResourceMetadata `json:"ackResourceMetadata"`
	// All CRs managed by ACK have a common `Status.Conditions` member that
	// contains a collection of `ackv1alpha1.Condition` objects that describe
	// the various terminal states of the CR and its backend AWS service API
	// resource
	// +kubebuilder:validation:Optional
	Conditions []*ackv1alpha1.Condition `json:"conditions"`
	// The Epoch time when the collection was created.
	// +kubebuilder:validation:Optional
	CreatedDate *int64 `json:"createdDate,omitempty"`
	// The unique identifier of the collection.
	//
	// Regex Pattern: `^[a-z0-9]{3,40}$`
	// +kubebuilder:validation:Optional
	ID *string `json:"id,omitempty"`
	// The Amazon Resource Name (ARN) of the KMS key with which to encrypt the collection.
	// +kubebuilder:validation:Optional
	KMSKeyARN *string `json:"kmsKeyARN,omitempty"`
	// The date and time when the collection was last modified.
	// +kubebuilder:validation:Optional
	LastModifiedDate *int64 `json:"lastModifiedDate,omitempty"`
	// The current status of the collection.
	// +kubebuilder:validation:Optional
	Status *string `json:"status,omitempty"`
}

// Collection is the Schema for the Collections API
// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
type Collection struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`
	Spec              CollectionSpec   `json:"spec,omitempty"`
	Status            CollectionStatus `json:"status,omitempty"`
}

// CollectionList contains a list of Collection
// +kubebuilder:object:root=true
type CollectionList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Collection `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Collection{}, &CollectionList{})
}
