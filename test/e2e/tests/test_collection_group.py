# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You may
# not use this file except in compliance with the License. A copy of the
# License is located at
#
#     http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is distributed
# on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
# express or implied. See the License for the specific language governing
# permissions and limitations under the License.

"""Integration tests for the OpenSearch Serverless CollectionGroup resource."""

import pytest

from acktest import tags
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import CRD_GROUP, CRD_VERSION, load_resource, service_marker
from e2e import collection_group
from e2e.replacement_values import REPLACEMENT_VALUES

INITIAL_DESCRIPTION = "Initial group description"
UPDATED_DESCRIPTION = "Updated group description"
DELETE_WAIT_PERIODS = 30
DELETE_WAIT_PERIOD_LENGTH = 30


@pytest.fixture
def simple_collection_group(opensearchserverless_client):
    group_name = random_suffix_name("ack-group", 24)
    replacements = REPLACEMENT_VALUES.copy()
    replacements["GROUP_NAME"] = group_name
    replacements["GROUP_DESCRIPTION"] = INITIAL_DESCRIPTION
    resource_data = load_resource(
        "collection_group", additional_replacements=replacements
    )
    requested_tags = [tag.copy() for tag in resource_data["spec"]["tags"]]
    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, "collectiongroups", group_name, namespace="default"
    )
    group_id = None
    k8s.create_custom_resource(ref, resource_data)
    try:
        k8s.wait_resource_consumed_by_controller(ref)
        assert collection_group.wait_until(
            lambda: collection_group.is_synced(ref)
        ), "CollectionGroup did not reach ResourceSynced=True"
        cr = k8s.get_resource(ref)
        group_id = cr["status"]["id"]
        latest = collection_group.get(group_id, opensearchserverless_client)
        assert latest is not None
        # Compare against the requested fixture, so a controller that drops
        # tags from both the CR and AWS cannot pass by comparing those two.
        for actual in (cr["spec"]["tags"], latest["tags"]):
            tags.assert_equal_without_ack_tags(
                expected=requested_tags,
                actual=actual,
                key_member_name="key",
                value_member_name="value",
            )
        yield ref, cr
    finally:
        if k8s.get_resource_exists(ref):
            current = k8s.get_resource(ref)
            group_id = current.get("status", {}).get("id", group_id)
            _, deleted = k8s.delete_custom_resource(
                ref,
                wait_periods=DELETE_WAIT_PERIODS,
                period_length=DELETE_WAIT_PERIOD_LENGTH,
            )
            assert deleted, "Empty CollectionGroup did not finish deletion"
        if group_id is not None:
            assert collection_group.wait_until(
                lambda: collection_group.get(
                    group_id, opensearchserverless_client
                ) is None
            ), "CollectionGroup still exists in AWS after Kubernetes deletion"


@service_marker
@pytest.mark.canary
class TestCollectionGroup:
    def test_crud(self, simple_collection_group, opensearchserverless_client):
        ref, cr = simple_collection_group
        group_id = cr["status"]["id"]
        initial_limits = cr["spec"]["capacityLimits"].copy()
        initial_tags = [tag.copy() for tag in cr["spec"]["tags"]]
        latest = collection_group.get(group_id, opensearchserverless_client)
        assert latest["name"] == cr["spec"]["name"]
        assert latest["description"] == INITIAL_DESCRIPTION
        assert latest["standbyReplicas"] == cr["spec"]["standbyReplicas"]
        assert latest["generation"] == cr["spec"]["generation"]
        assert latest["capacityLimits"] == initial_limits
        assert latest["arn"] == cr["status"]["ackResourceMetadata"]["arn"]
        assert latest["createdDate"] == cr["status"]["createdDate"]
        assert latest["numberOfCollections"] == cr["status"]["numberOfCollections"] == 0
        assert cr["status"].get("memberCollections", []) == []
        self.assert_tags(cr, latest, initial_tags)

        # Description-only updates must preserve all four capacity limits.
        k8s.patch_custom_resource(
            ref, {"spec": {"description": UPDATED_DESCRIPTION}}
        )
        assert collection_group.wait_until(
            lambda: collection_group.is_synced(ref)
            and collection_group.get(
                group_id, opensearchserverless_client
            )["description"] == UPDATED_DESCRIPTION
        ), "Description update did not reconcile"
        cr = k8s.get_resource(ref)
        latest = collection_group.get(group_id, opensearchserverless_client)
        assert cr["spec"]["description"] == latest["description"] == UPDATED_DESCRIPTION
        assert cr["spec"]["capacityLimits"] == latest["capacityLimits"] == initial_limits
        self.assert_tags(cr, latest, initial_tags)

        updated_limits = {
            "maxIndexingCapacityInOCU": 4,
            "maxSearchCapacityInOCU": 4,
            "minIndexingCapacityInOCU": 2,
            "minSearchCapacityInOCU": 2,
        }
        k8s.patch_custom_resource(ref, {"spec": {"capacityLimits": updated_limits}})
        assert collection_group.wait_until(
            lambda: collection_group.is_synced(ref)
            and collection_group.get(
                group_id, opensearchserverless_client
            )["capacityLimits"] == updated_limits
        ), "Capacity update did not reconcile"
        cr = k8s.get_resource(ref)
        latest = collection_group.get(group_id, opensearchserverless_client)
        assert cr["spec"]["capacityLimits"] == latest["capacityLimits"] == updated_limits
        self.assert_tags(cr, latest, initial_tags)

        # Exercise upsert, removal, and addition without a business-field update.
        updated_tags = [
            {"key": "owner", "value": "updated"},
            {"key": "added", "value": "updated"},
        ]
        k8s.patch_custom_resource(ref, {"spec": {"tags": updated_tags}})

        def tags_reconciled():
            if not collection_group.is_synced(ref):
                return False
            latest = collection_group.get(group_id, opensearchserverless_client)
            actual = {tag["key"]: tag["value"] for tag in latest["tags"]}
            return (
                actual.get("owner") == "updated"
                and actual.get("added") == "updated"
                and "remove-me" not in actual
            )

        assert collection_group.wait_until(tags_reconciled), "Tag update did not reconcile"
        cr = k8s.get_resource(ref)
        latest = collection_group.get(group_id, opensearchserverless_client)
        tags.assert_equal_without_ack_tags(
            expected=updated_tags,
            actual=cr["spec"]["tags"],
            key_member_name="key",
            value_member_name="value",
        )
        assert latest["description"] == UPDATED_DESCRIPTION
        assert latest["capacityLimits"] == updated_limits
        self.assert_tags(cr, latest, updated_tags)
        # The fixture checks Kubernetes and AWS deletion of this empty group.

    @staticmethod
    def assert_tags(cr, latest, expected_tags):
        tags.assert_ack_system_tags(
            tags=latest["tags"], key_member_name="key", value_member_name="value"
        )
        for actual in (cr["spec"]["tags"], latest["tags"]):
            tags.assert_equal_without_ack_tags(
                expected=expected_tags,
                actual=actual,
                key_member_name="key",
                value_member_name="value",
            )
