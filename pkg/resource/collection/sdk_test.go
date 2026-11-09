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
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"testing"

	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	ackerr "github.com/aws-controllers-k8s/runtime/pkg/errors"
	ackmetrics "github.com/aws-controllers-k8s/runtime/pkg/metrics"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/opensearchserverless"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client/fake"

	svcapitypes "github.com/aws-controllers-k8s/opensearchserverless-controller/apis/v1alpha1"
)

const collectionTestARN = "arn:aws:aoss:us-west-2:123456789012:collection/collection123"

type collectionHTTPClient func(*http.Request) (*http.Response, error)

func (client collectionHTTPClient) Do(request *http.Request) (*http.Response, error) {
	return client(request)
}

func newCollectionTestManager(client collectionHTTPClient) *resourceManager {
	return &resourceManager{
		sdkapi: svcsdk.New(svcsdk.Options{
			Region:           "us-west-2",
			BaseEndpoint:     aws.String("https://aoss.example"),
			Credentials:      aws.AnonymousCredentials{},
			HTTPClient:       client,
			RetryMaxAttempts: 1,
		}),
		metrics: ackmetrics.NewMetrics("opensearchserverless"),
	}
}

func testCollection(groupName *string) *resource {
	arn := ackv1alpha1.AWSResourceName(collectionTestARN)
	return &resource{ko: &svcapitypes.Collection{
		ObjectMeta: metav1.ObjectMeta{Name: "collection", Namespace: "default"},
		Spec: svcapitypes.CollectionSpec{
			Name:                aws.String("collection"),
			Description:         aws.String("initial"),
			CollectionGroupName: groupName,
			Tags: []*svcapitypes.Tag{{
				Key: aws.String("owner"), Value: aws.String("initial"),
			}},
		},
		Status: svcapitypes.CollectionStatus{
			ID:                  aws.String("collection123"),
			Status:              aws.String("ACTIVE"),
			ACKResourceMetadata: &ackv1alpha1.ResourceMetadata{ARN: &arn},
		},
	}}
}

func TestSDKUpdateRejectsCollectionGroupChangesWithoutAWSCalls(t *testing.T) {
	cases := []struct {
		name    string
		oldName *string
		newName *string
	}{
		{name: "add group", newName: aws.String("new-group")},
		{name: "remove group", oldName: aws.String("old-group")},
		{name: "replace group", oldName: aws.String("old-group"), newName: aws.String("new-group")},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			calls := 0
			rm := newCollectionTestManager(func(request *http.Request) (*http.Response, error) {
				calls++
				return nil, errors.New("unexpected AWS call")
			})
			latest := testCollection(tc.oldName)
			desired := latest.DeepCopy().(*resource)
			desired.ko.Spec.CollectionGroupName = tc.newName
			// The unsupported group change must be rejected before either the
			// otherwise valid description update or tag synchronization.
			desired.ko.Spec.Description = aws.String("updated")
			desired.ko.Spec.Tags[0].Value = aws.String("updated")
			delta := newResourceDelta(desired, latest)

			updated, err := rm.sdkUpdate(context.Background(), desired, latest, delta)
			var terminal *ackerr.TerminalError
			require.ErrorAs(t, err, &terminal)
			assert.EqualError(t, err, "collectionGroupName can only be set when the collection is created")
			assert.Same(t, desired, updated)
			assert.Zero(t, calls, "group changes must not send update or tag requests")

			withConditions, changed := rm.updateConditions(updated, false, err)
			require.True(t, changed)
			require.Len(t, withConditions.ko.Status.Conditions, 1)
			assert.Equal(t, ackv1alpha1.ConditionTypeTerminal, withConditions.ko.Status.Conditions[0].Type)
			assert.Equal(t, corev1.ConditionTrue, withConditions.ko.Status.Conditions[0].Status)
		})
	}
}

