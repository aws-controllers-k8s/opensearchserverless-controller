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
from e2e import collection

COLLECTION_RESOURCE_PLURAL = "collections"
DELETE_WAIT_AFTER_SECONDS = 10
CHECK_STATUS_WAIT_SECONDS = 30
MODIFY_WAIT_AFTER_SECONDS = 30
INITIAL_DESCRIPTION = "Initial Description"
UPDATED_DESCRIPTION = "UPDATEd Description"


@pytest.fixture(scope="module")
def simple_collection():
    collection_name = random_suffix_name("my-collection", 24)

    replacements = REPLACEMENT_VALUES.copy()
    replacements['COLLECTION_NAME'] = collection_name
    replacements['DESCRIPTION'] = "Initial Description"

    resource_data = load_resource(
        "collection",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, COLLECTION_RESOURCE_PLURAL,
        collection_name, namespace="default",
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
    def test_crud(self, simple_collection):
        ref, _ = simple_collection

        time.sleep(CHECK_STATUS_WAIT_SECONDS)
        condition.assert_synced(ref)

        # Check that collection exists
        cr = k8s.get_resource(ref)
        assert cr is not None
        assert 'status' in cr
        assert 'id' in cr['status']
        collection_id = cr['status']['id']

        latest = collection.get(collection_id)
        assert latest is not None
        latest['description'] == INITIAL_DESCRIPTION

        assert 'ackResourceMetadata' in cr['status']
        assert 'arn' in cr['status']['ackResourceMetadata']
        arn = cr['status']['ackResourceMetadata']['arn']

        latest_tags = collection.get_tags(arn)
        desired_tags = cr['spec']['tags']
        tags.assert_ack_system_tags(
            tags=latest_tags,
        )
        tags.assert_equal_without_ack_tags(
            expected=desired_tags,
            actual=latest_tags,
        )

        # Update the collection
        updates = {
            "spec": {
                "description": UPDATED_DESCRIPTION,
                "tags": [{
                    "key": "newKey",
                    "value": "newVal"
                }]
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        cr = k8s.get_resource(ref)
        assert cr is not None
        assert 'status' in cr
        assert 'id' in cr['status']
        collection_id = cr['status']['id']

        latest = collection.get(collection_id)
        assert latest is not None
        latest['description'] == INITIAL_DESCRIPTION

        assert 'ackResourceMetadata' in cr['status']
        assert 'arn' in cr['status']['ackResourceMetadata']
        arn = cr['status']['ackResourceMetadata']['arn']

        assert 'spec' in cr
        assert 'tags' in cr['spec']
        latest_tags = collection.get_tags(arn)
        desired_tags = cr['spec']['tags']
        tags.assert_ack_system_tags(
            tags=latest_tags,
        )
        tags.assert_equal_without_ack_tags(
            expected=desired_tags,
            actual=latest_tags,
        )
