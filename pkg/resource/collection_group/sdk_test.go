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

package collection_group

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
	smithy "github.com/aws/smithy-go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	corev1 "k8s.io/api/core/v1"

	svcapitypes "github.com/aws-controllers-k8s/opensearchserverless-controller/apis/v1alpha1"
)

const groupARN = "arn:aws:aoss:us-west-2:123456789012:collection-group/group123"

type stubResponse struct {
	operation string
	body      string
	status    int
}

type sdkRequest struct {
	operation string
	input     map[string]interface{}
}

// sdkHTTPStub exercises real SDK serialization/deserialization without opening
// a port, obtaining credentials, or sending a request to AWS.
type sdkHTTPStub struct {
	t         *testing.T
	responses []stubResponse
	requests  []sdkRequest
}

func (stub *sdkHTTPStub) Do(req *http.Request) (*http.Response, error) {
	stub.t.Helper()
	parts := strings.Split(req.Header.Get("X-Amz-Target"), ".")
	operation := parts[len(parts)-1]
	input := map[string]interface{}{}
	require.NoError(stub.t, json.NewDecoder(req.Body).Decode(&input))
	index := len(stub.requests)
	stub.requests = append(stub.requests, sdkRequest{operation: operation, input: input})
	require.Less(stub.t, index, len(stub.responses), "unexpected SDK request: %s", operation)
	response := stub.responses[index]
	require.Equal(stub.t, response.operation, operation)
	status := response.status
	if status == 0 {
		status = http.StatusOK
	}
	return &http.Response{
		StatusCode: status,
		Status:     http.StatusText(status),
		Header:     http.Header{"Content-Type": []string{"application/x-amz-json-1.0"}},
		Body:       io.NopCloser(strings.NewReader(response.body)),
		Request:    req,
	}, nil
}

func newTestManager(t *testing.T, responses ...stubResponse) (*resourceManager, *sdkHTTPStub) {
	t.Helper()
	stub := &sdkHTTPStub{t: t, responses: responses}
	client := svcsdk.New(svcsdk.Options{
		Region:           "us-west-2",
		BaseEndpoint:     aws.String("https://aoss.example"),
		Credentials:      aws.AnonymousCredentials{},
		HTTPClient:       stub,
		RetryMaxAttempts: 1,
	})
	return &resourceManager{
		sdkapi:       client,
		metrics:      ackmetrics.NewMetrics("opensearchserverless"),
		awsAccountID: ackv1alpha1.AWSAccountID("123456789012"),
		awsRegion:    ackv1alpha1.AWSRegion("us-west-2"),
		awsPartition: ackv1alpha1.AWSPartition("aws"),
	}, stub
}

func testGroup() *resource {
	arn := ackv1alpha1.AWSResourceName(groupARN)
	return &resource{ko: &svcapitypes.CollectionGroup{
		Spec: svcapitypes.CollectionGroupSpec{
			Name:            aws.String("aws-group"),
			StandbyReplicas: aws.String("DISABLED"),
			Generation:      aws.String("CLASSIC"),
			Description:     aws.String("initial"),
			CapacityLimits:  testLimits(),
			Tags: []*svcapitypes.Tag{
				{Key: aws.String("owner"), Value: aws.String("initial")},
				{Key: aws.String("remove-me"), Value: aws.String("initial")},
			},
		},
		Status: svcapitypes.CollectionGroupStatus{
			ID:                  aws.String("group123"),
			ACKResourceMetadata: &ackv1alpha1.ResourceMetadata{ARN: &arn},
		},
	}}
}

func testLimits() *svcapitypes.CollectionGroupCapacityLimits {
	return &svcapitypes.CollectionGroupCapacityLimits{
		MaxIndexingCapacityInOCU: aws.Float64(4),
		MaxSearchCapacityInOCU:   aws.Float64(6),
		MinIndexingCapacityInOCU: aws.Float64(1),
		MinSearchCapacityInOCU:   aws.Float64(2),
	}
}

