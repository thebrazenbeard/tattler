const release = require("../release.json");

function paramsFrom(req) {
  const out = Object.assign({}, req.query || {});
  if (req.body && typeof req.body === "object") {
    Object.assign(out, req.body);
  } else if (typeof req.body === "string") {
    for (const [key, value] of new URLSearchParams(req.body)) out[key] = value;
  }
  return out;
}

module.exports = function handler(req, res) {
  const params = paramsFrom(req);
  const arch = String(params.arch || "");
  const build = Number.parseInt(String(params.build || "0"), 10) || 0;
  const host = req.headers["x-forwarded-host"] || req.headers.host;
  const proto = req.headers["x-forwarded-proto"] || "https";
  const base = host ? `${proto}://${host}` : "";

  const packages = [];
  if (arch === release.arch && build >= release.min_build) {
    const icon64 = `${base}/icons/PACKAGE_ICON.PNG`;
    const icon256 = `${base}/icons/PACKAGE_ICON_256.PNG`;
    packages.push({
      package: "Tattler",
      version: release.version,
      dname: "Tattler",
      desc: "Low-overhead host diagnostics and passive connection observability for Synology DSM.",
      link: `${base}/releases/${release.filename}`,
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
      md5: release.md5,
      size: release.size,
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
