import re
import unittest
from pathlib import Path

ROOT = Path(__file__).resolve().parents[1]
SPK = ROOT / "spk"


class SPKLifecycleTests(unittest.TestCase):
    def test_package_revision_and_description_are_current(self):
        info = (SPK / "INFO").read_text(encoding="utf-8")
        self.assertIn('version="0.2.0-0003"', info)
        self.assertIn("sampled network activity", info)
        self.assertNotIn("passive connection observability", info)

    def test_service_verifies_pid_ownership_before_kill(self):
        text = (SPK / "scripts" / "start-stop-status").read_text(encoding="utf-8")
        self.assertIn("/proc/$pid/exe", text)
        self.assertIn("readlink -f", text)
        self.assertIn("BIN_REAL", text)
        self.assertRegex(text, r'\[ "\$exe" = "\$BIN_REAL" \]')

    def test_service_waits_for_loopback_listener(self):
        text = (SPK / "scripts" / "start-stop-status").read_text(encoding="utf-8")
        self.assertIn("0100007F:23BB", text)
        self.assertIn("/proc/net/tcp", text)
        self.assertIn("health_ready", text)
        self.assertIn("--listen 127.0.0.1:9147", text)

    def test_service_rotates_log_and_writes_pid_atomically(self):
        text = (SPK / "scripts" / "start-stop-status").read_text(encoding="utf-8")
        self.assertRegex(text, r"MAX_LOG_BYTES=[1-9][0-9]+")
        self.assertIn("rotate_log", text)
        self.assertIn('PIDTMP="' + '$' + '{PIDFILE}.tmp.$$"', text)
        self.assertIn('mv -f "$PIDTMP" "$PIDFILE"', text)

    def test_install_and_upgrade_lock_state_directory(self):
        for name in ("postinst", "postupgrade"):
            text = (SPK / "scripts" / name).read_text(encoding="utf-8")
            self.assertIn("SYNOPKG_PKGVAR", text, name)
            self.assertIn('mkdir -p "$STATE_DIR"', text, name)
            self.assertIn('chmod 700 "$STATE_DIR"', text, name)
            self.assertIn("umask 077", text, name)

    def test_spk_scripts_do_not_request_privilege_escalation(self):
        combined = "\n".join(
            path.read_text(encoding="utf-8")
            for path in sorted((SPK / "scripts").iterdir())
            if path.is_file()
        ).lower()
        for forbidden in ("sudo ", "synosystemctl", "setcap ", "chmod u+s", "run-as=root"):
            self.assertNotIn(forbidden, combined)


if __name__ == "__main__":
    unittest.main()