func TestSDKFindReadsGroupAndEveryMemberPage(t *testing.T) {
	rm, stub := newTestManager(t,
		stubResponse{
			operation: "BatchGetCollectionGroup",
			body: `{"collectionGroupDetails":[{
				"id":"group123","arn":"` + groupARN + `","name":"aws-group",
				"standbyReplicas":"DISABLED","generation":"CLASSIC","createdDate":123,
				"description":"observed",
				"capacityLimits":{"maxIndexingCapacityInOCU":4,"maxSearchCapacityInOCU":6,
					"minIndexingCapacityInOCU":1,"minSearchCapacityInOCU":2},
				"currentCapacity":{"search":{"capacityInOcu":1.75,"autoscalingStatus":"NO_ACTION"},
					"indexing":{"capacityInOcu":1.5,"autoscalingStatus":"NO_ACTION"}},
				"numberOfCollections":2
			}]}`,
		},
		stubResponse{
			operation: "ListTagsForResource",
			body:      `{"tags":[{"key":"owner","value":"test"}]}`,
		},
		stubResponse{
			operation: "ListCollections",
			body: `{"collectionSummaries":[{"id":"collection1","name":"first",
				"arn":"arn:aws:aoss:us-west-2:123456789012:collection/collection1",
				"kmsKeyArn":"arn:aws:kms:us-west-2:123456789012:key/key1",
				"collectionGroupName":"aws-group","status":"ACTIVE"}],"nextToken":"page2"}`,
		},
		stubResponse{
			operation: "ListCollections",
			body:      `{"collectionSummaries":[{"id":"collection2","name":"second","collectionGroupName":"aws-group","status":"CREATING"}]}`,
		},
	)

	latest, err := rm.sdkFind(context.Background(), testGroup())
	require.NoError(t, err)
	require.Len(t, stub.requests, 4)
	assert.Equal(t, []interface{}{"group123"}, stub.requests[0].input["ids"])
	assert.NotContains(t, stub.requests[0].input, "names")
	assert.Equal(t, groupARN, stub.requests[1].input["resourceArn"])
	assert.Equal(t, map[string]interface{}{"collectionGroupName": "aws-group"}, stub.requests[2].input["collectionFilters"])
	assert.NotContains(t, stub.requests[2].input, "nextToken")
	assert.Equal(t, "page2", stub.requests[3].input["nextToken"])
	assert.Equal(t, stub.requests[2].input["collectionFilters"], stub.requests[3].input["collectionFilters"])
	assert.Equal(t, "observed", *latest.ko.Spec.Description)
	assert.Equal(t, testLimits(), latest.ko.Spec.CapacityLimits)
	assert.Equal(t, ackv1alpha1.AWSResourceName(groupARN), *latest.ko.Status.ACKResourceMetadata.ARN)
	assert.Equal(t, int64(123), *latest.ko.Status.CreatedDate)
	assert.Equal(t, int64(2), *latest.ko.Status.NumberOfCollections)
	assert.Equal(t, 1.75, *latest.ko.Status.CurrentCapacity.Search.CapacityInOcu)
	assert.Equal(t, 1.5, *latest.ko.Status.CurrentCapacity.Indexing.CapacityInOcu)
	assert.Equal(t, "NO_ACTION", *latest.ko.Status.CurrentCapacity.Search.AutoscalingStatus)
	require.Len(t, latest.ko.Spec.Tags, 1)
	assert.Equal(t, "owner", *latest.ko.Spec.Tags[0].Key)
	assert.Equal(t, "test", *latest.ko.Spec.Tags[0].Value)
	require.Len(t, latest.ko.Status.MemberCollections, 2)
	member := latest.ko.Status.MemberCollections[0]
	assert.Equal(t, "collection1", *member.ID)
	assert.Equal(t, "first", *member.Name)
	assert.Equal(t, "ACTIVE", *member.Status)
	assert.Equal(t, "aws-group", *member.CollectionGroupName)
	assert.Equal(t, "arn:aws:aoss:us-west-2:123456789012:collection/collection1", *member.ARN)
	assert.Equal(t, "arn:aws:kms:us-west-2:123456789012:key/key1", *member.KMSKeyARN)
	assert.Equal(t, "collection2", *latest.ko.Status.MemberCollections[1].ID)
}

