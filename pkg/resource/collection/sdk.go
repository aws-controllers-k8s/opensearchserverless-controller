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

package collection

import (
	"context"
	"errors"
	"fmt"
	"reflect"
	"strings"

	ackv1alpha1 "github.com/aws-controllers-k8s/runtime/apis/core/v1alpha1"
	ackcompare "github.com/aws-controllers-k8s/runtime/pkg/compare"
	ackcondition "github.com/aws-controllers-k8s/runtime/pkg/condition"
	ackerr "github.com/aws-controllers-k8s/runtime/pkg/errors"
	ackrequeue "github.com/aws-controllers-k8s/runtime/pkg/requeue"
	ackrtlog "github.com/aws-controllers-k8s/runtime/pkg/runtime/log"
	"github.com/aws/aws-sdk-go-v2/aws"
	svcsdk "github.com/aws/aws-sdk-go-v2/service/opensearchserverless"
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/opensearchserverless/types"
	smithy "github.com/aws/smithy-go"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	svcapitypes "github.com/aws-controllers-k8s/opensearchserverless-controller/apis/v1alpha1"
)

// Hack to avoid import errors during build...
var (
	_ = &metav1.Time{}
	_ = strings.ToLower("")
	_ = &svcsdk.Client{}
	_ = &svcapitypes.Collection{}
	_ = ackv1alpha1.AWSAccountID("")
	_ = &ackerr.NotFound
	_ = &ackcondition.NotManagedMessage
	_ = &reflect.Value{}
	_ = fmt.Sprintf("")
	_ = &ackrequeue.NoRequeue{}
	_ = &aws.Config{}
)

// sdkFind returns SDK-specific information about a supplied resource
func (rm *resourceManager) sdkFind(
	ctx context.Context,
	r *resource,
) (latest *resource, err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.sdkFind")
	defer func() {
		exit(err)
	}()
	// If any required fields in the input shape are missing, AWS resource is
	// not created yet. Return NotFound here to indicate to callers that the
	// resource isn't yet created.
	if rm.requiredFieldsMissingFromReadManyInput(r) {
		return nil, ackerr.NotFound
	}

	input, err := rm.newListRequestPayload(r)
	if err != nil {
		return nil, err
	}
	var resp *svcsdk.BatchGetCollectionOutput
	resp, err = rm.sdkapi.BatchGetCollection(ctx, input)
	rm.metrics.RecordAPICall("READ_MANY", "BatchGetCollection", err)
	if err != nil {
		var awsErr smithy.APIError
		if errors.As(err, &awsErr) && awsErr.ErrorCode() == "UNKNOWN" {
			return nil, ackerr.NotFound
		}
		return nil, err
	}

	// Merge in the information we read from the API call above to the copy of
	// the original Kubernetes object we passed to the function
	ko := r.ko.DeepCopy()

	found := false
	for _, elem := range resp.CollectionDetails {
		if elem.Arn != nil {
			if ko.Status.ACKResourceMetadata == nil {
				ko.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{}
			}
			tmpARN := ackv1alpha1.AWSResourceName(*elem.Arn)
			ko.Status.ACKResourceMetadata.ARN = &tmpARN
		}
		if elem.CreatedDate != nil {
			ko.Status.CreatedDate = elem.CreatedDate
		} else {
			ko.Status.CreatedDate = nil
		}
		if elem.Description != nil {
			ko.Spec.Description = elem.Description
		} else {
			ko.Spec.Description = nil
		}
		if elem.Id != nil {
			ko.Status.ID = elem.Id
		} else {
			ko.Status.ID = nil
		}
		if elem.KmsKeyArn != nil {
			ko.Status.KMSKeyARN = elem.KmsKeyArn
		} else {
			ko.Status.KMSKeyARN = nil
		}
		if elem.LastModifiedDate != nil {
			ko.Status.LastModifiedDate = elem.LastModifiedDate
		} else {
			ko.Status.LastModifiedDate = nil
		}
		if elem.Name != nil {
			ko.Spec.Name = elem.Name
		} else {
			ko.Spec.Name = nil
		}
		if elem.StandbyReplicas != "" {
			ko.Spec.StandbyReplicas = aws.String(string(elem.StandbyReplicas))
		} else {
			ko.Spec.StandbyReplicas = nil
		}
		if elem.Status != "" {
			ko.Status.Status = aws.String(string(elem.Status))
		} else {
			ko.Status.Status = nil
		}
		if elem.Type != "" {
			ko.Spec.Type = aws.String(string(elem.Type))
		} else {
			ko.Spec.Type = nil
		}
		found = true
		break
	}
	if !found {
		return nil, ackerr.NotFound
	}

	rm.setStatusDefaults(ko)
	ko.Spec.Tags, err = getTags(ctx, string(*ko.Status.ACKResourceMetadata.ARN), rm.sdkapi, rm.metrics)
	if err != nil {
		return &resource{ko}, err
	}

	if !collectionIsActive(&resource{ko}) {
		ackcondition.SetSynced(&resource{ko}, corev1.ConditionFalse, aws.String("collection is not active"), nil)
	}

	return &resource{ko}, nil
}

