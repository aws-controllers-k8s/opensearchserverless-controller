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

def get(name, type):
    """Returns a dict containing the collection record with the supplied collection
    Name from the Opensearchserverless API.
    If no such collection exists, returns None.
    """
    c = boto3.client("opensearchserverless")

    try:
        resp = c.get_security_policy(
            name=name,
            type=type
        )
        if resp != None:
            return resp['securityPolicyDetail']
        return None
    except c.exceptions.ResourceNotFoundException:
        return None