func TestSDKFindTagReadOverridesBatchTags(t *testing.T) {
	rm, stub := newTestManager(t,
		stubResponse{
			operation: "BatchGetCollectionGroup",
			body: `{"collectionGroupDetails":[{"id":"group123","arn":"` + groupARN + `",
				"name":"aws-group","tags":[{"key":"owner","value":"stale"}]}]}`,
		},
		stubResponse{operation: "ListTagsForResource", body: `{"tags":[{"key":"owner","value":"actual"}]}`},
		stubResponse{operation: "ListCollections", body: `{"collectionSummaries":[]}`},
	)
	latest, err := rm.sdkFind(context.Background(), testGroup())
	require.NoError(t, err)
	require.Len(t, stub.requests, 3)
	require.Len(t, latest.ko.Spec.Tags, 1)
	assert.Equal(t, "actual", *latest.ko.Spec.Tags[0].Value)
}

func TestSDKFindTagReadFailureRemainsRetryable(t *testing.T) {
	for _, tc := range []struct {
		code   string
		status int
	}{
		{code: "AccessDeniedException", status: http.StatusForbidden},
		{code: "ResourceNotFoundException", status: http.StatusNotFound},
		{code: "InternalServerException", status: http.StatusInternalServerError},
	} {
		t.Run(tc.code, func(t *testing.T) {
			rm, stub := newTestManager(t,
				stubResponse{
					operation: "BatchGetCollectionGroup",
					body:      `{"collectionGroupDetails":[{"id":"group123","arn":"` + groupARN + `","name":"aws-group"}]}`,
				},
				stubResponse{
					operation: "ListTagsForResource",
					status:    tc.status,
					body:      `{"__type":"` + tc.code + `","message":"tag lookup failed"}`,
				},
			)
			_, err := rm.sdkFind(context.Background(), testGroup())
			require.Error(t, err)
			var apiErr smithy.APIError
			require.ErrorAs(t, err, &apiErr)
			assert.Equal(t, tc.code, apiErr.ErrorCode())
			assert.False(t, errors.Is(err, ackerr.NotFound))
			assert.False(t, rm.terminalAWSError(err))
			require.Len(t, stub.requests, 2, "failed tag reads must not list members")
			assert.Equal(t, groupARN, stub.requests[1].input["resourceArn"])
		})
	}
}

func TestTagLookupWithoutARNIsNilSafe(t *testing.T) {
	for _, metadata := range []*ackv1alpha1.ResourceMetadata{nil, {}} {
		rm, stub := newTestManager(t)
		ko := testGroup().ko
		ko.Status.ACKResourceMetadata = metadata
		require.Error(t, rm.setTags(context.Background(), ko))
		assert.Empty(t, stub.requests)
	}
}

func TestTagLookupFailurePreservesExistingTags(t *testing.T) {
	rm, _ := newTestManager(t, stubResponse{
		operation: "ListTagsForResource",
		status:    http.StatusInternalServerError,
		body:      `{"__type":"InternalServerException","message":"tag lookup failed"}`,
	})
	ko := testGroup().ko
	ko.Spec.Tags = []*svcapitypes.Tag{{Key: aws.String("previous"), Value: aws.String("value")}}
	require.Error(t, rm.setTags(context.Background(), ko))
	require.Len(t, ko.Spec.Tags, 1)
	assert.Equal(t, "previous", *ko.Spec.Tags[0].Key)
}

