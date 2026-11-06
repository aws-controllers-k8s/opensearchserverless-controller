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

from acktest.k8s import resource as k8s
from acktest import tags
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e import collection, collection_group
from e2e.tests.test_collection_group import simple_collection_group
from e2e.tests.test_security_policy import simple_security_policy

COLLECTION_RESOURCE_PLURAL = "collections"
DELETE_WAIT_AFTER_SECONDS = 10
# Collection provisioning can exceed 5 min under load; allow up to 15 minutes.
ACTIVE_WAIT_PERIODS = 30
ACTIVE_WAIT_PERIOD_LENGTH = 30  # 30 * 30s = up to 15 minutes
CHECK_STATUS_WAIT_SECONDS = 30
MODIFY_WAIT_AFTER_SECONDS = 30
INITIAL_DESCRIPTION = "Initial Description"
UPDATED_DESCRIPTION = "Updated Description"


@pytest.fixture
def simple_collection(simple_security_policy):
    # EncryptionType securityPolicy is required to create Collection resource.
    # Name the collection under the security policy's unique scope so its
    # (uniquely scoped) encryption policy covers it without colliding with the
    # policies created by other concurrent iterations.
    _, _, scope = simple_security_policy

    collection_name = random_suffix_name(scope, 28)
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


@pytest.fixture
def group_collection(simple_security_policy, simple_collection_group):
    policy_ref, _, scope = simple_security_policy
    group_ref, group_cr = simple_collection_group
    assert collection_group.wait_until(
        lambda: collection_group.is_synced(policy_ref)
    ), "Encryption policy did not reach ResourceSynced=True"
    collection_name = random_suffix_name(scope, 28)
    replacements = REPLACEMENT_VALUES.copy()
    replacements["COLLECTION_NAME"] = collection_name
    replacements["DESCRIPTION"] = INITIAL_DESCRIPTION
    resource_data = load_resource("collection", additional_replacements=replacements)
    resource_data["spec"].update({
        "type": "SEARCH",
        "standbyReplicas": "DISABLED",
        "collectionGroupRef": {"from": {"name": group_ref.name}},
    })
    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, COLLECTION_RESOURCE_PLURAL,
        collection_name, namespace="default",
    )
    k8s.create_custom_resource(ref, resource_data)
    try:
        k8s.wait_resource_consumed_by_controller(ref)
        yield ref, group_ref, group_cr
    finally:
        # Children must finish deletion before the group fixture tears down.
        if k8s.get_resource_exists(ref):
            _, deleted = k8s.delete_custom_resource(
                ref,
                wait_periods=ACTIVE_WAIT_PERIODS,
                period_length=ACTIVE_WAIT_PERIOD_LENGTH,
            )
            assert deleted, "Member Collection did not finish deletion"


