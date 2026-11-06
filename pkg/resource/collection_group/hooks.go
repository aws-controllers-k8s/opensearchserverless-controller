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
	"fmt"

	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/opensearchserverless"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/opensearchserverless/types"
	smithy "github.com/aws/smithy-go"

	svcapitypes "github.com/aws-controllers-k8s/opensearchserverless-controller/apis/v1alpha1"
	"github.com/aws-controllers-k8s/opensearchserverless-controller/pkg/sync"
)

var syncTags = sync.Tags
var getTags = sync.GetTags

// BatchGetCollectionGroup can report failures inside a successful response.
// Only NOT_FOUND may fall through to the generated empty-detail NotFound check.
// Other errors must remain retryable instead of causing a second create.
func collectionGroupReadError(resp *svcsdk.BatchGetCollectionGroupOutput) error {
	if resp == nil {
		return fmt.Errorf("BatchGetCollectionGroup returned no response")
	}
	for _, detail := range resp.CollectionGroupErrorDetails {
		code := aws.ToString(detail.ErrorCode)
		if code == "NOT_FOUND" {
			continue
		}
		if code == "" {
			code = "CollectionGroupReadError"
		}
		return &smithy.GenericAPIError{
			Code:    code,
			Message: aws.ToString(detail.ErrorMessage),
			Fault:   smithy.FaultUnknown,
		}
	}
	return nil
}

// setTags reads the authoritative tag API. The live batch response can omit
// Tags even though the SDK model includes that field.
func (rm *resourceManager) setTags(ctx context.Context, ko *svcapitypes.CollectionGroup) error {
	if ko.Status.ACKResourceMetadata == nil || ko.Status.ACKResourceMetadata.ARN == nil {
		return fmt.Errorf("collection group ARN is unavailable for tag lookup")
	}
	tags, err := getTags(ctx, string(*ko.Status.ACKResourceMetadata.ARN), rm.sdkapi, rm.metrics)
	rm.metrics.RecordAPICall("READ_MANY", "ListTagsForResource", err)
	if err != nil {
		return err
	}
	ko.Spec.Tags = tags
	return nil
}

// setMemberCollections reads actual AWS membership. The batch group API returns
// only a member count; ListCollections provides filtered, paginated summaries.
// Publish the list only after every page succeeds.
func (rm *resourceManager) setMemberCollections(ctx context.Context, ko *svcapitypes.CollectionGroup) error {
	if ko.Spec.Name == nil {
		return fmt.Errorf("collection group name is unavailable for membership lookup")
	}
	paginator := svcsdk.NewListCollectionsPaginator(
		rm.sdkapi,
		&svcsdk.ListCollectionsInput{
			CollectionFilters: &svcsdktypes.CollectionFilters{
				CollectionGroupName: ko.Spec.Name,
			},
		},
	)
	members := make([]*svcapitypes.CollectionSummary, 0)
	seenTokens := make(map[string]struct{})
	for paginator.HasMorePages() {
		resp, err := paginator.NextPage(ctx)
		rm.metrics.RecordAPICall("READ_MANY", "ListCollections", err)
		if err != nil {
			return err
		}
		if token := aws.ToString(resp.NextToken); token != "" {
			if _, seen := seenTokens[token]; seen {
				return fmt.Errorf("ListCollections returned a repeated pagination token")
			}
			seenTokens[token] = struct{}{}
		}
		for _, member := range resp.CollectionSummaries {
			summary := &svcapitypes.CollectionSummary{
				ARN:                 member.Arn,
				CollectionGroupName: member.CollectionGroupName,
				ID:                  member.Id,
				KMSKeyARN:           member.KmsKeyArn,
				Name:                member.Name,
			}
			if member.Status != "" {
				summary.Status = aws.String(string(member.Status))
			}
			members = append(members, summary)
		}
	}
	ko.Status.MemberCollections = members
	return nil
}