// requiredFieldsMissingFromReadManyInput returns true if there are any fields
// for the ReadMany Input shape that are required but not present in the
// resource's Spec or Status
func (rm *resourceManager) requiredFieldsMissingFromReadManyInput(
	r *resource,
) bool {
	return r.ko.Status.ID == nil

}

// newListRequestPayload returns SDK-specific struct for the HTTP request
// payload of the List API call for the resource
func (rm *resourceManager) newListRequestPayload(
	r *resource,
) (*svcsdk.BatchGetCollectionInput, error) {
	res := &svcsdk.BatchGetCollectionInput{}

	if r.ko.Status.ID != nil {
		f0 := []string{}
		f0 = append(f0, *r.ko.Status.ID)
		res.Ids = f0
	}

	return res, nil
}

// sdkCreate creates the supplied resource in the backend AWS service API and
// returns a copy of the resource with resource fields (in both Spec and
// Status) filled in with values from the CREATE API operation's Output shape.
func (rm *resourceManager) sdkCreate(
	ctx context.Context,
	desired *resource,
) (created *resource, err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.sdkCreate")
	defer func() {
		exit(err)
	}()
	input, err := rm.newCreateRequestPayload(ctx, desired)
	if err != nil {
		return nil, err
	}

	var resp *svcsdk.CreateCollectionOutput
	_ = resp
	resp, err = rm.sdkapi.CreateCollection(ctx, input)
	rm.metrics.RecordAPICall("CREATE", "CreateCollection", err)
	if err != nil {
		return nil, err
	}
	// Merge in the information we read from the API call above to the copy of
	// the original Kubernetes object we passed to the function
	ko := desired.ko.DeepCopy()

	if ko.Status.ACKResourceMetadata == nil {
		ko.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{}
	}
	if resp.CreateCollectionDetail.Arn != nil {
		arn := ackv1alpha1.AWSResourceName(*resp.CreateCollectionDetail.Arn)
		ko.Status.ACKResourceMetadata.ARN = &arn
	}
	if resp.CreateCollectionDetail.CreatedDate != nil {
		ko.Status.CreatedDate = resp.CreateCollectionDetail.CreatedDate
	} else {
		ko.Status.CreatedDate = nil
	}
	if resp.CreateCollectionDetail.Description != nil {
		ko.Spec.Description = resp.CreateCollectionDetail.Description
	} else {
		ko.Spec.Description = nil
	}
	if resp.CreateCollectionDetail.Id != nil {
		ko.Status.ID = resp.CreateCollectionDetail.Id
	} else {
		ko.Status.ID = nil
	}
	if resp.CreateCollectionDetail.KmsKeyArn != nil {
		ko.Status.KMSKeyARN = resp.CreateCollectionDetail.KmsKeyArn
	} else {
		ko.Status.KMSKeyARN = nil
	}
	if resp.CreateCollectionDetail.LastModifiedDate != nil {
		ko.Status.LastModifiedDate = resp.CreateCollectionDetail.LastModifiedDate
	} else {
		ko.Status.LastModifiedDate = nil
	}
	if resp.CreateCollectionDetail.Name != nil {
		ko.Spec.Name = resp.CreateCollectionDetail.Name
	} else {
		ko.Spec.Name = nil
	}
	if resp.CreateCollectionDetail.StandbyReplicas != "" {
		ko.Spec.StandbyReplicas = aws.String(string(resp.CreateCollectionDetail.StandbyReplicas))
	} else {
		ko.Spec.StandbyReplicas = nil
	}
	if resp.CreateCollectionDetail.Status != "" {
		ko.Status.Status = aws.String(string(resp.CreateCollectionDetail.Status))
	} else {
		ko.Status.Status = nil
	}
	if resp.CreateCollectionDetail.Type != "" {
		ko.Spec.Type = aws.String(string(resp.CreateCollectionDetail.Type))
	} else {
		ko.Spec.Type = nil
	}

	rm.setStatusDefaults(ko)
	return &resource{ko}, nil
}