func TestSDKUpdateRejectsRetargetedCollectionGroupReference(t *testing.T) {
	scheme := runtime.NewScheme()
	require.NoError(t, svcapitypes.AddToScheme(scheme))
	var groups []*svcapitypes.CollectionGroup
	for _, name := range []string{"old-group", "new-group"} {
		groups = append(groups, &svcapitypes.CollectionGroup{
			ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: "default"},
			Spec:       svcapitypes.CollectionGroupSpec{Name: aws.String("aws-" + name)},
			Status: svcapitypes.CollectionGroupStatus{Conditions: []*ackv1alpha1.Condition{{
				Type:   ackv1alpha1.ConditionTypeResourceSynced,
				Status: corev1.ConditionTrue,
			}}},
		})
	}
	reader := fake.NewClientBuilder().WithScheme(scheme).WithObjects(groups[0], groups[1]).Build()
	calls := 0
	rm := newCollectionTestManager(func(request *http.Request) (*http.Response, error) {
		calls++
		return nil, errors.New("unexpected AWS call")
	})
	latest := testCollection(nil)
	latest.ko.Spec.CollectionGroupRef = &ackv1alpha1.AWSResourceReferenceWrapper{
		From: &ackv1alpha1.AWSResourceReference{Name: aws.String("old-group")},
	}
	desired := latest.DeepCopy().(*resource)
	desired.ko.Spec.CollectionGroupRef.From.Name = aws.String("new-group")
	resolvedLatest, _, err := rm.ResolveReferences(context.Background(), reader, latest)
	require.NoError(t, err)
	resolvedDesired, _, err := rm.ResolveReferences(context.Background(), reader, desired)
	require.NoError(t, err)
	latest = resolvedLatest.(*resource)
	desired = resolvedDesired.(*resource)
	require.Equal(t, "aws-old-group", *latest.ko.Spec.CollectionGroupName)
	require.Equal(t, "aws-new-group", *desired.ko.Spec.CollectionGroupName)

	_, err = rm.sdkUpdate(context.Background(), desired, latest, newResourceDelta(desired, latest))
	var terminal *ackerr.TerminalError
	require.ErrorAs(t, err, &terminal)
	assert.EqualError(t, err, "collectionGroupName can only be set when the collection is created")
	assert.Zero(t, calls, "retargeted references must not send AWS update requests")
}

func TestSDKUpdateAllowsDescriptionChangeWithUnchangedCollectionGroup(t *testing.T) {
	calls := 0
	rm := newCollectionTestManager(func(request *http.Request) (*http.Response, error) {
		calls++
		assert.True(t, strings.HasSuffix(request.Header.Get("X-Amz-Target"), ".UpdateCollection"))
		input := map[string]interface{}{}
		require.NoError(t, json.NewDecoder(request.Body).Decode(&input))
		assert.Equal(t, "collection123", input["id"])
		assert.Equal(t, "updated", input["description"])
		assert.NotContains(t, input, "collectionGroupName")
		return &http.Response{
			StatusCode: http.StatusOK,
			Header:     http.Header{"Content-Type": []string{"application/x-amz-json-1.0"}},
			Body: io.NopCloser(strings.NewReader(`{"updateCollectionDetail":{
				"id":"collection123","arn":"` + collectionTestARN + `",
				"name":"collection","description":"updated","status":"ACTIVE"
			}}`)),
			Request: request,
		}, nil
	})
	latest := testCollection(aws.String("existing-group"))
	desired := latest.DeepCopy().(*resource)
	desired.ko.Spec.Description = aws.String("updated")

	updated, err := rm.sdkUpdate(context.Background(), desired, latest, newResourceDelta(desired, latest))
	require.NoError(t, err)
	require.NotNil(t, updated)
	assert.Equal(t, "updated", *updated.ko.Spec.Description)
	assert.Equal(t, "existing-group", *updated.ko.Spec.CollectionGroupName)
	assert.Equal(t, 1, calls)
}
