#!/usr/bin/env python3
r"""Inspect the authenticated DSM Package Center window through Windows UI Automation.

This is intentionally an inspection/navigation helper, not a blind click script.
It locates the DSM Firefox window by the exact address-bar value, can dismiss the
known stale v0004 root-privilege rejection dialog, opens Package Center Settings
by accessible name, and prints the resulting accessibility controls.

Runtime dependency (kept outside the repo in current qualification):
    python -m pip install --target %LOCALAPPDATA%\Temp\bt2-pywinauto pywinauto
    set PYTHONPATH=%LOCALAPPDATA%\Temp\bt2-pywinauto
"""

from __future__ import annotations

import argparse
import sys
import time


DSM_ADDRESS = "192.168.1.187:5001"
STALE_ROOT_ERROR = "Unable to install Tattler because it runs with root privileges"


def get_desktop():
    try:
        from pywinauto import Desktop
    except Exception as exc:  # pragma: no cover - workstation-only dependency
        raise SystemExit(
            "pywinauto is required at runtime; install it into an isolated temp path"
        ) from exc
    return Desktop(backend="uia")


def find_dsm_window():
    desktop = get_desktop()
    matches = []
    for window in desktop.windows():
        try:
            if "Mozilla Firefox" not in window.window_text():
                continue
            for control in window.descendants(control_type="ComboBox"):
                if (
                    control.element_info.automation_id == "urlbar-input"
                    and control.window_text().strip() == DSM_ADDRESS
                ):
                    matches.append(window)
                    break
        except Exception:
            continue
    if len(matches) != 1:
        raise RuntimeError(f"expected exactly one DSM Firefox window, found {len(matches)}")
    return matches[0]


def dismiss_known_stale_dialog(window) -> bool:
    for pane in window.descendants(control_type="Pane"):
        try:
            if STALE_ROOT_ERROR not in pane.window_text():
                continue
            buttons = [
                b for b in pane.descendants(control_type="Button")
                if b.window_text() == "OK"
            ]
            if len(buttons) != 1:
                raise RuntimeError("stale root-dialog found but OK button is ambiguous")
            buttons[0].invoke()
            time.sleep(0.5)
            return True
        except Exception:
            continue
    return False


def open_settings(window) -> None:
    settings = [
        b for b in window.descendants(control_type="Button")
        if b.window_text() == "Settings"
    ]
    if len(settings) != 1:
        raise RuntimeError(f"expected one Package Center Settings button, found {len(settings)}")
    settings[0].invoke()
    time.sleep(1.5)


def dump_controls(window) -> None:
    interesting = {
        "Edit",
        "Button",
        "TabItem",
        "List",
        "ListItem",
        "TreeItem",
        "DataItem",
        "Pane",
    }
    for i, control in enumerate(window.descendants()):
        try:
            info = control.element_info
            name = control.window_text()
            if not name and info.control_type not in interesting:
                continue
            print(
                f"{i:04d} | {info.control_type} | {name!r} | "
                f"aid={info.automation_id!r} | cls={info.class_name!r}"
            )
        except Exception:
            continue


def main() -> int:
    parser = argparse.ArgumentParser()
    parser.add_argument(
        "--open-settings",
        action="store_true",
        help="dismiss only the known stale v0004 error dialog and open Package Center Settings",
    )
    args = parser.parse_args()

    window = find_dsm_window()
    print(f"DSM_WINDOW handle={window.handle} title={window.window_text()!r}")

    if args.open_settings:
        if dismiss_known_stale_dialog(window):
            print("DISMISSED_KNOWN_STALE_V0004_ROOT_DIALOG")
        open_settings(window)
        print("OPENED_PACKAGE_CENTER_SETTINGS")

    dump_controls(window)
    return 0


if __name__ == "__main__":
    raise SystemExit(main())
