import tempfile
import unittest
from pathlib import Path

from tools import release_hygiene


class HygieneTests(unittest.TestCase):
    def scan(self, files):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            for name, text in files.items():
                path = root / name
                path.parent.mkdir(parents=True, exist_ok=True)
                path.write_text(text, encoding="utf-8")
            return release_hygiene.scan_tree(root)

    def test_blocks_private_key_header(self):
        result = self.scan({"bad.txt": "-----BEGIN OPENSSH PRIVATE KEY-----\n"})
        self.assertTrue(result.blockers)

    def test_blocks_obvious_openai_and_github_tokens(self):
        result = self.scan({
            "a.txt": "sk-" + "A" * 40,
            "b.txt": "ghp_" + "B" * 40,
        })
        self.assertGreaterEqual(len(result.blockers), 2)

    def test_placeholders_do_not_block(self):
        result = self.scan({
            "docs.txt": "OPENAI_API_KEY=<your-key-here>\nTOKEN=example-token\nPASSWORD=changeme\n"
        })
        self.assertEqual(result.blockers, [])

    def test_paths_and_private_ips_are_review_only(self):
        result = self.scan({
            "notes.txt": r"C:\Users\alice\repo" + "\n" + r"D:\VERA\work" + "\n192.168.1.50\n10.0.0.8\n"
        })
        self.assertEqual(result.blockers, [])
        self.assertGreaterEqual(len(result.review), 4)


if __name__ == "__main__":
    unittest.main()
