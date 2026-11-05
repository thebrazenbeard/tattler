"""Offline contract tests; no network requests or provider side effects."""

import importlib.util
import unittest
from pathlib import Path
from urllib.parse import parse_qs, urlsplit

MODULE_PATH = Path(__file__).with_name("supabase_catalog_live_probe.py")
SPEC = importlib.util.spec_from_file_location("supabase_catalog_live_probe", MODULE_PATH)
probe_module = importlib.util.module_from_spec(SPEC)
SPEC.loader.exec_module(probe_module)

COMMIT = probe_module.DEPLOYED_SOURCE_COMMIT
ENDPOINT = probe_module.DEFAULT_ENDPOINT


def fake_catalog(version="0.1.0-0007", commit=COMMIT, offer_unknown=False):
    requests = []

    def get(url):
        requests.append(url)
        q = parse_qs(urlsplit(url).query)
        arch = q["arch"][0]
        build = int(q["build"][0])
        accepted = (arch == "armada38x" or offer_unknown) and build >= 72806
        package = {
            "package": "Tattler",
            "version": version,
            "link": (
                "https://raw.githubusercontent.com/thebrazenbeard/tattler/"
                f"{commit}/package-source/public/releases/Tattler-armada38x-0.1.0-0007.spk"
            ),
            "size": 2273280,
            "md5": "d22eae80bd6b8226aaa168bdf403910a",
        }
        return 200, {"packages": [package] if accepted else []}, {"sb-request-id": "fixture"}

    return get, requests


class ReadOnlyCatalogProbeTests(unittest.TestCase):
    def run_probe(self, get):
        return probe_module.probe(ENDPOINT, COMMIT, "0.1.0-0007", "armada38x", 72806, get)

    def test_supported_and_negative_cases_are_only_three_gets(self):
        get, requests = fake_catalog()
        result = self.run_probe(get)
        self.assertEqual(result["result"], "LIVE_PROVIDER_HTTP_PASS")
        self.assertEqual(len(requests), 3)
        self.assertEqual([parse_qs(urlsplit(x).query)["build"][0] for x in requests],
                         ["72806", "72806", "72805"])

    def test_source_commit_mismatch_fails_closed(self):
        get, _ = fake_catalog(commit="0" * 40)
        with self.assertRaisesRegex(probe_module.ProbeError, "immutable commit"):
            self.run_probe(get)

    def test_release_version_mismatch_fails_closed(self):
        get, _ = fake_catalog(version="0.2.0-0004")
        with self.assertRaisesRegex(probe_module.ProbeError, "version differs"):
            self.run_probe(get)

    def test_unknown_architecture_must_not_be_offered(self):
        get, _ = fake_catalog(offer_unknown=True)
        with self.assertRaisesRegex(probe_module.ProbeError, "unknown architecture"):
            self.run_probe(get)

    def test_https_required_without_credentials(self):
        get, requests = fake_catalog()
        with self.assertRaisesRegex(probe_module.ProbeError, "HTTPS"):
            probe_module.probe("http://example.test", COMMIT, "0.1.0-0007", "armada38x", 72806, get)
        self.assertEqual(requests, [])


if __name__ == "__main__":
    unittest.main()
