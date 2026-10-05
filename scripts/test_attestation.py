import unittest
import subprocess
from unittest.mock import patch
from attestation import verify_observation, run


class EvidenceTest(unittest.TestCase):
    def test_timeout_does_not_disclose_bootstrap_arguments(self):
        arguments=["spire-server","entry","-parentID","test-only-secret"]
        with patch("attestation.subprocess.run",side_effect=subprocess.TimeoutExpired(arguments,1)):
            with self.assertRaises(RuntimeError) as error:
                run(arguments)
        self.assertNotIn("test-only-secret",str(error.exception))

    def test_missing_or_extra_selector_cannot_count_as_executable_proof(self):
        selectors = ["unix:uid:1000", "unix:sha256:binary",
                     "docker:image_config_digest:image", "docker:label:poc-spire.attestation.role:approved"]
        verify_observation(selectors, 1000, "image", "binary", "approved")
        for invalid in (selectors[:-1], selectors + ["unix:sha256:other"],
                        [s.replace("binary", "other") for s in selectors]):
            with self.assertRaises(RuntimeError):
                verify_observation(invalid, 1000, "image", "binary", "approved")


if __name__ == "__main__":
    unittest.main()
