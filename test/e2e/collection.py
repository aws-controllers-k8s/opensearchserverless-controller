# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You may
# not use this file except in compliance with the License. A copy of the
# License is located at
#
# 	 http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is distributed
# on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
# express or implied. See the License for the specific language governing
# permissions and limitations under the License.

"""Utilities for working with Collection resources"""

import boto3

DEFAULT_WAIT_UNTIL_TIMEOUT_SECONDS = 15
DEFAULT_WAIT_UNTIL_INTERVAL_SECONDS = 15
DEFAULT_WAIT_UNTIL_EXISTS_TIMEOUT_SECONDS = 15
DEFAULT_WAIT_UNTIL_EXISTS_INTERVAL_SECONDS = 15
DEFAULT_WAIT_UNTIL_DELETED_TIMEOUT_SECONDS = 15
DEFAULT_WAIT_UNTIL_DELETED_INTERVAL_SECONDS = 15

def get(collection_id):
    """Returns a dict containing the collection record with the supplied collection
    Name from the Opensearchserverless API.

    If no such collection exists, returns None.
    """
    c = boto3.client("opensearchserverless")

    try:
        resp = c.batch_get_collection(
            ids=[collection_id],
        )
        for c in resp["collectionDetails"]:
            if c["id"] == collection_id:
                return c
    except c.exceptions.NotFoundException:
        return None


def get_tags(collection_arn):
    """Returns a list containing the tags that have been associated to the
    supplied collection.

    If no such collection exists, returns None.
    """
    c = boto3.client("opensearchserverless")
    try:
        resp = c.list_tags_for_resource(resourceArn=collection_arn)
        return resp["tags"]
    except c.exceptions.NotFoundException:
        return None
