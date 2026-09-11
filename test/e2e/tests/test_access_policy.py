# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You may
# not use this file except in compliance with the License. A copy of the
# License is located at
#
#         http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is distributed
# on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
# express or implied. See the License for the specific language governing
# permissions and limitations under the License.

"""Integration tests for the OpensearchServerless AccessPolicy resource"""

import json
import time

import pytest

from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from acktest.aws.identity import get_account_id
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e import access_policy

ACCESS_POLICY_RESOURCE_PLURAL = "accesspolicies"
# Wait time
DELETE_WAIT_AFTER_SECONDS = 10
CHECK_STATUS_WAIT_SECONDS = 30
MODIFY_WAIT_AFTER_SECONDS = 30
# Descriptions
INITIAL_DESCRIPTION = "Initial Description"
UPDATED_DESCRIPTION = "Updated Description"


def _data_access_policy(principal, collection_resource):
    """Build a data access policy granting access to a specific collection scope."""
    return json.dumps([{
        "Rules": [
            {
                "ResourceType": "collection",
                "Resource": [collection_resource],
                "Permission": ["aoss:CreateCollectionItems", "aoss:UpdateCollectionItems",
                               "aoss:DescribeCollectionItems"],
            },
            {
                "ResourceType": "index",
                "Resource": [collection_resource.replace("collection/", "index/") + "/*"],
                "Permission": ["aoss:CreateIndex", "aoss:UpdateIndex", "aoss:DescribeIndex",
                               "aoss:ReadDocument", "aoss:WriteDocument"],
            },
        ],
        "Principal": [principal],
    }])


@pytest.fixture
def simple_access_policy(request):
    ap_name = random_suffix_name("my-access-policy", 24)
    # Unique collection scope for this fixture instance so policies created by
    # concurrent iterations never overlap.
    scope = random_suffix_name("col", 12)
    collection_resource = f"collection/{scope}"

    # Use the current account's root principal so OpenSearch Serverless does
    # not reject the policy with "Cross account principal(s) are not allowed".
    account_id = get_account_id()
    principal = f"arn:aws:iam::{account_id}:root"
    marker = request.node.get_closest_marker("resource_data")
    if marker is not None:
        data = marker.args[0]
        if 'principal' in data:
            principal = data['principal']

    ap_policy = _data_access_policy(principal, collection_resource)

    replacements = REPLACEMENT_VALUES.copy()
    replacements['ACCESS_POLICY_NAME'] = ap_name
    replacements['ACCESS_POLICY_DESCRIPTION'] = INITIAL_DESCRIPTION
    replacements['ACCESS_POLICY_TYPE'] = "data"
    replacements['ACCESS_POLICY'] = ap_policy

    resource_data = load_resource(
        "access_policy",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, ACCESS_POLICY_RESOURCE_PLURAL,
        ap_name, namespace="default",
    )
    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    assert k8s.get_resource_exists(ref)

    yield (ref, cr, scope, principal)

    _, deleted = k8s.delete_custom_resource(
        ref,
        period_length=DELETE_WAIT_AFTER_SECONDS,
    )
    assert deleted
    # Confirm the backing AWS resource is gone, not just the Kubernetes object.
    time.sleep(DELETE_WAIT_AFTER_SECONDS)
    assert access_policy.get(ap_name, "data") is None


@service_marker
@pytest.mark.canary
class TestAccessPolicy:
    def test_crud(self, simple_access_policy):
        ref, _, scope, principal = simple_access_policy

        time.sleep(CHECK_STATUS_WAIT_SECONDS)
        condition.assert_synced(ref)

        # Check that access policy exists
        cr = k8s.get_resource(ref)
        assert cr is not None
        assert 'status' in cr
        assert 'policyVersion' in cr['status']

        assert 'spec' in cr
        assert 'type' in cr['spec']
        assert 'name' in cr['spec']

        name = cr['spec']['name']
        ap_type = cr['spec']['type']

        latest = access_policy.get(name, ap_type)
        assert latest is not None
        pre_update_version = cr['status']['policyVersion']

        # Update the access policy description and policy with a broader scope
        updated_policy = _data_access_policy(principal, f"collection/{scope}*")
        updates = {
            "spec": {
                "description": UPDATED_DESCRIPTION,
                "policy": updated_policy,
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        cr = k8s.get_resource(ref)
        assert cr is not None
        assert cr['spec'].get('description') == UPDATED_DESCRIPTION
        assert cr['status']['policyVersion'] != pre_update_version

        latest = access_policy.get(name, ap_type)
        assert latest is not None
        assert latest['policyVersion'] != pre_update_version
        # The service returns the policy document as parsed JSON (whitespace
        # stripped, keys reordered), so compare structurally against what we
        # submitted rather than as raw strings.
        assert latest['policy'] == json.loads(updated_policy)

        # Verify deletion happens in fixture teardown
