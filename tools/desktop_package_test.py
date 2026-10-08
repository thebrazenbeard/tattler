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
        self.assertIn("oldestSeconds(b)-oldestSeconds(a)", text)



class ProtocolEvidenceUITests(unittest.TestCase):
    def test_desktop_shows_transport_protocol_evidence_and_semantic_activity(self):
        text = (ROOT / "desktop" / "frontend" / "dist" / "index.html").read_text(encoding="utf-8")
        for token in ("Network activity", "Protocol evidence", "Semantic activity", "protocol_evidence", "semanticDetail", "cache.semantic"):
            self.assertIn(token, text)


class SemanticDesktopLayoutTests(unittest.TestCase):
    def test_four_independent_disclosures_and_cached_filters(self):
        text=(ROOT/"desktop"/"frontend"/"dist"/"index.html").read_text(encoding="utf-8")
        for tag in ("findings","network","semantic","events"):
            self.assertIn(f'<details id="{tag}-panel"',text)
            self.assertIn(f"tattler.panel.",text)
        for token in ('id="view-filter"','id="process-filter"','id="search-filter"','id="sort-filter"',
                      'data-firstseen', 'function filteredRows()', 'function groupRows(', 'function networkFingerprint(',
                      'matching tracking keys do not establish distinct socket instances'):
            self.assertIn(token,text)
        for cadence in ('setInterval(pollFast,2000)','setInterval(pollFindings,5000)',
                        'setInterval(pollEvents,10000)','setInterval(pollSemantic,10000)'):
            self.assertIn(cadence,text)
        self.assertIn("App.Network()",text)
        self.assertNotIn("setInterval(refresh,2000)",text)

    def test_network_filter_language_preserves_evidence_boundaries(self):
        text=(ROOT/"desktop"/"frontend"/"dist"/"index.html").read_text(encoding="utf-8")
        for value in ("Remote peers (non-loopback)","Loopback-bound","Unknown owner",
                      "Counted rows are not proven distinct sockets",
                      "A missing report is not proof of no traffic"):
            self.assertIn(value,text)

if __name__ == "__main__":
    unittest.main()
