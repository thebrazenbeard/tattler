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
        for kind in ("tcp_session", "tcp_listener", "udp_endpoint", "udp_flow"):
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

    def test_package_manifest_version_and_arches_match_spk_info(self):
        release = json.loads((ROOT / "package-source" / "release.json").read_text(encoding="utf-8"))
        self.assertEqual(release["version"], spk_version())
        self.assertEqual(
            sorted(item["arch"] for item in release["releases"]),
            ["armv7", "armv8", "x86_64"],
        )

    def test_candidate_status_preserves_current_qualification_state(self):
        text = (ROOT / "README.md").read_text(encoding="utf-8")
        status = re.search(r"Current source/package candidate.*?(?=\n## |\Z)", text, re.S)
        self.assertIsNotNone(status)
        if spk_version() == "0.2.0-0005":
            self.assertIn("PROTOCOL_EVIDENCE_V1_SOURCE", status.group(0))
            self.assertIn("SEMANTIC_EVENTS_V1_SOURCE", status.group(0))
            self.assertIn("MULTIARCH_X86_64_ARMV7_ARMV8_SOURCE", status.group(0))
            self.assertIn("PACKAGE_SOURCE_BOUND_TO_CI_ARTIFACT", status.group(0))
            self.assertNotIn("PACKAGE_SOURCE_REBIND_PENDING", status.group(0))
            self.assertIn("EXACT_HEAD_CI_PENDING", status.group(0))
            self.assertNotIn("EXACT_HEAD_CI_RECEIPT_RECORDED", status.group(0))
            self.assertIn("docs/PROTOCOL_EVIDENCE_V1.md", status.group(0))
        elif spk_version() == "0.2.0-0004":
            self.assertIn("MULTIARCH_X86_64_ARMV7_ARMV8_SOURCE", status.group(0))
            self.assertIn("PACKAGE_SOURCE_BOUND_TO_CI_ARTIFACT", status.group(0))
            self.assertNotIn("PACKAGE_SOURCE_REBIND_PENDING", status.group(0))
            self.assertNotIn("EXACT_HEAD_CI_PENDING", status.group(0))
            self.assertIn("EXACT_HEAD_CI_RECEIPT_RECORDED", status.group(0))
            self.assertIn("docs/PUBLIC_RELEASE_QUALIFICATION_20261006_V0004.md", status.group(0))
        elif spk_version() == "0.2.0-0003":
            self.assertIn("PACKAGE_SOURCE_BOUND_TO_CI_ARTIFACT", status.group(0))
            self.assertIn("EXACT_HEAD_CI_RECEIPT_RECORDED", status.group(0))
            self.assertIn("docs/PUBLIC_RELEASE_QUALIFICATION_20261006_V0003.md", status.group(0))
        else:
            self.assertIn("PACKAGE_SOURCE_BOUND_TO_CI_ARTIFACT", status.group(0))
            self.assertIn("EXACT_HEAD_CI_RECEIPT_RECORDED", status.group(0))


if __name__ == "__main__":
    unittest.main()