// newCreateRequestPayload returns an SDK-specific struct for the HTTP request
// payload of the Create API call for the resource
func (rm *resourceManager) newCreateRequestPayload(
	ctx context.Context,
	r *resource,
) (*svcsdk.CreateCollectionInput, error) {
	res := &svcsdk.CreateCollectionInput{}

	if r.ko.Spec.Description != nil {
		res.Description = r.ko.Spec.Description
	}
	if r.ko.Spec.Name != nil {
		res.Name = r.ko.Spec.Name
	}
	if r.ko.Spec.StandbyReplicas != nil {
		res.StandbyReplicas = svcsdktypes.StandbyReplicas(*r.ko.Spec.StandbyReplicas)
	}
	if r.ko.Spec.Tags != nil {
		f3 := []svcsdktypes.Tag{}
		for _, f3iter := range r.ko.Spec.Tags {
			f3elem := &svcsdktypes.Tag{}
			if f3iter.Key != nil {
				f3elem.Key = f3iter.Key
			}
			if f3iter.Value != nil {
				f3elem.Value = f3iter.Value
			}
			f3 = append(f3, *f3elem)
		}
		res.Tags = f3
	}
	if r.ko.Spec.Type != nil {
		res.Type = svcsdktypes.CollectionType(*r.ko.Spec.Type)
	}

	return res, nil
}

