// Source-only dry-run of a proposed immutable Edge feed pointer. This file
// neither changes SOURCE_COMMIT nor invokes GitHub, Supabase or DSM.
const assert = require("node:assert/strict");
const crypto = require("node:crypto");
const fs = require("node:fs");
const path = require("node:path");
const vm = require("node:vm");
const { stripTypeScriptTypes } = require("node:module");

const ROOT = __dirname;
const CURRENT = "427db77274bc877b2b7d04c4970210bcda237f9e";
const CANDIDATE = "bda4500c6627cdd70734ecc620140e4298873ce6";
const EDGE_SOURCE_BLOB = "ab335e4b14f612d767a11a80b48aa9a24d317f0d";
// Exact immutable GitHub tree evidence, read only on 2026-11-03. SHA values
// are Git blob SHA-1 IDs, NOT independently checked artifact MD5/SHA-256.
const trees = {
  [CURRENT]: {
    manifest: ["319b19e34fffb0b8730df3b221bd9129b3f8cde4", 293],
    icons: {
      "PACKAGE_ICON.PNG": ["2e3e018965905fc25d977da4badf130b69129958", 9221],
      "PACKAGE_ICON_256.PNG": ["77d6bfea7fe0ee3e4b05508ca720fa3284080eb4", 86931],
    },
    artifacts: {
      "Tattler-armada38x-0.1.0-0007.spk": ["5e2b86f4ce1e67ab95eda3ec5e01e165fdcdd4af", 2273280],
    },
  },
  [CANDIDATE]: {
    manifest: ["26250ccca14e8b3660c0d0891df74007a4c2274c", 821],
    icons: {
      "PACKAGE_ICON.PNG": ["2e3e018965905fc25d977da4badf130b69129958", 9221],
      "PACKAGE_ICON_256.PNG": ["77d6bfea7fe0ee3e4b05508ca720fa3284080eb4", 86931],
    },
    artifacts: {
      "Tattler-armv7-0.2.0-0004.spk": ["d57c26755839af36cbfa3dcaf3c704f2cff70009", 3092480],
      "Tattler-armv8-0.2.0-0004.spk": ["f4953ee96b254e8ed7705542db901216c1bf5b61", 2877440],
      "Tattler-x86_64-0.2.0-0004.spk": ["8e4fe0ba3edd5c8e285778a3ad1960093c614898", 3184640],
    },
  },
};

function gitBlob(bytes) {
  return crypto.createHash("sha1")
    .update(Buffer.from(`blob ${bytes.length}\0`)).update(bytes).digest("hex");
}

function checkManifest(bytes, tree, expectedArches, label) {
  assert.equal(gitBlob(bytes), tree.manifest[0], `${label}: immutable manifest blob`);
  assert.equal(bytes.length, tree.manifest[1], `${label}: manifest length`);
  const m = JSON.parse(bytes.toString("utf8"));
  assert.equal(m.package, "Tattler");
  assert.match(m.version, /^\d+\.\d+\.\d+-\d+$/);
  assert(Number.isSafeInteger(m.min_build) && m.min_build > 0);
  const entries = Array.isArray(m.releases) ? m.releases : [m];
  assert.deepEqual(entries.map(e => e.arch).sort(), [...expectedArches].sort(), `${label}: exact arch set`);
  assert.equal(entries.length, Object.keys(tree.artifacts).length, `${label}: complete package count`);
  assert.deepEqual(Object.keys(tree.icons).sort(), ["PACKAGE_ICON.PNG", "PACKAGE_ICON_256.PNG"]);
  for (const [icon, [sha, size]] of Object.entries(tree.icons)) {
    assert.match(icon, /^PACKAGE_ICON(?:_256)?\.PNG$/);
    assert.match(sha, /^[a-f0-9]{40}$/);
    assert(size > 0);
  }
  for (const e of entries) {
    assert.equal(e.filename, `Tattler-${e.arch}-${m.version}.spk`, `${label}: filename identity`);
    assert.match(e.md5, /^[0-9a-f]{32}$/);
    assert.match(e.sha256, /^[0-9a-f]{64}$/);
    const file = tree.artifacts[e.filename];
    assert(file, `${label}: missing immutable tree package ${e.filename}`);
    assert.equal(file[1], e.size, `${label}: package size mismatch`);
    assert.match(file[0], /^[0-9a-f]{40}$/);
  }
  return m;
}

