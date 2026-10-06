import json
import struct
import tarfile
import tempfile
import unittest
from pathlib import Path

from tools import build_spk, update_package_source, verify_spk

ROOT = Path(__file__).resolve().parents[1]
SPK = ROOT / "spk"


def fake_elf(machine: int) -> bytes:
    data = bytearray(64)
    data[:4] = b"\x7fELF"
    data[4] = 2
    data[5] = 1
    data[6] = 1
    data[18:20] = struct.pack("<H", machine)
    return bytes(data)


class MultiArchSPKTests(unittest.TestCase):
    def test_supported_generic_architectures_have_expected_elf_machine(self):
        self.assertEqual(
            build_spk.ELF_MACHINE_BY_ARCH,
            {"x86_64": 62, "armv7": 40, "armv8": 183},
        )

    def test_builder_stamps_each_target_architecture(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            for arch, machine in build_spk.ELF_MACHINE_BY_ARCH.items():
                binary = root / f"tattler-{arch}"
                output = root / f"{arch}.spk"
                binary.write_bytes(fake_elf(machine))

                build_spk.build(binary, output, arch)
                with tarfile.open(output, "r:") as tf:
                    info = tf.extractfile("INFO").read().decode("utf-8")

                self.assertIn(f'arch="{arch}"', info)
                result = verify_spk.verify(output)
                self.assertEqual(result["arch"], arch)
                self.assertEqual(result["elf_e_machine"], machine)

    def test_builder_rejects_arch_binary_mismatch(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            binary = root / "wrong"
            output = root / "wrong.spk"
            binary.write_bytes(fake_elf(40))
            with self.assertRaises(ValueError):
                build_spk.build(binary, output, "x86_64")

    def test_package_source_stages_three_architecture_artifacts(self):
        with tempfile.TemporaryDirectory() as td:
            root = Path(td)
            built = []
            for arch, machine in build_spk.ELF_MACHINE_BY_ARCH.items():
                binary = root / f"tattler-{arch}"
                output = root / f"{arch}.spk"
                binary.write_bytes(fake_elf(machine))
                build_spk.build(binary, output, arch)
                built.append(output)

            package_source = root / "package-source"
            manifest = update_package_source.stage_many(built, package_source, SPK)

            self.assertEqual(manifest["version"], "0.2.0-0004")
            releases = {item["arch"]: item for item in manifest["releases"]}
            self.assertEqual(set(releases), {"x86_64", "armv7", "armv8"})
            for arch in releases:
                filename = releases[arch]["filename"]
                self.assertEqual(filename, f"Tattler-{arch}-0.2.0-0004.spk")
                self.assertTrue((package_source / "public" / "releases" / filename).is_file())

            saved = json.loads((package_source / "release.json").read_text(encoding="utf-8"))
            self.assertEqual(saved, manifest)


if __name__ == "__main__":
    unittest.main()