func TestTagLookupEmptyResponseClearsObservedTags(t *testing.T) {
	rm, stub := newTestManager(t, stubResponse{operation: "ListTagsForResource", body: `{"tags":[]}`})
	ko := testGroup().ko
	ko.Spec.Tags = []*svcapitypes.Tag{{Key: aws.String("previous"), Value: aws.String("value")}}
	require.NoError(t, rm.setTags(context.Background(), ko))
	assert.Empty(t, ko.Spec.Tags)
	require.Len(t, stub.requests, 1)
	assert.Equal(t, groupARN, stub.requests[0].input["resourceArn"])
}

func TestSDKFindDistinguishesAbsenceFromBatchErrors(t *testing.T) {
	cases := []struct {
		name      string
		body      string
		notFound  bool
		errorCode string
	}{
		{
			name:     "documented missing-group error",
			body:     `{"collectionGroupErrorDetails":[{"id":"group123","errorCode":"NOT_FOUND","errorMessage":"missing"}]}`,
			notFound: true,
		},
		{
			name:     "empty successful response",
			body:     `{"collectionGroupDetails":[]}`,
			notFound: true,
		},
		{
			name:      "access error must not trigger another create",
			body:      `{"collectionGroupErrorDetails":[{"id":"group123","errorCode":"ACCESS_DENIED","errorMessage":"denied"}]}`,
			errorCode: "ACCESS_DENIED",
		},
		{
			name:      "missing error code remains an error",
			body:      `{"collectionGroupErrorDetails":[{"id":"group123","errorMessage":"unknown failure"}]}`,
			errorCode: "CollectionGroupReadError",
		},
		{
			name:      "only documented per-item NOT_FOUND means absence",
			body:      `{"collectionGroupErrorDetails":[{"id":"group123","errorCode":"ResourceNotFoundException","errorMessage":"unexpected item failure"}]}`,
			errorCode: "ResourceNotFoundException",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rm, stub := newTestManager(t, stubResponse{operation: "BatchGetCollectionGroup", body: tc.body})
			latest, err := rm.sdkFind(context.Background(), testGroup())
			require.Error(t, err)
			assert.Nil(t, latest)
			assert.Equal(t, tc.notFound, errors.Is(err, ackerr.NotFound))
			require.Len(t, stub.requests, 1, "failed reads must not list members")
			if tc.errorCode != "" {
				var apiErr smithy.APIError
				require.ErrorAs(t, err, &apiErr)
				assert.Equal(t, tc.errorCode, apiErr.ErrorCode())
				assert.False(t, rm.terminalAWSError(err))
			}
		})
	}
}

func TestSDKFindPreservesOperationError(t *testing.T) {
	rm, _ := newTestManager(t, stubResponse{
		operation: "BatchGetCollectionGroup",
		status:    http.StatusInternalServerError,
		body:      `{"__type":"InternalServerException","message":"try again"}`,
	})
	_, err := rm.sdkFind(context.Background(), testGroup())
	require.Error(t, err)
	var apiErr smithy.APIError
	require.ErrorAs(t, err, &apiErr)
	assert.Equal(t, "InternalServerException", apiErr.ErrorCode())
	assert.False(t, errors.Is(err, ackerr.NotFound))
}

func TestSDKFindWithoutIDDoesNotCallAWS(t *testing.T) {
	rm, stub := newTestManager(t)
	r := testGroup()
	r.ko.Status.ID = nil
	_, err := rm.sdkFind(context.Background(), r)
	assert.ErrorIs(t, err, ackerr.NotFound)
	assert.Empty(t, stub.requests)
}

func TestMemberLookupFailureDoesNotPublishPartialList(t *testing.T) {
	rm, stub := newTestManager(t,
		stubResponse{operation: "ListCollections", body: `{"collectionSummaries":[{"id":"partial"}],"nextToken":"page2"}`},
		stubResponse{operation: "ListCollections", status: http.StatusInternalServerError, body: `{"__type":"InternalServerException","message":"failed page"}`},
	)
	ko := testGroup().ko
	ko.Status.MemberCollections = []*svcapitypes.CollectionSummary{{ID: aws.String("previous")}}
	err := rm.setMemberCollections(context.Background(), ko)
	require.Error(t, err)
	assert.False(t, errors.Is(err, ackerr.NotFound))
	require.Len(t, stub.requests, 2)
	require.Len(t, ko.Status.MemberCollections, 1)
	assert.Equal(t, "previous", *ko.Status.MemberCollections[0].ID)
}

