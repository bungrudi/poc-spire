import base64
import ipaddress
import unittest

from cryptography import x509
from cryptography.hazmat.primitives import hashes, serialization
from cryptography.hazmat.primitives.asymmetric import ec
from cryptography.x509.oid import NameOID

from kubernetes_kubelet import validate_csr


class ServingCSRTests(unittest.TestCase):
    def fixture(self, extras=()):
        node, address = 'owned-control-plane', '172.31.244.2'
        key = ec.generate_private_key(ec.SECP256R1())
        names = [x509.DNSName(node), x509.IPAddress(ipaddress.ip_address(address)), *extras]
        csr = x509.CertificateSigningRequestBuilder().subject_name(x509.Name([
            x509.NameAttribute(NameOID.ORGANIZATION_NAME, 'system:nodes'),
            x509.NameAttribute(NameOID.COMMON_NAME, 'system:node:' + node)]))
        csr = csr.add_extension(x509.SubjectAlternativeName(names), critical=False).sign(key, hashes.SHA256())
        item = {'spec': {'signerName': 'kubernetes.io/kubelet-serving', 'username': 'system:node:' + node,
                         'groups': ['system:nodes', 'system:authenticated'], 'usages': ['digital signature', 'server auth'],
                         'request': base64.b64encode(csr.public_bytes(serialization.Encoding.PEM)).decode()}}
        return item, node, address

    def test_complete_san_set_rejects_other_identity_types(self):
        item, node, address = self.fixture()
        validate_csr(item, node, address)
        for extra in (x509.UniformResourceIdentifier('spiffe://unrelated/admin'),
                      x509.RFC822Name('foreign@example.invalid'), x509.DNSName('other-node'),
                      x509.IPAddress(ipaddress.ip_address('172.31.244.3'))):
            item, node, address = self.fixture([extra])
            with self.assertRaisesRegex(ValueError, 'SAN'):
                validate_csr(item, node, address)

    def test_foreign_requester_signer_and_usage_are_rejected(self):
        for field, bad in [('username', 'system:node:foreign'), ('signerName', 'kubernetes.io/kube-apiserver-client'),
                           ('usages', ['digital signature', 'server auth', 'client auth']),
                           ('groups', ['system:nodes', 'system:authenticated', 'system:masters'])]:
            item, node, address = self.fixture()
            item['spec'][field] = bad
            with self.assertRaises(ValueError):
                validate_csr(item, node, address)


if __name__ == '__main__':
    unittest.main()
