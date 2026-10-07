import re
import subprocess
import tempfile
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


def extract_script(text: str) -> str:
    match = re.search(r"<script>(.*?)</script>", text, re.S)
    if not match:
        raise AssertionError("script block not found")
    return match.group(1)


class UIJavaScriptTests(unittest.TestCase):
    def check_script(self, text: str):
        script = extract_script(text)
        with tempfile.TemporaryDirectory() as td:
            path = Path(td) / "ui.js"
            path.write_text(script, encoding="utf-8")
            result = subprocess.run(
                ["node", "--check", str(path)],
                capture_output=True,
                text=True,
                check=False,
            )
        self.assertEqual(result.returncode, 0, result.stderr or result.stdout)

    def test_desktop_javascript_syntax(self):
        self.check_script(
            (ROOT / "desktop" / "frontend" / "dist" / "index.html").read_text(encoding="utf-8")
        )

    def test_embedded_browser_javascript_syntax(self):
        self.check_script(
            (ROOT / "internal" / "server" / "server.go").read_text(encoding="utf-8")
        )


if __name__ == "__main__":
    unittest.main()