func TestMemberLookupWithoutNameDoesNotListEntireAccount(t *testing.T) {
	rm, stub := newTestManager(t)
	ko := testGroup().ko
	ko.Spec.Name = nil
	require.Error(t, rm.setMemberCollections(context.Background(), ko))
	assert.Empty(t, stub.requests)
}

func TestMemberLookupRepeatedTokenDoesNotPublishPartialList(t *testing.T) {
	rm, stub := newTestManager(t,
		stubResponse{operation: "ListCollections", body: `{"collectionSummaries":[{"id":"partial1"}],"nextToken":"repeated"}`},
		stubResponse{operation: "ListCollections", body: `{"collectionSummaries":[{"id":"partial2"}],"nextToken":"repeated"}`},
	)
	ko := testGroup().ko
	ko.Status.MemberCollections = []*svcapitypes.CollectionSummary{{ID: aws.String("previous")}}
	err := rm.setMemberCollections(context.Background(), ko)
	require.Error(t, err)
	require.Len(t, stub.requests, 2)
	require.Len(t, ko.Status.MemberCollections, 1)
	assert.Equal(t, "previous", *ko.Status.MemberCollections[0].ID)
}

func TestSDKUpdateTagsOnlyUsesTaggingAPIs(t *testing.T) {
	rm, stub := newTestManager(t,
		stubResponse{operation: "TagResource", body: `{}`},
		stubResponse{operation: "UntagResource", body: `{}`},
	)
	latest := testGroup()
	latest.ko.Spec.Tags = []*svcapitypes.Tag{{Key: aws.String("old"), Value: aws.String("value")}}
	desired := &resource{ko: latest.ko.DeepCopy()}
	desired.ko.Spec.Tags = []*svcapitypes.Tag{{Key: aws.String("new"), Value: aws.String("value")}}
	updated, err := rm.sdkUpdate(context.Background(), desired, latest, newResourceDelta(desired, latest))
	require.NoError(t, err)
	require.NotNil(t, updated)
	require.Len(t, stub.requests, 2)
	assert.Equal(t, groupARN, stub.requests[0].input["resourceArn"])
	assert.Equal(t, []interface{}{map[string]interface{}{"key": "new", "value": "value"}}, stub.requests[0].input["tags"])
	assert.Equal(t, groupARN, stub.requests[1].input["resourceArn"])
	assert.Equal(t, []interface{}{"old"}, stub.requests[1].input["tagKeys"])
	assert.Equal(t, latest.ko.Status.ID, updated.ko.Status.ID)
}

func TestSDKUpdateTagsWithoutARNIsRetryableAndNilSafe(t *testing.T) {
	rm, stub := newTestManager(t)
	latest := testGroup()
	latest.ko.Status.ACKResourceMetadata = nil
	desired := &resource{ko: latest.ko.DeepCopy()}
	desired.ko.Spec.Tags = []*svcapitypes.Tag{{Key: aws.String("new"), Value: aws.String("value")}}
	_, err := rm.sdkUpdate(context.Background(), desired, latest, newResourceDelta(desired, latest))
	require.Error(t, err)
	assert.False(t, rm.terminalAWSError(err))
	assert.Empty(t, stub.requests)
}