async function edgeAtPointer(indexSource, commit, manifest) {
  assert.match(commit, /^[0-9a-f]{40}$/);
  const original = `const SOURCE_COMMIT = "${CURRENT}";`;
  assert.equal(indexSource.split(original).length, 2, "exact unmodified source pointer required");
  const source = indexSource.replace(original, `const SOURCE_COMMIT = "${commit}";`);
  let handler;
  let fetchCount = 0;
  vm.runInNewContext(stripTypeScriptTypes(source), {
    Deno: { serve(h) { handler = h; } },
    fetch: async url => {
      fetchCount++;
      assert.equal(url, `https://raw.githubusercontent.com/thebrazenbeard/tattler/${commit}/package-source/release.json`);
      return { ok: true, json: async () => manifest };
    },
    URL, URLSearchParams, Request, Response, TextDecoder, Uint8Array,
  });
  assert.equal(typeof handler, "function");
  return {
    invoke: async (arch, build, method = "GET") => {
      const url = `https://tattler.example/functions/v1/tattler-package-source`;
      const req = method === "GET"
        ? new Request(`${url}?arch=${encodeURIComponent(arch)}&build=${build}`)
        : new Request(url, { method: "POST", headers: { "content-type": "application/x-www-form-urlencoded" }, body: `arch=${encodeURIComponent(arch)}&build=${build}` });
      const response = await handler(req);
      return { status: response.status, data: await response.json() };
    },
    fetchCount: () => fetchCount,
  };
}

async function checkCatalog(indexSource, commit, manifest, aliases, expectedLegacy) {
  const edge = await edgeAtPointer(indexSource, commit, manifest);
  const base = `https://raw.githubusercontent.com/thebrazenbeard/tattler/${commit}/package-source/public`;
  const artifactFor = target => Array.isArray(manifest.releases)
    ? manifest.releases.find(e => e.arch === target)
    : manifest.arch === target ? manifest : undefined;
  const cases = expectedLegacy
    ? [["armada38x", "armada38x"]]
    : Object.entries(aliases).flatMap(([target, list]) => [...list].map(alias => [alias, target]));
  for (const [arch, target] of cases) {
    for (const method of ["GET", "POST"]) {
      const { status, data } = await edge.invoke(arch, manifest.min_build, method);
      assert.equal(status, 200, `${commit} ${method} ${arch}`);
      assert.equal(data.packages.length, 1, `${commit} ${method} ${arch}`);
      const pkg = data.packages[0];
      const artifact = artifactFor(target);
      assert.equal(pkg.link, `${base}/releases/${artifact.filename}`);
      assert.equal(pkg.version, manifest.version);
      assert.equal(pkg.md5, artifact.md5);
      assert.equal(pkg.size, artifact.size);
      assert.deepEqual(Array.from(pkg.thumbnail), [`${base}/icons/PACKAGE_ICON.PNG`]);
      assert.deepEqual(Array.from(pkg.thumbnail_retina), [`${base}/icons/PACKAGE_ICON_256.PNG`, `${base}/icons/PACKAGE_ICON_256.PNG`]);
    }
  }
  for (const [arch, build] of [["unknown", manifest.min_build], [cases[0][0], manifest.min_build - 1]]) {
    const { status, data } = await edge.invoke(arch, build);
    assert.equal(status, 200);
    assert.equal(data.packages.length, 0);
  }
  if (expectedLegacy) {
    const { data } = await edge.invoke("rtd1296", manifest.min_build);
    assert.equal(data.packages.length, 0, "v0.1 legacy must remain single-armada38x");
  }
  assert.equal(edge.fetchCount(), 1, "immutable release should be cached per isolate");
  return cases.length;
}

