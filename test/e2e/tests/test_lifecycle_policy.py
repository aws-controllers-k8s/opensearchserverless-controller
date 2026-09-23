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

"""Integration tests for the OpensearchServerless LifecyclePolicy resource"""

import json
import time

import pytest

from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e import lifecycle_policy

LIFECYCLE_POLICY_RESOURCE_PLURAL = "lifecyclepolicies"
# Wait time
DELETE_WAIT_AFTER_SECONDS = 10
CHECK_STATUS_WAIT_SECONDS = 30
MODIFY_WAIT_AFTER_SECONDS = 30
# Descriptions
INITIAL_DESCRIPTION = "Initial Description"
UPDATED_DESCRIPTION = "Updated Description"


def _retention_policy(index_pattern):
    """Build a retention lifecycle policy for a given index pattern."""
    return json.dumps({
        "Rules": [
            {
                "ResourceType": "index",
                "Resource": [f"index/{index_pattern}/*"],
                "MinIndexRetention": "81d",
            },
        ],
    })


@pytest.fixture
def simple_lifecycle_policy(request):
    lp_name = random_suffix_name("my-lifecycle-pol", 24)
    scope = random_suffix_name("col", 12)
    lp_policy = _retention_policy(scope)

    replacements = REPLACEMENT_VALUES.copy()
    replacements['LIFECYCLE_POLICY_NAME'] = lp_name
    replacements['LIFECYCLE_POLICY_DESCRIPTION'] = INITIAL_DESCRIPTION
    replacements['LIFECYCLE_POLICY_TYPE'] = "retention"
    replacements['LIFECYCLE_POLICY'] = lp_policy

    resource_data = load_resource(
        "lifecycle_policy",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, LIFECYCLE_POLICY_RESOURCE_PLURAL,
        lp_name, namespace="default",
    )
    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    assert k8s.get_resource_exists(ref)

    yield (ref, cr, scope)

    _, deleted = k8s.delete_custom_resource(
        ref,
        period_length=DELETE_WAIT_AFTER_SECONDS,
    )
    assert deleted
    # Confirm the backing AWS resource is gone, not just the Kubernetes object.
    time.sleep(DELETE_WAIT_AFTER_SECONDS)
    assert lifecycle_policy.get(lp_name, "retention") is None


@service_marker
@pytest.mark.canary
class TestLifecyclePolicy:
    def test_crud(self, simple_lifecycle_policy):
        ref, _, scope = simple_lifecycle_policy

        time.sleep(CHECK_STATUS_WAIT_SECONDS)
        condition.assert_synced(ref)

        # Check that lifecycle policy exists
        cr = k8s.get_resource(ref)
        assert cr is not None
        assert 'status' in cr
        assert 'policyVersion' in cr['status']

        assert 'spec' in cr
        assert 'type' in cr['spec']
        assert 'name' in cr['spec']

        name = cr['spec']['name']
        lp_type = cr['spec']['type']

        latest = lifecycle_policy.get(name, lp_type)
        assert latest is not None
        pre_update_version = cr['status']['policyVersion']

        # Update the lifecycle policy description and policy with a different retention
        updated_policy = _retention_policy(scope + "*")
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

        latest = lifecycle_policy.get(name, lp_type)
        assert latest is not None
        assert latest['policyVersion'] != pre_update_version
        # The service returns the policy document as parsed JSON (whitespace
        # stripped, keys reordered), so compare structurally against what we
        # submitted rather than as raw strings.
        assert latest['policy'] == json.loads(updated_policy)

        # Verify deletion happens in fixture teardown
