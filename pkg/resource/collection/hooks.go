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
	svcsdktypes "github.com/aws/aws-sdk-go-v2/service/opensearchserverless/types"

	"github.com/aws-controllers-k8s/opensearchserverless-controller/pkg/sync"
)

var syncTags = sync.Tags
var getTags = sync.GetTags

// collectionIsActive returns true if the collection is active, or false if it is not active
func collectionIsActive(desired *resource) bool {
	if desired.ko.Status.Status != nil && *desired.ko.Status.Status == string(svcsdktypes.CollectionStatusActive) {
		return true
	}

	return false
}