function checkArtifactBytes(manifest, tree, binaryDirectory) {
  const releases = manifest.releases || [manifest];
  if (!binaryDirectory) return 0;
  for (const item of releases) {
    const bytes = fs.readFileSync(path.join(binaryDirectory, item.filename));
    assert.equal(bytes.length, item.size, `binary size ${item.filename}`);
    assert.equal(gitBlob(bytes), tree.artifacts[item.filename][0], `Git blob ${item.filename}`);
    assert.equal(crypto.createHash("md5").update(bytes).digest("hex"), item.md5, `MD5 ${item.filename}`);
    assert.equal(crypto.createHash("sha256").update(bytes).digest("hex"), item.sha256, `SHA256 ${item.filename}`);
  }
  return releases.length;
}

(async () => {
  const indexBytes = fs.readFileSync(path.join(ROOT, "supabase/index.ts"));
  assert.equal(gitBlob(indexBytes), EDGE_SOURCE_BLOB, "PR #21 exact source blob drift");
  const oldBytes = fs.readFileSync(path.join(ROOT, "fixtures_legacy_release.json"));
  const candidateBytes = fs.readFileSync(path.join(ROOT, "release.json"));
  const oldRelease = checkManifest(oldBytes, trees[CURRENT], ["armada38x"], "v0.1 pinned");
  const candidate = checkManifest(candidateBytes, trees[CANDIDATE], ["armv7", "armv8", "x86_64"], "v0.2 candidate");
  const { ARCH_ALIASES } = require("./api/catalog.js");
  const source = indexBytes.toString("utf8");
  const legacyCases = await checkCatalog(source, CURRENT, oldRelease, ARCH_ALIASES, true);
  const candidateCases = await checkCatalog(source, CANDIDATE, candidate, ARCH_ALIASES, false);
  // Challenge manifest mutation, artifact omission/size drift, and missing icons.
  const mutated = Buffer.from(candidateBytes);
  mutated[mutated.length - 2] = mutated[mutated.length - 2] === 32 ? 33 : 32;
  assert.throws(() => checkManifest(mutated, trees[CANDIDATE], ["armv7", "armv8", "x86_64"], "tampered"));
  const missingPackage = structuredClone(trees[CANDIDATE]);
  delete missingPackage.artifacts["Tattler-armv7-0.2.0-0004.spk"];
  assert.throws(() => checkManifest(candidateBytes, missingPackage, ["armv7", "armv8", "x86_64"], "missing"));
  const wrongSize = structuredClone(trees[CANDIDATE]);
  wrongSize.artifacts["Tattler-armv7-0.2.0-0004.spk"][1]++;
  assert.throws(() => checkManifest(candidateBytes, wrongSize, ["armv7", "armv8", "x86_64"], "wrong size"));
  const missingIcon = structuredClone(trees[CANDIDATE]);
  delete missingIcon.icons["PACKAGE_ICON_256.PNG"];
  assert.throws(() => checkManifest(candidateBytes, missingIcon, ["armv7", "armv8", "x86_64"], "missing icon"));

  const dir = process.env.TATTLER_PACKAGE_BINARY_DIR;
  const binaryCount = checkArtifactBytes(candidate, trees[CANDIDATE], dir);
  console.log(`SOURCE_CONTRACT_PASS: pinned=${CURRENT} candidate=${CANDIDATE} Git-tree packages=1+3 DSM aliases=${candidateCases} old aliases=${legacyCases}`);
  console.log(`ARTIFACT_DIGEST_${binaryCount ? "PASS" : "UNVERIFIED"}: ${binaryCount}/3 candidate .spk bytes checked; immutable tree metadata is not binary digest proof`);
  console.log("EDGE_DEPLOY_AND_DSM_INSTALL_UNVERIFIED; SOURCE_COMMIT remains unchanged");
})().catch(e => { console.error(e); process.exitCode = 1; });