func TestSDKUpdateDescriptionDoesNotResubmitCapacity(t *testing.T) {
	rm, stub := newTestManager(t, stubResponse{
		operation: "UpdateCollectionGroup",
		body:      `{"updateCollectionGroupDetail":{"id":"group123","name":"aws-group","description":"updated"}}`,
	})
	latest := testGroup()
	desired := &resource{ko: latest.ko.DeepCopy()}
	desired.ko.Spec.Description = aws.String("updated")
	updated, err := rm.sdkUpdate(context.Background(), desired, latest, newResourceDelta(desired, latest))
	require.NoError(t, err)
	require.Len(t, stub.requests, 1)
	assert.Equal(t, "group123", stub.requests[0].input["id"])
	assert.Equal(t, "updated", stub.requests[0].input["description"])
	assert.NotContains(t, stub.requests[0].input, "capacityLimits")
	assert.NotContains(t, stub.requests[0].input, "tags")
	assert.Equal(t, desired.ko.Spec.Tags, updated.ko.Spec.Tags, "omitted update-output tags must preserve desired tags")
}

func TestSDKUpdateNestedCapacitySendsEveryDesiredLimit(t *testing.T) {
	rm, stub := newTestManager(t, stubResponse{
		operation: "UpdateCollectionGroup",
		body:      `{"updateCollectionGroupDetail":{"id":"group123","name":"aws-group"}}`,
	})
	latest := testGroup()
	desired := &resource{ko: latest.ko.DeepCopy()}
	desired.ko.Spec.CapacityLimits.MaxIndexingCapacityInOCU = aws.Float64(8)
	updated, err := rm.sdkUpdate(context.Background(), desired, latest, newResourceDelta(desired, latest))
	require.NoError(t, err)
	require.Len(t, stub.requests, 1)
	assert.NotContains(t, stub.requests[0].input, "description")
	assert.Equal(t, map[string]interface{}{
		"maxIndexingCapacityInOCU": float64(8),
		"maxSearchCapacityInOCU":   float64(6),
		"minIndexingCapacityInOCU": float64(1),
		"minSearchCapacityInOCU":   float64(2),
	}, stub.requests[0].input["capacityLimits"])
	assert.NotContains(t, stub.requests[0].input, "tags")
	assert.Equal(t, desired.ko.Spec.Tags, updated.ko.Spec.Tags, "capacity updates must preserve desired tags")
}

func TestSDKUpdatePreservesSimultaneousTagAndDescriptionChanges(t *testing.T) {
	rm, stub := newTestManager(t,
		stubResponse{operation: "TagResource", body: `{}`},
		stubResponse{operation: "UntagResource", body: `{}`},
		stubResponse{
			operation: "UpdateCollectionGroup",
			body:      `{"updateCollectionGroupDetail":{"id":"group123","name":"aws-group","description":"updated"}}`,
		},
	)
	latest := testGroup()
	desired := &resource{ko: latest.ko.DeepCopy()}
	desired.ko.Spec.Description = aws.String("updated")
	desired.ko.Spec.Tags = []*svcapitypes.Tag{{Key: aws.String("owner"), Value: aws.String("updated")}}
	updated, err := rm.sdkUpdate(context.Background(), desired, latest, newResourceDelta(desired, latest))
	require.NoError(t, err)
	require.Len(t, stub.requests, 3)
	assert.Equal(t, []interface{}{map[string]interface{}{"key": "owner", "value": "updated"}}, stub.requests[0].input["tags"])
	assert.Equal(t, []interface{}{"remove-me"}, stub.requests[1].input["tagKeys"])
	assert.Equal(t, "updated", stub.requests[2].input["description"])
	assert.NotContains(t, stub.requests[2].input, "tags")
	assert.Equal(t, desired.ko.Spec.Tags, updated.ko.Spec.Tags)
}

