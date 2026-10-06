const release = require("../release.json");

const ARCH_ALIASES = {
  x86_64: new Set([
    "x86_64",
    "apollolake", "avoton", "braswell", "broadwell", "broadwellnk",
    "bromolow", "denverton", "geminilake", "grantley", "purley",
    "v1000", "r1000", "r1600", "epyc7002", "kvmx64", "dockerx64"
  ]),
  armv7: new Set([
    "armv7",
    "armada370", "armada375", "armada38x", "armadaxp",
    "alpine", "alpine4k", "monaco", "comcerto2k", "hi3535"
  ]),
  armv8: new Set([
    "armv8", "aarch64",
    "armada37xx", "rtd1296", "rtd1619", "rtd1619b"
  ])
};

function paramsFrom(req) {
  const out = Object.assign({}, req.query || {});
  if (req.body && typeof req.body === "object") {
    Object.assign(out, req.body);
  } else if (typeof req.body === "string") {
    for (const [key, value] of new URLSearchParams(req.body)) out[key] = value;
  }
  return out;
}

function genericArch(requested) {
  const value = String(requested || "").trim().toLowerCase();
  for (const [arch, aliases] of Object.entries(ARCH_ALIASES)) {
    if (aliases.has(value)) return arch;
  }
  return "";
}

module.exports = function handler(req, res) {
  const params = paramsFrom(req);
  const requestedArch = String(params.arch || "");
  const arch = genericArch(requestedArch);
  const build = Number.parseInt(String(params.build || "0"), 10) || 0;
  const host = req.headers["x-forwarded-host"] || req.headers.host;
  const proto = req.headers["x-forwarded-proto"] || "https";
  const base = host ? `${proto}://${host}` : "";

  const packages = [];
  const artifact = Array.isArray(release.releases)
    ? release.releases.find(item => item.arch === arch)
    : null;

  if (artifact && build >= release.min_build) {
    const icon64 = `${base}/icons/PACKAGE_ICON.PNG`;
    const icon256 = `${base}/icons/PACKAGE_ICON_256.PNG`;
    packages.push({
      package: release.package,
      version: release.version,
      dname: "Tattler",
      desc: "Low-overhead host diagnostics and sampled network activity for Synology DSM.",
      link: `${base}/releases/${artifact.filename}`,
      thumbnail: [icon64],
      thumbnail_retina: [icon256, icon256],
      snapshot: [],
      qinst: true,
      qupgrade: true,
      qstart: true,
      deppkgs: null,
      conflictpkgs: null,
      download_count: 0,
      recent_download_count: 0,
      md5: artifact.md5,
      size: artifact.size,
      maintainer: "thebrazenbeard",
      maintainer_url: "https://github.com/thebrazenbeard/tattler",
      distributor: "thebrazenbeard",
      distributor_url: "https://github.com/thebrazenbeard/tattler",
      type: 0
    });
  }

  res.setHeader("Content-Type", "application/json; charset=utf-8");
  res.setHeader("Cache-Control", "public, max-age=300");
  res.status(200).json({ packages });
};

module.exports.genericArch = genericArch;
module.exports.ARCH_ALIASES = ARCH_ALIASES;
