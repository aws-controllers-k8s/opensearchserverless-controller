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

package lifecycle_policy

import (
	"context"

	ackerr "github.com/aws-controllers-k8s/runtime/pkg/errors"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/opensearchserverless"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/opensearchserverless/types"
)

// customFindLifecyclePolicy implements the custom read logic for LifecyclePolicy.
//
// There is no GetLifecyclePolicy API. The only read API is BatchGetLifecyclePolicy,
// which takes a list of LifecyclePolicyIdentifier structs and returns results in
// LifecyclePolicyDetails (successes) and LifecyclePolicyErrorDetails (failures).
// A not-found policy is indicated by an entry in LifecyclePolicyErrorDetails with
// ErrorCode "NOT_FOUND", not by an exception.
func (rm *resourceManager) customFindLifecyclePolicy(
	ctx context.Context,
	r *resource,
) (*resource, error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.customFindLifecyclePolicy")
	var err error
	defer func() {
		exit(err)
	}()

	if r.ko.Spec.Name == nil || r.ko.Spec.Type == nil {
		return nil, ackerr.NotFound
	}

	input := &svcsdk.BatchGetLifecyclePolicyInput{
		Identifiers: []svcsdktypes.LifecyclePolicyIdentifier{
			{
				Name: r.ko.Spec.Name,
				Type: svcsdktypes.LifecyclePolicyType(*r.ko.Spec.Type),
			},
		},
	}

	resp, err := rm.sdkapi.BatchGetLifecyclePolicy(ctx, input)
	rm.metrics.RecordAPICall("READ_ONE", "BatchGetLifecyclePolicy", err)
	if err != nil {
		return nil, err
	}

	// Check if the policy was returned in the error details (NOT_FOUND)
	if len(resp.LifecyclePolicyErrorDetails) > 0 {
		for _, errDetail := range resp.LifecyclePolicyErrorDetails {
			if errDetail.ErrorCode != nil && *errDetail.ErrorCode == "NOT_FOUND" {
				return nil, ackerr.NotFound
			}
		}
	}

	// Check if we got a result
	if len(resp.LifecyclePolicyDetails) == 0 {
		return nil, ackerr.NotFound
	}

	detail := resp.LifecyclePolicyDetails[0]

	// Merge in the information we read from the API call above to the copy of
	// the original Kubernetes object we passed to the function
	ko := r.ko.DeepCopy()

	if detail.CreatedDate != nil {
		ko.Status.CreatedDate = detail.CreatedDate
	} else {
		ko.Status.CreatedDate = nil
	}
	if detail.Description != nil {
		ko.Spec.Description = detail.Description
	} else {
		ko.Spec.Description = nil
	}
	if detail.LastModifiedDate != nil {
		ko.Status.LastModifiedDate = detail.LastModifiedDate
	} else {
		ko.Status.LastModifiedDate = nil
	}
	if detail.Name != nil {
		ko.Spec.Name = detail.Name
	} else {
		ko.Spec.Name = nil
	}
	if detail.PolicyVersion != nil {
		ko.Status.PolicyVersion = detail.PolicyVersion
	} else {
		ko.Status.PolicyVersion = nil
	}
	if detail.Type != "" {
		ko.Spec.Type = aws.String(string(detail.Type))
	} else {
		ko.Spec.Type = nil
	}

	// Handle the Policy field which is a document.Interface type
	if detail.Policy != nil {
		policy, err := detail.Policy.MarshalSmithyDocument()
		if err != nil {
			return &resource{ko}, err
		}
		ko.Spec.Policy = aws.String(string(policy))
	}

	rm.setStatusDefaults(ko)
	return &resource{ko}, nil
}