func TestLateInitializationPreservesExplicitLimits(t *testing.T) {
	rm, _ := newTestManager(t)
	observed := testGroup()
	desired := &resource{ko: observed.ko.DeepCopy()}
	desired.ko.Spec.CapacityLimits = &svcapitypes.CollectionGroupCapacityLimits{MaxIndexingCapacityInOCU: aws.Float64(12)}
	desired.ko.Spec.Description = nil
	desired.ko.Spec.Generation = nil
	initialized := rm.lateInitializeFromReadOneOutput(observed, desired).(*resource)
	assert.Equal(t, float64(12), *initialized.ko.Spec.CapacityLimits.MaxIndexingCapacityInOCU)
	assert.Equal(t, float64(6), *initialized.ko.Spec.CapacityLimits.MaxSearchCapacityInOCU)
	assert.Equal(t, float64(1), *initialized.ko.Spec.CapacityLimits.MinIndexingCapacityInOCU)
	assert.Equal(t, float64(2), *initialized.ko.Spec.CapacityLimits.MinSearchCapacityInOCU)
	assert.Equal(t, "CLASSIC", *initialized.ko.Spec.Generation)
	assert.Equal(t, "initial", *initialized.ko.Spec.Description)
	assert.False(t, rm.incompleteLateInitialization(initialized))
}

func TestAbsentOptionalDescriptionDoesNotBlockLateInitialization(t *testing.T) {
	rm, _ := newTestManager(t)
	desired := testGroup()
	desired.ko.Spec.Description = nil
	desired.ko.Spec.CapacityLimits = nil
	desired.ko.Spec.Generation = nil
	observed := &resource{ko: desired.ko.DeepCopy()}
	initialized := rm.lateInitializeFromReadOneOutput(observed, desired).(*resource)
	assert.Nil(t, initialized.ko.Spec.Description)
	assert.False(t, rm.incompleteLateInitialization(initialized))
}

func TestSDKDeleteNonemptyGroupRemainsRecoverable(t *testing.T) {
	rm, stub := newTestManager(t, stubResponse{
		operation: "DeleteCollectionGroup",
		status:    http.StatusConflict,
		body:      `{"__type":"ConflictException","message":"group contains collections"}`,
	})
	latest, err := rm.Delete(context.Background(), testGroup())
	require.Error(t, err)
	require.Len(t, stub.requests, 1)
	assert.Equal(t, "group123", stub.requests[0].input["id"])
	require.NotNil(t, latest)
	var recoverable bool
	for _, condition := range latest.Conditions() {
		assert.False(t, condition.Type == ackv1alpha1.ConditionTypeTerminal && condition.Status == corev1.ConditionTrue)
		if condition.Type == ackv1alpha1.ConditionTypeRecoverable && condition.Status == corev1.ConditionTrue {
			recoverable = true
		}
	}
	assert.True(t, recoverable)
}

func TestSDKCreateMapsIdentifiersAndPreservesTagsWhenOutputOmitsThem(t *testing.T) {
	rm, stub := newTestManager(t, stubResponse{
		operation: "CreateCollectionGroup",
		body: `{"createCollectionGroupDetail":{"id":"group123","name":"aws-group",
			"arn":"` + groupARN + `","standbyReplicas":"DISABLED","generation":"CLASSIC",
			"description":"initial","createdDate":123}}`,
	})
	desired := testGroup()
	desired.ko.Status = svcapitypes.CollectionGroupStatus{}
	created, err := rm.sdkCreate(context.Background(), desired)
	require.NoError(t, err)
	require.Len(t, stub.requests, 1)
	assert.Equal(t, "aws-group", stub.requests[0].input["name"])
	assert.Equal(t, "DISABLED", stub.requests[0].input["standbyReplicas"])
	assert.NotEmpty(t, stub.requests[0].input["clientToken"])
	assert.Equal(t, []interface{}{
		map[string]interface{}{"key": "owner", "value": "initial"},
		map[string]interface{}{"key": "remove-me", "value": "initial"},
	}, stub.requests[0].input["tags"], "output-only ignore must retain tag creation payload")
	assert.Equal(t, desired.ko.Spec.Tags, created.ko.Spec.Tags, "omitted create-output tags must not clear desired tags")
	assert.Equal(t, "group123", *created.ko.Status.ID)
	assert.Equal(t, ackv1alpha1.AWSResourceName(groupARN), *created.ko.Status.ACKResourceMetadata.ARN)
	assert.Equal(t, int64(123), *created.ko.Status.CreatedDate)
}
