import json
import re
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def spk_version():
    for line in (ROOT / "spk" / "INFO").read_text(encoding="utf-8").splitlines():
        if line.startswith("version="):
            return line.split("=", 1)[1].strip().strip('"')
    raise AssertionError("spk/INFO version missing")


class ReleaseDocsTests(unittest.TestCase):
    def test_readme_uses_typed_observation_semantics(self):
        text = (ROOT / "README.md").read_text(encoding="utf-8")
        for kind in ("tcp_session", "tcp_listener", "udp_endpoint"):
            self.assertIn(kind, text)
        self.assertNotIn("Windows UDP endpoint capture", text)
        self.assertNotIn("Unconnected inbound UDP is not represented as a connection", text)
        self.assertRegex(text, r"UDP endpoint.*does not prove|does not prove.*UDP")

    def test_readme_documents_desktop_companion_and_license(self):
        text = (ROOT / "README.md").read_text(encoding="utf-8")
        self.assertIn("desktop/", text)
        self.assertIn("Wails", text)
        self.assertIn("source-visible proprietary", text)
        self.assertIn("LICENSE", text)

    def test_package_manifest_version_matches_spk_info(self):
        release = json.loads((ROOT / "package-source" / "release.json").read_text(encoding="utf-8"))
        self.assertEqual(release["version"], spk_version())

    def test_candidate_status_does_not_claim_exact_head_ci_success_yet(self):
        text = (ROOT / "README.md").read_text(encoding="utf-8")
        status = re.search(r"Current source/package candidate.*?(?=\n## |\Z)", text, re.S)
        self.assertIsNotNone(status)
        self.assertNotIn("EXACT_HEAD_CI_PASS", status.group(0))
        self.assertIn("EXACT_HEAD_CI_PENDING", status.group(0))


if __name__ == "__main__":
    unittest.main()
