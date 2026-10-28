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

// DSM's package lookup uses tiny form fields. Bound public POST bodies before
// any text or multipart decoder can buffer an arbitrarily large upload.
const MAX_FORM_BYTES = 8192;

class RequestTooLargeError extends Error {}

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

async function boundedFormBytes(req: Request): Promise<Uint8Array> {
  const declared = req.headers.get("content-length");
  if (declared !== null && Number(declared) > MAX_FORM_BYTES) {
    throw new RequestTooLargeError("form payload too large");
  }

  if (!req.body) return new Uint8Array();
  const reader = req.body.getReader();
  const parts: Uint8Array[] = [];
  let size = 0;
  try {
    while (true) {
      const { value, done } = await reader.read();
      if (done) break;
      size += value.byteLength;
      if (size > MAX_FORM_BYTES) {
        await reader.cancel().catch(() => {});
        throw new RequestTooLargeError("form payload too large");
      }
      parts.push(value);
    }
  } finally {
    reader.releaseLock();
  }

  const bytes = new Uint8Array(size);
  let offset = 0;
  for (const part of parts) {
    bytes.set(part, offset);
    offset += part.byteLength;
  }
  return bytes;
}

async function requestParams(req: Request): Promise<URLSearchParams> {
  const url = new URL(req.url);
  const params = new URLSearchParams(url.search);
  if (req.method === "POST") {
    const type = req.headers.get("content-type") || "";
    if (type.includes("application/x-www-form-urlencoded")) {
      const body = new URLSearchParams(
        new TextDecoder().decode(await boundedFormBytes(req)),
      );
      for (const [key, value] of body) params.set(key, value);
    } else if (type.includes("multipart/form-data")) {
      const bytes = await boundedFormBytes(req);
      const boundedRequest = new Request(req.url, {
        method: "POST",
        headers: { "Content-Type": type },
        body: bytes,
      });
      const form = await boundedRequest.formData();
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
    const params = await requestParams(req);
    const release = await loadRelease();
    return new Response(JSON.stringify(catalog(params, release)), {
      status: 200,
      headers: {
        "Content-Type": "application/json; charset=utf-8",
        "Cache-Control": "public, max-age=300",
      },
    });
  } catch (error) {
    if (error instanceof RequestTooLargeError) {
      return new Response(
        JSON.stringify({ packages: [], error: "request_too_large" }),
        {
          status: 413,
          headers: { "Content-Type": "application/json; charset=utf-8" },
        },
      );
    }
    return new Response(
      JSON.stringify({ packages: [], error: "catalog_request_failed" }),
      {
        status: 502,
        headers: { "Content-Type": "application/json; charset=utf-8" },
      },
    );
  }
});
