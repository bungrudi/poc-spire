import json
import unittest
from unittest.mock import patch
import kubernetes_attestation as a

class RegistrationTests(unittest.TestCase):
    def test_real_pinned_owner_selector_keeps_kind_and_uid_conjunction(self):
        pod={'metadata':{'namespace':'owned','uid':'pod','labels':{'poc-role':'money'},
                        'ownerReferences':[{'kind':'ReplicaSet','uid':'owner','controller':True}]},
             'spec':{'serviceAccountName':'money'}}
        with patch.object(a,'admin',side_effect=[json.dumps({'entries':[]}), '{}']) as admin:
            a.register({'run':'run'},'parent','spiffe://poc.internal/money',pod,'money','repo@sha256:actual','executable',1000)
        args=admin.call_args.args
        selectors=[args[i+1] for i,x in enumerate(args[:-1]) if x=='-selector']
        self.assertIn('k8s:pod-owner-uid:ReplicaSet:owner',selectors)
        self.assertNotIn('k8s:pod-owner-uid:owner',selectors)
        for required in ['unix:uid:1000','unix:sha256:executable','k8s:pod-uid:pod','k8s:container-image:repo@sha256:actual','k8s:sa:money','k8s:container-name:money']:
            self.assertIn(required,selectors)
        self.assertEqual(args[args.index('-parentID')+1],'parent')