// sdkUpdate patches the supplied resource in the backend AWS service API and
// returns a new resource with updated fields.
func (rm *resourceManager) sdkUpdate(
	ctx context.Context,
	desired *resource,
	latest *resource,
	delta *ackcompare.Delta,
) (updated *resource, err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.sdkUpdate")
	defer func() {
		exit(err)
	}()
	desired.SetStatus(latest)
	if !collectionIsActive(desired) {
		return desired, ackrequeue.Needed(fmt.Errorf("resource is %s", *desired.ko.Status.Status))
	}
	if delta.DifferentAt("Spec.Tags") {
		arn := string(*latest.ko.Status.ACKResourceMetadata.ARN)
		err = syncTags(
			ctx,
			desired.ko.Spec.Tags, latest.ko.Spec.Tags,
			&arn, convertToOrderedACKTags, rm.sdkapi, rm.metrics,
		)
		if err != nil {
			return desired, err
		}
	}
	if !delta.DifferentExcept("Spec.Tags") {
		return desired, nil
	}

	input, err := rm.newUpdateRequestPayload(ctx, desired, delta)
	if err != nil {
		return nil, err
	}

	var resp *svcsdk.UpdateCollectionOutput
	_ = resp
	resp, err = rm.sdkapi.UpdateCollection(ctx, input)
	rm.metrics.RecordAPICall("UPDATE", "UpdateCollection", err)
	if err != nil {
		return nil, err
	}
	// Merge in the information we read from the API call above to the copy of
	// the original Kubernetes object we passed to the function
	ko := desired.ko.DeepCopy()

	if ko.Status.ACKResourceMetadata == nil {
		ko.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{}
	}
	if resp.UpdateCollectionDetail.Arn != nil {
		arn := ackv1alpha1.AWSResourceName(*resp.UpdateCollectionDetail.Arn)
		ko.Status.ACKResourceMetadata.ARN = &arn
	}
	if resp.UpdateCollectionDetail.CreatedDate != nil {
		ko.Status.CreatedDate = resp.UpdateCollectionDetail.CreatedDate
	} else {
		ko.Status.CreatedDate = nil
	}
	if resp.UpdateCollectionDetail.Description != nil {
		ko.Spec.Description = resp.UpdateCollectionDetail.Description
	} else {
		ko.Spec.Description = nil
	}
	if resp.UpdateCollectionDetail.Id != nil {
		ko.Status.ID = resp.UpdateCollectionDetail.Id
	} else {
		ko.Status.ID = nil
	}
	if resp.UpdateCollectionDetail.LastModifiedDate != nil {
		ko.Status.LastModifiedDate = resp.UpdateCollectionDetail.LastModifiedDate
	} else {
		ko.Status.LastModifiedDate = nil
	}
	if resp.UpdateCollectionDetail.Name != nil {
		ko.Spec.Name = resp.UpdateCollectionDetail.Name
	} else {
		ko.Spec.Name = nil
	}
	if resp.UpdateCollectionDetail.Status != "" {
		ko.Status.Status = aws.String(string(resp.UpdateCollectionDetail.Status))
	} else {
		ko.Status.Status = nil
	}
	if resp.UpdateCollectionDetail.Type != "" {
		ko.Spec.Type = aws.String(string(resp.UpdateCollectionDetail.Type))
	} else {
		ko.Spec.Type = nil
	}

	rm.setStatusDefaults(ko)
	return &resource{ko}, nil
}

// newUpdateRequestPayload returns an SDK-specific struct for the HTTP request
// payload of the Update API call for the resource
func (rm *resourceManager) newUpdateRequestPayload(
	ctx context.Context,
	r *resource,
	delta *ackcompare.Delta,
) (*svcsdk.UpdateCollectionInput, error) {
	res := &svcsdk.UpdateCollectionInput{}

	if r.ko.Spec.Description != nil {
		res.Description = r.ko.Spec.Description
	}
	if r.ko.Status.ID != nil {
		res.Id = r.ko.Status.ID
	}

	return res, nil
}

// sdkDelete deletes the supplied resource in the backend AWS service API
func (rm *resourceManager) sdkDelete(
	ctx context.Context,
	r *resource,
) (latest *resource, err error) {
	rlog := ackrtlog.FromContext(ctx)
	exit := rlog.Trace("rm.sdkDelete")
	defer func() {
		exit(err)
	}()
	input, err := rm.newDeleteRequestPayload(r)
	if err != nil {
		return nil, err
	}
	var resp *svcsdk.DeleteCollectionOutput
	_ = resp
	resp, err = rm.sdkapi.DeleteCollection(ctx, input)
	rm.metrics.RecordAPICall("DELETE", "DeleteCollection", err)
	return nil, err
}

// newDeleteRequestPayload returns an SDK-specific struct for the HTTP request
// payload of the Delete API call for the resource
func (rm *resourceManager) newDeleteRequestPayload(
	r *resource,
) (*svcsdk.DeleteCollectionInput, error) {
	res := &svcsdk.DeleteCollectionInput{}

	if r.ko.Status.ID != nil {
		res.Id = r.ko.Status.ID
	}

	return res, nil
}