@service_marker
@pytest.mark.canary
class TestCollection:
    def test_crud(self, simple_collection):
        ref, _ = simple_collection

        # Wait for the collection to reach ACTIVE (ResourceSynced=True).
        time.sleep(CHECK_STATUS_WAIT_SECONDS)
        assert k8s.wait_on_condition(
            ref, "ACK.ResourceSynced", "True",
            wait_periods=ACTIVE_WAIT_PERIODS,
            period_length=ACTIVE_WAIT_PERIOD_LENGTH,
        ), "Collection did not reach ACTIVE (ResourceSynced=True) in time"

        # Check that collection exists
        cr = k8s.get_resource(ref)
        assert cr is not None
        assert 'status' in cr
        assert 'id' in cr['status']
        collection_id = cr['status']['id']

        latest = collection.get(collection_id)
        assert latest is not None
        assert latest['description'] == INITIAL_DESCRIPTION

        assert 'ackResourceMetadata' in cr['status']
        assert 'arn' in cr['status']['ackResourceMetadata']
        arn = cr['status']['ackResourceMetadata']['arn']

        latest_tags = collection.get_tags(arn)
        desired_tags = cr['spec']['tags']
        tags.assert_ack_system_tags(
            tags=latest_tags,
            key_member_name='key',
            value_member_name='value'
        )
        tags.assert_equal_without_ack_tags(
            expected=desired_tags,
            actual=latest_tags,
            key_member_name='key',
            value_member_name='value'
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

        # Wait for the update to reconcile
        assert k8s.wait_on_condition(
            ref, "ACK.ResourceSynced", "True",
            wait_periods=ACTIVE_WAIT_PERIODS,
            period_length=ACTIVE_WAIT_PERIOD_LENGTH,
        ), "Collection did not reach ResourceSynced=True after update"

        cr = k8s.get_resource(ref)
        assert cr is not None
        assert 'status' in cr
        assert 'id' in cr['status']
        collection_id = cr['status']['id']

        latest = collection.get(collection_id)
        assert latest is not None
        assert latest['description'] == UPDATED_DESCRIPTION

        assert 'ackResourceMetadata' in cr['status']
        assert 'arn' in cr['status']['ackResourceMetadata']
        arn = cr['status']['ackResourceMetadata']['arn']

        assert 'spec' in cr
        assert 'tags' in cr['spec']
        latest_tags = collection.get_tags(arn)
        desired_tags = cr['spec']['tags']
        tags.assert_ack_system_tags(
            tags=latest_tags,
            key_member_name='key',
            value_member_name='value'
        )
        tags.assert_equal_without_ack_tags(
            expected=desired_tags,
            actual=latest_tags,
            key_member_name='key',
            value_member_name='value'
        )

    def test_group_reference_membership_and_deletion(
        self, group_collection, opensearchserverless_client
    ):
        ref, group_ref, group_cr = group_collection
        group_name = group_cr["spec"]["name"]
        group_id = group_cr["status"]["id"]
        assert collection_group.wait_until(
            lambda: collection_group.is_synced(ref)
        ), "Member Collection did not reach ResourceSynced=True"
        cr = k8s.get_resource(ref)
        collection_id = cr["status"]["id"]
        assert cr["spec"]["collectionGroupRef"]["from"]["name"] == group_ref.name
        resolved = k8s.get_resource_condition(ref, "ACK.ReferencesResolved")
        assert resolved is not None and resolved["status"] == "True"
        latest = collection.get(collection_id)
        assert latest is not None and latest["collectionGroupName"] == group_name

        def member_is_listed():
            return any(
                member["id"] == collection_id
                and member["collectionGroupName"] == group_name
                for member in collection_group.get_members(
                    group_name, opensearchserverless_client
                )
            )

        assert collection_group.wait_until(member_is_listed), "AWS did not list the member"

        # Child changes do not immediately enqueue the parent. A legitimate
        # group edit causes its next read to refresh the observed member list.
        description = "Group with an observed member"
        k8s.patch_custom_resource(
            group_ref, {"spec": {"description": description}}
        )

        def parent_observes_member():
            parent = k8s.get_resource(group_ref)
            members = parent["status"].get("memberCollections", [])
            latest_group = collection_group.get(
                group_id, opensearchserverless_client
            )
            return (
                collection_group.is_synced(group_ref)
                and latest_group["description"] == description
                and any(member["id"] == collection_id for member in members)
                and parent["status"]["numberOfCollections"]
                == latest_group["numberOfCollections"] == 1
            )

        assert collection_group.wait_until(
            parent_observes_member
        ), "CollectionGroup did not refresh its member status"
        parent = k8s.get_resource(group_ref)
        member = next(
            item for item in parent["status"]["memberCollections"]
            if item["id"] == collection_id
        )
        assert member["name"] == cr["spec"]["name"]
        assert member["arn"] == cr["status"]["ackResourceMetadata"]["arn"]
        assert member["collectionGroupName"] == group_name

        # The service rejects deleting a nonempty group; ACK must retain the
        # finalizer and a recoverable error while the Collection still exists.
        _, deleted = k8s.delete_custom_resource(
            group_ref, wait_periods=1, period_length=DELETE_WAIT_AFTER_SECONDS
        )
        assert not deleted, "A group containing a Collection was deleted"

        def deletion_is_retryable():
            parent = k8s.get_resource(group_ref)
            recoverable = k8s.get_resource_condition(group_ref, "ACK.Recoverable")
            return (
                bool(parent["metadata"].get("deletionTimestamp"))
                and bool(parent["metadata"].get("finalizers"))
                and recoverable is not None
                and recoverable["status"] == "True"
            )

        assert collection_group.wait_until(
            deletion_is_retryable
        ), "Nonempty group deletion did not retain a recoverable error"
        terminal = k8s.get_resource_condition(group_ref, "ACK.Terminal")
        assert terminal is None or terminal["status"] != "True"
        assert collection_group.get(group_id, opensearchserverless_client) is not None

        _, deleted = k8s.delete_custom_resource(
            ref,
            wait_periods=ACTIVE_WAIT_PERIODS,
            period_length=ACTIVE_WAIT_PERIOD_LENGTH,
        )
        assert deleted, "Member Collection did not finish deletion"
        assert collection_group.wait_until(
            lambda: collection.get(collection_id) is None
        ), "Member Collection still exists in AWS"
        assert collection_group.wait_until(
            lambda: not k8s.get_resource_exists(group_ref)
        ), "Empty group deletion did not resume after the member was removed"
        assert collection_group.wait_until(
            lambda: collection_group.get(
                group_id, opensearchserverless_client
            ) is None
        ), "CollectionGroup still exists in AWS"
