# Copyright Amazon.com Inc. or its affiliates. All Rights Reserved.
#
# Licensed under the Apache License, Version 2.0 (the "License"). You may
# not use this file except in compliance with the License. A copy of the
# License is located at
#
#          http://aws.amazon.com/apache2.0/
#
# or in the "license" file accompanying this file. This file is distributed
# on an "AS IS" BASIS, WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either
# express or implied. See the License for the specific language governing
# permissions and limitations under the License.

"""Utilities for working with LifecyclePolicy resources"""

import boto3


def get(name, type):
    """Returns a dict containing the lifecycle policy record with the supplied
    Name and Type from the OpenSearch Serverless API.
    If no such lifecycle policy exists, returns None.
    """
    c = boto3.client("opensearchserverless")

    resp = c.batch_get_lifecycle_policy(
        identifiers=[
            {
                "name": name,
                "type": type,
            }
        ]
    )

    if resp.get("lifecyclePolicyDetails"):
        return resp["lifecyclePolicyDetails"][0]

    # Check error details for NOT_FOUND
    if resp.get("lifecyclePolicyErrorDetails"):
        for err_detail in resp["lifecyclePolicyErrorDetails"]:
            if err_detail.get("errorCode") == "NOT_FOUND":
                return None

    return None
