const SOURCE_COMMIT = "427db77274bc877b2b7d04c4970210bcda237f9e";
const rawBase =
  `https://raw.githubusercontent.com/thebrazenbeard/tattler/${SOURCE_COMMIT}/package-source/public`;
const releaseUrl =
  `https://raw.githubusercontent.com/thebrazenbeard/tattler/${SOURCE_COMMIT}/package-source/release.json`;

type Artifact = {
  arch: string;
  filename: string;
  md5: string;
  sha256: string;
  size: number;
};

type Release = {
  min_build: number;
  package: string;
  version: string;
} & (
  | (Artifact & { releases?: never })
  | { releases: Artifact[]; arch?: never }
);

// DSM reports hardware names, while the multi-architecture manifest uses
// portable package targets. Match the existing local catalog adapter.
const ARCH_ALIASES: Record<string, string[]> = {
  x86_64: [
    "x86_64", "apollolake", "avoton", "braswell", "broadwell",
    "broadwellnk", "bromolow", "denverton", "geminilake", "grantley",
    "purley", "v1000", "r1000", "r1600", "epyc7002", "kvmx64",
    "dockerx64",
  ],
  armv7: [
    "armv7", "armada370", "armada375", "armada38x", "armadaxp",
    "alpine", "alpine4k", "monaco", "comcerto2k", "hi3535",
  ],
  armv8: [
    "armv8", "aarch64", "armada37xx", "rtd1296", "rtd1619", "rtd1619b",
  ],
};
const MULTIARCH_TARGETS = new Set(["armv7", "armv8", "x86_64"]);
const LEGACY_TARGETS = new Set(["armada38x"]);
const CHECKSUM_MD5 = /^[0-9a-f]{32}$/i;
const CHECKSUM_SHA256 = /^[0-9a-f]{64}$/i;
const SAFE_PACKAGE_FILE = /^[A-Za-z0-9._-]+\.spk$/;
const MAX_PACKAGE_BYTES = 1_000_000_000;

function genericArch(value: string): string {
  const requested = value.trim().toLowerCase();
  return Object.entries(ARCH_ALIASES)
    .find(([, aliases]) => aliases.includes(requested))?.[0] ?? "";
}

function validArtifact(value: unknown, targets: Set<string>): value is Artifact {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const item = value as Record<string, unknown>;
  return typeof item.arch === "string" &&
    targets.has(item.arch) &&
    typeof item.filename === "string" &&
    SAFE_PACKAGE_FILE.test(item.filename) &&
    item.filename.startsWith(`Tattler-${item.arch}-`) &&
    typeof item.md5 === "string" && CHECKSUM_MD5.test(item.md5) &&
    typeof item.sha256 === "string" && CHECKSUM_SHA256.test(item.sha256) &&
    typeof item.size === "number" && Number.isSafeInteger(item.size) &&
    item.size > 0 && item.size <= MAX_PACKAGE_BYTES;
}

function validRelease(value: unknown): value is Release {
  if (!value || typeof value !== "object" || Array.isArray(value)) return false;
  const release = value as Record<string, unknown>;
  if (
    release.package !== "Tattler" ||
    typeof release.version !== "string" || !release.version ||
    typeof release.min_build !== "number" ||
    !Number.isSafeInteger(release.min_build) || release.min_build < 0
  ) return false;

  if (Object.hasOwn(release, "releases")) {
    if (
      !Array.isArray(release.releases) || release.releases.length === 0 ||
      release.releases.length > MULTIARCH_TARGETS.size
    ) return false;
    const arches = new Set<string>();
    return release.releases.every((artifact: unknown) => {
      if (!validArtifact(artifact, MULTIARCH_TARGETS)) return false;
      if (arches.has(artifact.arch)) return false;
      arches.add(artifact.arch);
      return true;
    });
  }
  return validArtifact(release, LEGACY_TARGETS);
}

// DSM's package lookup uses tiny form fields. Bound public POST bodies before
// any text or multipart decoder can buffer an arbitrarily large upload.
const MAX_FORM_BYTES = 8192;
const MAX_QUERY_BYTES = 8192;

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
  const value: unknown = await response.json();
  if (!validRelease(value)) {
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
  // URL.search is serialized/percent-encoded ASCII, so length bounds the
  // encoded query bytes before URLSearchParams allocates form entries.
  if (url.search.length > MAX_QUERY_BYTES) {
    throw new RequestTooLargeError("query too large");
  }
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
  const requestedArch = params.get("arch") || "";
  const build = Number.parseInt(params.get("build") || "0", 10) || 0;
  const packages = [];

  const artifact = Array.isArray(release.releases)
    ? release.releases.find((item) => item.arch === genericArch(requestedArch))
    : "filename" in release && release.arch === requestedArch ? release : undefined;

  if (artifact && build >= release.min_build) {
    const icon64 = `${rawBase}/icons/PACKAGE_ICON.PNG`;
    const icon256 = `${rawBase}/icons/PACKAGE_ICON_256.PNG`;
    packages.push({
      package: "Tattler",
      version: release.version,
      dname: "Tattler",
      desc: "Low-overhead host diagnostics and passive connection observability for Synology DSM.",
      link: `${rawBase}/releases/${artifact.filename}`,
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
