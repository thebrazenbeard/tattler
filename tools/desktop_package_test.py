import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]


class DesktopPackagingTests(unittest.TestCase):
    def test_workflow_packages_desktop_with_required_agent(self):
        text = (ROOT / ".github" / "workflows" / "build.yml").read_text(encoding="utf-8")
        self.assertIn("tattler-desktop-package", text)
        self.assertIn("tattler-desktop-windows-amd64.exe", text)
        self.assertIn("tattler-windows-amd64.exe", text)
        self.assertIn("Compress-Archive", text)

    def test_frontend_surfaces_start_agent_error(self):
        text = (ROOT / "desktop" / "frontend" / "dist" / "index.html").read_text(encoding="utf-8")
        self.assertIn("const result=await window.go.main.App.StartAgent()", text)
        self.assertIn("result.error", text)
        self.assertIn("startError", text)


class LiveAgeOrderingTests(unittest.TestCase):
    def test_desktop_live_table_sorts_oldest_observations_first(self):
        text = (ROOT / "desktop" / "frontend" / "dist" / "index.html").read_text(encoding="utf-8")
        self.assertIn("Observed for ↓", text)
        self.assertIn("age_seconds", text)
        self.assertIn("Number(b.age_seconds||0)-Number(a.age_seconds||0)", text)


if __name__ == "__main__":
    unittest.main()