// setStatusDefaults sets default properties into supplied custom resource
func (rm *resourceManager) setStatusDefaults(
	ko *svcapitypes.Collection,
) {
	if ko.Status.ACKResourceMetadata == nil {
		ko.Status.ACKResourceMetadata = &ackv1alpha1.ResourceMetadata{}
	}
	if ko.Status.ACKResourceMetadata.Region == nil {
		ko.Status.ACKResourceMetadata.Region = &rm.awsRegion
	}
	if ko.Status.ACKResourceMetadata.OwnerAccountID == nil {
		ko.Status.ACKResourceMetadata.OwnerAccountID = &rm.awsAccountID
	}
	if ko.Status.Conditions == nil {
		ko.Status.Conditions = []*ackv1alpha1.Condition{}
	}
}

// updateConditions returns updated resource, true; if conditions were updated
// else it returns nil, false
func (rm *resourceManager) updateConditions(
	r *resource,
	onSuccess bool,
	err error,
) (*resource, bool) {
	ko := r.ko.DeepCopy()
	rm.setStatusDefaults(ko)

	// Terminal condition
	var terminalCondition *ackv1alpha1.Condition = nil
	var recoverableCondition *ackv1alpha1.Condition = nil
	var syncCondition *ackv1alpha1.Condition = nil
	for _, condition := range ko.Status.Conditions {
		if condition.Type == ackv1alpha1.ConditionTypeTerminal {
			terminalCondition = condition
		}
		if condition.Type == ackv1alpha1.ConditionTypeRecoverable {
			recoverableCondition = condition
		}
		if condition.Type == ackv1alpha1.ConditionTypeResourceSynced {
			syncCondition = condition
		}
	}
	var termError *ackerr.TerminalError
	if rm.terminalAWSError(err) || err == ackerr.SecretTypeNotSupported || err == ackerr.SecretNotFound || errors.As(err, &termError) {
		if terminalCondition == nil {
			terminalCondition = &ackv1alpha1.Condition{
				Type: ackv1alpha1.ConditionTypeTerminal,
			}
			ko.Status.Conditions = append(ko.Status.Conditions, terminalCondition)
		}
		var errorMessage = ""
		if err == ackerr.SecretTypeNotSupported || err == ackerr.SecretNotFound || errors.As(err, &termError) {
			errorMessage = err.Error()
		} else {
			awsErr, _ := ackerr.AWSError(err)
			errorMessage = awsErr.Error()
		}
		terminalCondition.Status = corev1.ConditionTrue
		terminalCondition.Message = &errorMessage
	} else {
		// Clear the terminal condition if no longer present
		if terminalCondition != nil {
			terminalCondition.Status = corev1.ConditionFalse
			terminalCondition.Message = nil
		}
		// Handling Recoverable Conditions
		if err != nil {
			if recoverableCondition == nil {
				// Add a new Condition containing a non-terminal error
				recoverableCondition = &ackv1alpha1.Condition{
					Type: ackv1alpha1.ConditionTypeRecoverable,
				}
				ko.Status.Conditions = append(ko.Status.Conditions, recoverableCondition)
			}
			recoverableCondition.Status = corev1.ConditionTrue
			awsErr, _ := ackerr.AWSError(err)
			errorMessage := err.Error()
			if awsErr != nil {
				errorMessage = awsErr.Error()
			}
			recoverableCondition.Message = &errorMessage
		} else if recoverableCondition != nil {
			recoverableCondition.Status = corev1.ConditionFalse
			recoverableCondition.Message = nil
		}
	}
	// Required to avoid the "declared but not used" error in the default case
	_ = syncCondition
	if terminalCondition != nil || recoverableCondition != nil || syncCondition != nil {
		return &resource{ko}, true // updated
	}
	return nil, false // not updated
}

// terminalAWSError returns awserr, true; if the supplied error is an aws Error type
// and if the exception indicates that it is a Terminal exception
// 'Terminal' exception are specified in generator configuration
func (rm *resourceManager) terminalAWSError(err error) bool {
	// No terminal_errors specified for this resource in generator config
	return false
}
