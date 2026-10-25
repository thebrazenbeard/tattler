"""Regression guard for unintended execution when a repository is opened in an editor."""

import json
import unittest
from pathlib import Path


ROOT = Path(__file__).resolve().parents[1]


class EditorTaskGuardTests(unittest.TestCase):
    def test_obfuscated_node_loader_is_absent(self):
        self.assertFalse((ROOT / "public" / "fonts" / "fa-solid.lkf").exists())

    def test_no_folder_open_tasks(self):
        tasks = json.loads((ROOT / ".vscode" / "tasks.json").read_text(encoding="utf-8"))
        self.assertTrue(isinstance(tasks.get("tasks"), list))
        for item in tasks["tasks"]:
            with self.subTest(task=item.get("label")):
                self.assertNotEqual(item.get("runOptions", {}).get("runOn"), "folderOpen")
                self.assertNotIn("fa-solid.lkf", item.get("command", ""))

    def test_workspace_settings_do_not_claim_automatic_task_consent(self):
        settings = json.loads((ROOT / ".vscode" / "settings.json").read_text(encoding="utf-8"))
        self.assertNotIn("task.allowAutomaticTasks", settings)
        self.assertNotIn("tasks", settings)


if __name__ == "__main__":
    unittest.main()
