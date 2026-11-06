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

package collection

import (
	"context"
	"testing"

	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	svcapitypes "github.com/aws-controllers-k8s/opensearchserverless-controller/apis/v1alpha1"
)

func TestCollectionGroupReferenceResolvesAWSNameAndCreatePayload(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, svcapitypes.AddToScheme(scheme))
	group := &svcapitypes.CollectionGroup{
		ObjectMeta: metav1.ObjectMeta{Name: "kubernetes-group", Namespace: "default"},
		Spec:       svcapitypes.CollectionGroupSpec{Name: aws.String("different-aws-name")},
		Status: svcapitypes.CollectionGroupStatus{Conditions: []*ackv1alpha1.Condition{{
			Type:   ackv1alpha1.ConditionTypeResourceSynced,
			Status: corev1.ConditionTrue,
		}}},
	}
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(group).Build()
	rm := &resourceManager{}
	desired := &resource{ko: &svcapitypes.Collection{
		ObjectMeta: metav1.ObjectMeta{Name: "collection", Namespace: "default"},
		Spec: svcapitypes.CollectionSpec{
			Name: aws.String("collection"),
			CollectionGroupRef: &ackv1alpha1.AWSResourceReferenceWrapper{
				From: &ackv1alpha1.AWSResourceReference{Name: aws.String("kubernetes-group")},
			},
		},
	}}

	resolved, hasReferences, err := rm.ResolveReferences(context.Background(), reader, desired)
	require.NoError(t, err)
	assert.True(t, hasReferences)
	input, err := rm.newCreateRequestPayload(context.Background(), resolved.(*resource))
	require.NoError(t, err)
	assert.Equal(t, "different-aws-name", *input.CollectionGroupName)
	cleared := rm.ClearResolvedReferences(resolved).(*resource)
	assert.Nil(t, cleared.ko.Spec.CollectionGroupName)
	require.NotNil(t, cleared.ko.Spec.CollectionGroupRef)
	assert.Equal(t, "kubernetes-group", *cleared.ko.Spec.CollectionGroupRef.From.Name)
}

func TestCollectionGroupReferenceRequiresSyncedResource(t *testing.T) {
	cases := []struct {
		name       string
		conditions []*ackv1alpha1.Condition
	}{
		{name: "no conditions"},
		{name: "not synced", conditions: []*ackv1alpha1.Condition{{
			Type: ackv1alpha1.ConditionTypeResourceSynced, Status: corev1.ConditionFalse,
		}}},
		{name: "terminal", conditions: []*ackv1alpha1.Condition{{
			Type: ackv1alpha1.ConditionTypeTerminal, Status: corev1.ConditionTrue,
		}}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			scheme := runtime.NewScheme()
			require.NoError(t, svcapitypes.AddToScheme(scheme))
			group := &svcapitypes.CollectionGroup{
				ObjectMeta: metav1.ObjectMeta{Name: "group", Namespace: "default"},
				Spec:       svcapitypes.CollectionGroupSpec{Name: aws.String("aws-group")},
				Status:     svcapitypes.CollectionGroupStatus{Conditions: tc.conditions},
			}
			reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(group).Build()
			rm := &resourceManager{}
			desired := &resource{ko: &svcapitypes.Collection{
				ObjectMeta: metav1.ObjectMeta{Namespace: "default"},
				Spec: svcapitypes.CollectionSpec{
					CollectionGroupRef: &ackv1alpha1.AWSResourceReferenceWrapper{
						From: &ackv1alpha1.AWSResourceReference{Name: aws.String("group")},
					},
				},
			}}
			resolved, hasReferences, err := rm.ResolveReferences(context.Background(), reader, desired)
			require.Error(t, err)
			assert.True(t, hasReferences)
			assert.Nil(t, resolved.(*resource).ko.Spec.CollectionGroupName)
		})
	}
}
