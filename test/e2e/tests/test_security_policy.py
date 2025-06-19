# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You may
# not use this file except in compliance with the License. A copy of the
# License is located at
#
#	 http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is distributed
# on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
# express or implied. See the License for the specific language governing
# permissions and limitations under the License.

"""Integration tests for the OpensearchServerless Collection resource"""

import time

import pytest

from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest import tags
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e import security_policy

COLLECTION_RESOURCE_PLURAL = "securitypolicies"
# Wait time
DELETE_WAIT_AFTER_SECONDS = 10
CHECK_STATUS_WAIT_SECONDS = 30
MODIFY_WAIT_AFTER_SECONDS = 30
# Descriptions
INITIAL_DESCRIPTION = "Initial Description"
UPDATED_DESCRIPTION = "Updated Description"
# Encryption Policy
INITIAL_ENCRYPTION_POLICY = '{"AWSOwnedKey":true,"Rules":[{"Resource":["collection/mycollection"],"ResourceType":"collection"}]}'
UPDATED_ENCRYPTION_POLICY = '{"AWSOwnedKey":true,"Rules":[{"Resource":["collection/*"],"ResourceType":"collection"}]}'
# Network Policy
INITIAL_NETWORK_POLICY = '[{"Rules": [{"ResourceType": "collection","Resource": ["collection/logs*"]},{"ResourceType": "dashboard","Resource": ["collection/logs*"]}],"AllowFromPublic": true}]'
UPDATED_NETWORK_POLICY = '[{"Rules": [{"ResourceType": "collection","Resource": ["collection/*"]},{"ResourceType": "dashboard","Resource": ["collection/logs*"]}],"AllowFromPublic": true}]'



@pytest.fixture
def simple_security_policy(request):
    sp_name = random_suffix_name("my-security-policy", 24)
    marker = request.node.get_closest_marker("resource_data")
    assert marker is not None
    data = marker.args[0]
    # either encryption or network
    assert 'type' in data
    assert 'policy' in data

    replacements = REPLACEMENT_VALUES.copy()
    replacements['SECURITY_POLICY_NAME'] = sp_name
    replacements['SECURITY_POLICY_DESCRIPTION'] = INITIAL_DESCRIPTION
    replacements['SECURITY_POLICY_TYPE'] = data['type']
    replacements['SECURITY_POLICY'] = data['policy']

    resource_data = load_resource(
        "security_policy",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, COLLECTION_RESOURCE_PLURAL,
        sp_name, namespace="default",
    )
    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    assert k8s.get_resource_exists(ref)

    yield (ref, cr)

    _, deleted = k8s.delete_custom_resource(
        ref,
        period_length=DELETE_WAIT_AFTER_SECONDS,
    )
    assert deleted


@service_marker
@pytest.mark.canary
class TestCollection:
    @pytest.mark.resource_data({'type': 'encryption', 'policy': INITIAL_ENCRYPTION_POLICY})
    def test_encryption_crud(self, simple_security_policy):
        ref, _ = simple_security_policy

        time.sleep(CHECK_STATUS_WAIT_SECONDS)
        condition.assert_synced(ref)

        # Check that security policy exists
        cr = k8s.get_resource(ref)
        assert cr is not None
        assert 'status' in cr
        assert 'policyVersion' in cr['status']

        assert 'spec' in cr
        assert 'type' in cr['spec']
        assert 'name' in cr['spec']

        name = cr['spec']['name']
        type = cr['spec']['type']

        latest = security_policy.get(name, type)
        assert latest is not None
        latest['description'] == INITIAL_DESCRIPTION
        latest['policy'] == INITIAL_ENCRYPTION_POLICY

        # Update the security policy
        updates = {
            "spec": {
                "description": UPDATED_DESCRIPTION,
                "policy": UPDATED_ENCRYPTION_POLICY
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        cr = k8s.get_resource(ref)
        latest = security_policy.get(name, type)
        assert latest is not None
        latest['description'] == UPDATED_DESCRIPTION
        latest['policy'] == UPDATED_ENCRYPTION_POLICY

    @pytest.mark.resource_data({'type': 'network', 'policy': INITIAL_NETWORK_POLICY})
    def test_network_crud(self, simple_security_policy):
        ref, _ = simple_security_policy

        time.sleep(CHECK_STATUS_WAIT_SECONDS)
        condition.assert_synced(ref)

        # Check that security policy exists
        cr = k8s.get_resource(ref)
        assert cr is not None
        assert 'status' in cr
        assert 'policyVersion' in cr['status']

        assert 'spec' in cr
        assert 'type' in cr['spec']
        assert 'name' in cr['spec']

        name = cr['spec']['name']
        type = cr['spec']['type']

        latest = security_policy.get(name, type)
        assert latest is not None
        latest['description'] == INITIAL_DESCRIPTION
        latest['policy'] == INITIAL_NETWORK_POLICY

        # Update the security policy
        updates = {
            "spec": {
                "description": UPDATED_DESCRIPTION,
                "policy": UPDATED_NETWORK_POLICY
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        cr = k8s.get_resource(ref)
        latest = security_policy.get(name, type)
        assert latest is not None
        latest['description'] == UPDATED_DESCRIPTION
        latest['policy'] == UPDATED_NETWORK_POLICY
