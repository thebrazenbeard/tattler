"""Offline contract tests; no network requests or provider side effects."""

import importlib.util
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from threading import Thread
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
        return 200, {"packages": [package] if accepted else []}, {
            "sb-request-id": "fixture",
            "sb-project-ref": probe_module.EXPECTED_PROJECT_REF,
        }

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

    def test_foreign_https_origin_fails_before_any_request(self):
        get, requests = fake_catalog()
        with self.assertRaisesRegex(probe_module.ProbeError, "exact approved Supabase"):
            probe_module.probe(
                "https://foreign.example/functions/v1/tattler-package-source",
                COMMIT, "0.1.0-0007", "armada38x", 72806, get,
            )
        self.assertEqual(requests, [])

    def test_wrong_project_header_cannot_be_attributed(self):
        get, _ = fake_catalog()

        def foreign_header(url):
            status, body, headers = get(url)
            return status, body, {**headers, "sb-project-ref": "different-project"}

        with self.assertRaisesRegex(probe_module.ProbeError, "approved Supabase project"):
            self.run_probe(foreign_header)

    def test_real_local_http_transport_refuses_302_and_307_redirects(self):
        for status in (302, 307):
            class Redirect(BaseHTTPRequestHandler):
                def do_GET(self):
                    self.send_response(status)
                    self.send_header("Location", "https://foreign.example/forged-catalog")
                    self.end_headers()

                def log_message(self, _format, *args):
                    pass

            server = ThreadingHTTPServer(("127.0.0.1", 0), Redirect)
            thread = Thread(target=server.serve_forever, daemon=True)
            thread.start()
            try:
                with self.subTest(status=status), self.assertRaisesRegex(
                    probe_module.ProbeError, "redirect refused"
                ):
                    probe_module.live_get(
                        f"http://127.0.0.1:{server.server_port}/redirect"
                    )
            finally:
                server.shutdown()
                server.server_close()
                thread.join(timeout=2)


if __name__ == "__main__":
    unittest.main()
