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

package sync

import (
	"context"

	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	"github.com/aws-controllers-k8s/runtime/pkg/metrics"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	acktags "github.com/aws-controllers-k8s/runtime/pkg/tags"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/opensearchserverless"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/opensearchserverless/types"

	svcapitypes "github.com/aws-controllers-k8s/opensearchserverless-controller/apis/v1alpha1"
)

// Tags examines the Tags in the supplied Resource and calls the
// TagResource and UntagResource APIs to ensure that the set of
// associated Tags stays in sync with the Resource.Spec.Tags
func Tags(
	ctx context.Context,
	desiredTags []*svcapitypes.Tag,
	latestTags []*svcapitypes.Tag,
	arn *string,
	toACKTags func([]*svcapitypes.Tag) (acktags.Tags, []string),
	sdkapi *svcsdk.Client,
	metrics *metrics.Metrics,
) (err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("syncTags")
	defer func() { exit(err) }()

	from, _ := toACKTags(latestTags)
	to, _ := toACKTags(desiredTags)

	added, _, removed := ackcompare.GetTagsDifference(from, to)

	for key := range removed {
		if _, ok := added[key]; ok {
			delete(removed, key)
		}
	}

	if len(added) > 0 {
		toAdd := make([]svcsdktypes.Tag, 0, len(added))
		for key, val := range added {
			key, val := key, val
			toAdd = append(toAdd, svcsdktypes.Tag{Key: &key, Value: &val})
		}
		rlog.Debug("adding tags to work group", "tags", added)
		_, err = sdkapi.TagResource(
			ctx,
			&svcsdk.TagResourceInput{
				ResourceArn: arn,
				Tags:        toAdd,
			},
		)
		metrics.RecordAPICall("UPDATE", "AddTagsToResource", err)
		if err != nil {
			return err
		}
	}

	if len(removed) > 0 {
		toRemove := make([]string, 0, len(removed))
		for key := range removed {
			key := key
			toRemove = append(toRemove, key)
		}
		rlog.Debug("removing tags from work group", "tags", removed)
		_, err = sdkapi.UntagResource(
			ctx,
			&svcsdk.UntagResourceInput{
				ResourceArn: arn,
				TagKeys:     toRemove,
			},
		)
		metrics.RecordAPICall("UPDATE", "RemoveTagsFromResource", err)
		if err != nil {
			return err
		}
	}

	return nil
}

func GetTags(ctx context.Context, arn string, sdkapi *svcsdk.Client, metrics *metrics.Metrics) ([]*svcapitypes.Tag, error) {

	resp, err := sdkapi.ListTagsForResource(ctx, &svcsdk.ListTagsForResourceInput{
		ResourceArn: &arn,
	})
	if err != nil {
		return nil, err
	}
	return convertTags(resp.Tags), nil
}

func convertTags(tags []svcsdktypes.Tag) []*svcapitypes.Tag {
	resp := make([]*svcapitypes.Tag, 0, len(tags))
	for _, t := range tags {
		resp = append(resp, &svcapitypes.Tag{Key: t.Key, Value: t.Value})
	}

	return resp
}
