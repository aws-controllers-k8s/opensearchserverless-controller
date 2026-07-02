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

import json
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


# OpenSearch Serverless rejects a security policy whose resource pattern
# overlaps an existing policy of the same type with a ConflictException. The
# soak suite runs these tests continuously across parallel workers, so shared
# patterns like "collection/*" or "collection/mycollection" collide between
# concurrent iterations. Build every policy against a unique per-fixture
# collection scope (collection/<unique>) instead so iterations never conflict.
def _encryption_policy(resource):
    return json.dumps({
        "AWSOwnedKey": True,
        "Rules": [{"Resource": [resource], "ResourceType": "collection"}],
    })


def _network_policy(resource):
    return json.dumps([{
        "Rules": [
            {"ResourceType": "collection", "Resource": [resource]},
            {"ResourceType": "dashboard", "Resource": [resource]},
        ],
        "AllowFromPublic": True,
    }])


@pytest.fixture
def simple_security_policy(request):
    sp_name = random_suffix_name("my-security-policy", 24)
    # Unique collection scope for this fixture instance so policies created by
    # concurrent iterations never overlap (see note above).
    scope = random_suffix_name("col", 12)
    resource = f"collection/{scope}"

    marker = request.node.get_closest_marker("resource_data")
    # Default (no marker): an encryption policy covering this instance's unique
    # scope, used when creating Collections. The trailing wildcard lets it cover
    # a collection named "<scope>-...".
    sp_type = "encryption"
    sp_policy = _encryption_policy(f"{resource}*")
    if marker is not None:
        data = marker.args[0]
        # either encryption or network
        assert 'type' in data
        sp_type = data['type']
        if sp_type == "network":
            sp_policy = _network_policy(resource)
        else:
            sp_policy = _encryption_policy(resource)

    replacements = REPLACEMENT_VALUES.copy()
    replacements['SECURITY_POLICY_NAME'] = sp_name
    replacements['SECURITY_POLICY_DESCRIPTION'] = INITIAL_DESCRIPTION
    replacements['SECURITY_POLICY_TYPE'] = sp_type
    replacements['SECURITY_POLICY'] = sp_policy

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

    yield (ref, cr, scope)

    _, deleted = k8s.delete_custom_resource(
        ref,
        period_length=DELETE_WAIT_AFTER_SECONDS,
    )
    assert deleted


@service_marker
@pytest.mark.canary
class TestCollection:
    @pytest.mark.resource_data({'type': 'encryption'})
    def test_encryption_crud(self, simple_security_policy):
        ref, _, scope = simple_security_policy

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

        # Update the security policy to a broader (but still unique) scope
        updates = {
            "spec": {
                "description": UPDATED_DESCRIPTION,
                "policy": _encryption_policy(f"collection/{scope}*")
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        cr = k8s.get_resource(ref)
        latest = security_policy.get(name, type)
        assert latest is not None

    @pytest.mark.resource_data({'type': 'network'})
    def test_network_crud(self, simple_security_policy):
        ref, _, scope = simple_security_policy

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

        # Update the security policy to a broader (but still unique) scope
        updates = {
            "spec": {
                "description": UPDATED_DESCRIPTION,
                "policy": _network_policy(f"collection/{scope}*")
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        cr = k8s.get_resource(ref)
        latest = security_policy.get(name, type)
        assert latest is not None
