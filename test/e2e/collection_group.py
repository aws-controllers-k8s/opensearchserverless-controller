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

"""Utilities for verifying CollectionGroup resources through AWS and Kubernetes."""

import time

from acktest.k8s import resource as k8s


def get(group_id, client):
    response = client.batch_get_collection_group(ids=[group_id])
    for error in response.get("collectionGroupErrorDetails", []):
        if error.get("errorCode") != "NOT_FOUND":
            raise RuntimeError(f"Collection group lookup failed: {error}")
    for detail in response.get("collectionGroupDetails", []):
        if detail["id"] == group_id:
            detail["tags"] = client.list_tags_for_resource(
                resourceArn=detail["arn"]
            )["tags"]
            return detail
    return None


def get_members(group_name, client):
    # The pinned botocore model supports nextToken but defines no paginators.
    members = []
    request = {"collectionFilters": {"collectionGroupName": group_name}}
    seen_tokens = set()
    while True:
        page = client.list_collections(**request)
        members.extend(page.get("collectionSummaries", []))
        token = page.get("nextToken")
        if not token:
            return members
        if token in seen_tokens:
            raise RuntimeError("ListCollections returned a repeated pagination token")
        seen_tokens.add(token)
        request["nextToken"] = token


def is_synced(ref):
    condition = k8s.get_resource_condition(ref, "ACK.ResourceSynced")
    return condition is not None and condition.get("status") == "True"


def wait_until(predicate, timeout_seconds=900, interval_seconds=15):
    """Poll a concrete outcome as well as Synced to avoid accepting stale True."""
    deadline = time.monotonic() + timeout_seconds
    while True:
        if predicate():
            return True
        remaining = deadline - time.monotonic()
        if remaining <= 0:
            return False
        time.sleep(min(interval_seconds, remaining))
