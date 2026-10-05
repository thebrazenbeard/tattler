const SOURCE_COMMIT = "427db77274bc877b2b7d04c4970210bcda237f9e";
const rawBase =
  `https://raw.githubusercontent.com/thebrazenbeard/tattler/${SOURCE_COMMIT}/package-source/public`;
const releaseUrl =
  `https://raw.githubusercontent.com/thebrazenbeard/tattler/${SOURCE_COMMIT}/package-source/release.json`;

type Release = {
  arch: string;
  filename: string;
  md5: string;
  min_build: number;
  package: string;
  sha256: string;
  size: number;
  version: string;
};

let releaseCache: Release | null = null;

async function loadRelease(): Promise<Release> {
  if (releaseCache) return releaseCache;
  const response = await fetch(releaseUrl, {
    headers: { "User-Agent": "Tattler-Package-Source/1" },
  });
  if (!response.ok) {
    throw new Error(`release manifest fetch failed: ${response.status}`);
  }
  const value = await response.json() as Release;
  if (value.package !== "Tattler" || value.arch !== "armada38x") {
    throw new Error("release manifest identity mismatch");
  }
  releaseCache = value;
  return value;
}

async function requestParams(req: Request): Promise<URLSearchParams> {
  const url = new URL(req.url);
  const params = new URLSearchParams(url.search);
  if (req.method === "POST") {
    const type = req.headers.get("content-type") || "";
    if (type.includes("application/x-www-form-urlencoded")) {
      const body = new URLSearchParams(await req.text());
      for (const [key, value] of body) params.set(key, value);
    } else if (type.includes("multipart/form-data")) {
      const form = await req.formData();
      for (const [key, value] of form) {
        if (typeof value === "string") params.set(key, value);
      }
    }
  }
  return params;
}

function catalog(params: URLSearchParams, release: Release) {
  const arch = params.get("arch") || "";
  const build = Number.parseInt(params.get("build") || "0", 10) || 0;
  const packages = [];

  if (arch === release.arch && build >= release.min_build) {
    const icon64 = `${rawBase}/icons/PACKAGE_ICON.PNG`;
    const icon256 = `${rawBase}/icons/PACKAGE_ICON_256.PNG`;
    packages.push({
      package: "Tattler",
      version: release.version,
      dname: "Tattler",
      desc: "Low-overhead host diagnostics and passive connection observability for Synology DSM.",
      link: `${rawBase}/releases/${release.filename}`,
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
      type: 0,
    });
  }

  return { packages };
}

Deno.serve(async (req: Request) => {
  if (req.method !== "GET" && req.method !== "POST") {
    return new Response("method not allowed", {
      status: 405,
      headers: { Allow: "GET, POST" },
    });
  }

  try {
    const [params, release] = await Promise.all([
      requestParams(req),
      loadRelease(),
    ]);
    return new Response(JSON.stringify(catalog(params, release)), {
      status: 200,
      headers: {
        "Content-Type": "application/json; charset=utf-8",
        "Cache-Control": "public, max-age=300",
      },
    });
  } catch {
    return new Response(
      JSON.stringify({ packages: [], error: "catalog_request_failed" }),
      {
        status: 502,
        headers: { "Content-Type": "application/json; charset=utf-8" },
      },
    );
  }
});
