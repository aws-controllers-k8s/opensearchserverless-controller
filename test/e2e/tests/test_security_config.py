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

"""Integration tests for the OpensearchServerless SecurityConfig resource"""

import base64
import datetime
import time

import pytest

from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import rsa
from cryptography.x509.oid import NameOID

from acktest.k8s import condition
from acktest.k8s import resource as k8s
from acktest.resources import random_suffix_name
from e2e import service_marker, CRD_GROUP, CRD_VERSION, load_resource
from e2e.replacement_values import REPLACEMENT_VALUES
from e2e import security_config

SECURITY_CONFIG_RESOURCE_PLURAL = "securityconfigs"
# Wait time
DELETE_WAIT_AFTER_SECONDS = 10
CHECK_STATUS_WAIT_SECONDS = 30
MODIFY_WAIT_AFTER_SECONDS = 30
# Descriptions
INITIAL_DESCRIPTION = "Initial Description"
UPDATED_DESCRIPTION = "Updated Description"
# Session timeouts (in minutes)
INITIAL_SESSION_TIMEOUT = 60
UPDATED_SESSION_TIMEOUT = 120


def _generate_saml_metadata():
    """Generate a valid SAML IdP metadata XML document with a self-signed
    X.509 certificate.  OpenSearch Serverless validates that the metadata
    contains a KeyDescriptor with a certificate (the ``Cert`` field in its
    internal JSON schema), so a bare metadata document without one is
    rejected with ``ValidationException: Policy json is invalid, error:
    [$.Cert: null found, string expected]``.

    Uses the ``cryptography`` Python library (available in the test
    environment as a transitive dependency of ``kubernetes``) instead of
    shelling out to ``openssl``, which is not installed in the test
    container.
    """
    # Generate a self-signed certificate using the cryptography library
    key = rsa.generate_private_key(public_exponent=65537, key_size=2048)
    subject = issuer = x509.Name([
        x509.NameAttribute(NameOID.COMMON_NAME, "idp.example.com"),
    ])
    cert = (
        x509.CertificateBuilder()
        .subject_name(subject)
        .issuer_name(issuer)
        .public_key(key.public_key())
        .serial_number(x509.random_serial_number())
        .not_valid_before(datetime.datetime.now(datetime.timezone.utc))
        .not_valid_after(datetime.datetime.now(datetime.timezone.utc) + datetime.timedelta(days=3650))
        .sign(key, hashes.SHA256())
    )
    cert_der = cert.public_bytes(serialization.Encoding.DER)
    cert_b64 = base64.b64encode(cert_der).decode("ascii")

    return (
        '<?xml version="1.0" encoding="UTF-8"?>'
        '<md:EntityDescriptor xmlns:md="urn:oasis:names:tc:SAML:2.0:metadata"'
        ' entityID="https://idp.example.com/saml">'
        '<md:IDPSSODescriptor'
        ' protocolSupportEnumeration="urn:oasis:names:tc:SAML:2.0:protocol">'
        '<md:KeyDescriptor use="signing">'
        '<ds:KeyInfo xmlns:ds="http://www.w3.org/2000/09/xmldsig#">'
        '<ds:X509Data>'
        f'<ds:X509Certificate>{cert_b64}</ds:X509Certificate>'
        '</ds:X509Data>'
        '</ds:KeyInfo>'
        '</md:KeyDescriptor>'
        '<md:SingleSignOnService'
        ' Binding="urn:oasis:names:tc:SAML:2.0:bindings:HTTP-Redirect"'
        ' Location="https://idp.example.com/sso"/>'
        '</md:IDPSSODescriptor>'
        '</md:EntityDescriptor>'
    )


@pytest.fixture
def simple_security_config(request):
    sc_name = random_suffix_name("my-sec-config", 24)

    saml_metadata = _generate_saml_metadata()

    replacements = REPLACEMENT_VALUES.copy()
    replacements['SECURITY_CONFIG_NAME'] = sc_name
    replacements['SECURITY_CONFIG_DESCRIPTION'] = INITIAL_DESCRIPTION
    replacements['SECURITY_CONFIG_TYPE'] = "saml"
    replacements['SECURITY_CONFIG_SAML_METADATA'] = saml_metadata
    replacements['SECURITY_CONFIG_SESSION_TIMEOUT'] = str(INITIAL_SESSION_TIMEOUT)

    resource_data = load_resource(
        "security_config",
        additional_replacements=replacements,
    )

    ref = k8s.CustomResourceReference(
        CRD_GROUP, CRD_VERSION, SECURITY_CONFIG_RESOURCE_PLURAL,
        sc_name, namespace="default",
    )
    k8s.create_custom_resource(ref, resource_data)
    cr = k8s.wait_resource_consumed_by_controller(ref)

    assert k8s.get_resource_exists(ref)

    yield (ref, cr, saml_metadata)

    # Capture the config ID *before* deletion so we can verify AWS-side
    # cleanup.  Use get_resource_exists first to avoid a 404 exception
    # if the CR was already removed (e.g. failed creation removed the
    # finalizer and the object was garbage-collected).
    config_id = None
    if k8s.get_resource_exists(ref):
        cr = k8s.get_resource(ref)
        if cr is not None and 'status' in cr and 'id' in cr['status']:
            config_id = cr['status']['id']

    _, deleted = k8s.delete_custom_resource(
        ref,
        period_length=DELETE_WAIT_AFTER_SECONDS,
    )
    assert deleted

    # Confirm the backing AWS resource is gone, not just the Kubernetes object.
    if config_id is not None:
        time.sleep(DELETE_WAIT_AFTER_SECONDS)
        assert security_config.get(config_id) is None


@service_marker
@pytest.mark.canary
class TestSecurityConfig:
    def test_crud(self, simple_security_config):
        ref, _, saml_metadata = simple_security_config

        time.sleep(CHECK_STATUS_WAIT_SECONDS)
        condition.assert_synced(ref)

        # Check that security config exists
        cr = k8s.get_resource(ref)
        assert cr is not None
        assert 'status' in cr
        assert 'configVersion' in cr['status']
        assert 'id' in cr['status']
        assert 'createdDate' in cr['status']

        assert 'spec' in cr
        assert 'type' in cr['spec']
        assert 'name' in cr['spec']
        assert 'samlOptions' in cr['spec']

        config_id = cr['status']['id']
        pre_update_version = cr['status']['configVersion']

        # Verify the resource exists in AWS
        latest = security_config.get(config_id)
        assert latest is not None
        assert latest['description'] == INITIAL_DESCRIPTION

        # Update the security config description and session timeout
        updates = {
            "spec": {
                "description": UPDATED_DESCRIPTION,
                "samlOptions": {
                    "metadata": saml_metadata,
                    "sessionTimeout": UPDATED_SESSION_TIMEOUT,
                },
            },
        }
        k8s.patch_custom_resource(ref, updates)
        time.sleep(MODIFY_WAIT_AFTER_SECONDS)

        cr = k8s.get_resource(ref)
        assert cr is not None
        assert cr['spec'].get('description') == UPDATED_DESCRIPTION
        assert cr['status']['configVersion'] != pre_update_version

        # Verify the update in AWS
        latest = security_config.get(config_id)
        assert latest is not None
        assert latest['description'] == UPDATED_DESCRIPTION
        assert latest['configVersion'] != pre_update_version

        # Verify deletion happens in fixture teardown
