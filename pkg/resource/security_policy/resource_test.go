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

package security_policy

import (
	"errors"
	"testing"

	ackerrors "github.com/aws-controllers-k8s/runtime/pkg/errors"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	svcapitypes "github.com/aws-controllers-k8s/opensearchserverless-controller/apis/v1alpha1"
)

func emptySecurityPolicy() *resource {
	return &resource{ko: &svcapitypes.SecurityPolicy{}}
}

func isTerminal(err error) bool {
	var termErr *ackerrors.TerminalError
	return errors.As(err, &termErr)
}

// SecurityPolicy is identified by the composite key (Name, Type). The Type
// adoption key is "type" (matching the CRD field). Earlier controller releases
// emitted the reserved-word-escaped "type_"; a pre_populate_resource_from_annotation
// hook aliases the legacy key so existing adoption annotations keep working.
func TestPopulateResourceFromAnnotation(t *testing.T) {
	testCases := []struct {
		name           string
		fields         map[string]string
		expectTerminal bool
		expectName     *string
		expectType     *string
	}{
		{
			name:       "current keys (name + type)",
			fields:     map[string]string{"name": "my-policy", "type": "encryption"},
			expectName: strptr("my-policy"),
			expectType: strptr("encryption"),
		},
		{
			name:       "legacy type_ key is aliased to type",
			fields:     map[string]string{"name": "my-policy", "type_": "encryption"},
			expectName: strptr("my-policy"),
			expectType: strptr("encryption"),
		},
		{
			name:           "both type and legacy type_ present is rejected",
			fields:         map[string]string{"name": "my-policy", "type": "network", "type_": "encryption"},
			expectTerminal: true,
		},
		{
			name:           "missing type (neither type nor type_)",
			fields:         map[string]string{"name": "my-policy"},
			expectTerminal: true,
		},
		{
			name:           "missing name",
			fields:         map[string]string{"type": "encryption"},
			expectTerminal: true,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			assert := assert.New(t)
			require := require.New(t)

			r := emptySecurityPolicy()
			err := r.PopulateResourceFromAnnotation(tc.fields)

			if tc.expectTerminal {
				require.Error(err)
				assert.True(isTerminal(err), "expected a terminal error, got: %v", err)
				return
			}

			require.NoError(err)
			assert.Equal(tc.expectName, r.ko.Spec.Name)
			assert.Equal(tc.expectType, r.ko.Spec.Type)
		})
	}
}

func strptr(s string) *string { return &s }
