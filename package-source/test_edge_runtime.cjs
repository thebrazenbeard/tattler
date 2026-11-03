// Offline Web API shim for the deployed Deno handler. Does not certify the
// Supabase Edge Runtime; deployment and NAS readback remain separate gates.
const assert = require("node:assert/strict");
const fs = require("node:fs");
const vm = require("node:vm");
const { stripTypeScriptTypes } = require("node:module");

const source = fs.readFileSync(`${__dirname}/supabase/index.ts`, "utf8");
const release = {
  arch: "armada38x",
  filename: "Tattler-armada38x-0.1.0-0007.spk",
  md5: "d22eae80bd6b8226aaa168bdf403910a",
  min_build: 72806,
  package: "Tattler",
  sha256: "a70428e2d9a0f132e5eb3b12d7c7c3608b12601200c023c51354083ea4b8b736",
  size: 2273280,
  version: "0.1.0-0007",
};
let handler;
let fetches = 0;
vm.runInNewContext(stripTypeScriptTypes(source), {
  Deno: { serve(h) { handler = h; } },
  fetch: async () => { fetches++; return { ok: true, json: async () => release }; },
  URL, URLSearchParams, Request, Response, TextDecoder, Uint8Array,
});

const url = "https://tattler.example/functions/v1/tattler-package-source";
const formType = "application/x-www-form-urlencoded";
const post = (body, headers = {}) => new Request(url, {
  method: "POST", headers: { "content-type": formType, ...headers }, body,
});

(async () => {
  let response = await handler(post("x".repeat(8193)));
  assert.equal(response.status, 413);
  assert.equal((await response.json()).error, "request_too_large");
  assert.equal(fetches, 0, "invalid body should not fetch release manifest");

  response = await handler(post("tiny", { "content-length": "9000" }));
  assert.equal(response.status, 413);
  assert.equal(fetches, 0);

  response = await handler(post("arch=armada38x&build=72806&language=enu"));
  assert.equal(response.status, 200);
  let body = await response.json();
  assert.equal(body.packages.length, 1);
  assert.equal(body.packages[0].version, release.version);
  assert.equal(fetches, 1);

  const form = new FormData();
  form.set("arch", "armada38x");
  form.set("build", "72806");
  response = await handler(new Request(url, { method: "POST", body: form }));
  assert.equal(response.status, 200);
  body = await response.json();
  assert.equal(body.packages.length, 1);

  response = await handler(new Request(`${url}?arch=armada38x&build=72805`));
  assert.equal(response.status, 200);
  assert.equal((await response.json()).packages.length, 0);

  response = await handler(new Request(url, { method: "PUT" }));
  assert.equal(response.status, 405);

  const stream = new ReadableStream({
    start(controller) { controller.enqueue(new Uint8Array(9000)); controller.close(); },
  });
  response = await handler(new Request(url, {
    method: "POST", headers: { "content-type": formType }, body: stream, duplex: "half",
  }));
  assert.equal(response.status, 413, "unknown length must be capped while streaming");

  // A source-pointer advance to the current multi-architecture release must
  // preserve DSM's existing armada38x behavior and recognize other aliases.
  // Each Deno isolate owns its own immutable-release cache.
  const multiarchRelease = require("./release.json");
  let multiarchHandler;
  vm.runInNewContext(stripTypeScriptTypes(source), {
    Deno: { serve(h) { multiarchHandler = h; } },
    fetch: async () => ({ ok: true, json: async () => multiarchRelease }),
    URL, URLSearchParams, Request, Response, TextDecoder, Uint8Array,
  });
  for (const [requested, expected] of [
    ["armada38x", "armv7"],
    ["rtd1296", "armv8"],
    ["geminilake", "x86_64"],
  ]) {
    response = await multiarchHandler(new Request(
      `${url}?arch=${requested}&build=${multiarchRelease.min_build}`,
    ));
    assert.equal(response.status, 200, `current multiarch ${requested}`);
    const result = await response.json();
    assert.equal(result.packages.length, 1, `current multiarch ${requested}`);
    const artifact = multiarchRelease.releases.find(item => item.arch === expected);
    assert.equal(result.packages[0].link.endsWith(`/${artifact.filename}`), true);
    assert.equal(result.packages[0].md5, artifact.md5);
    assert.equal(result.packages[0].size, artifact.size);
  }

  // The legacy Node catalog and the Edge adapter must not silently disagree
  // about which DSM hardware labels consume each published architecture.
  const { ARCH_ALIASES } = require("./api/catalog.js");
  for (const [expected, aliases] of Object.entries(ARCH_ALIASES)) {
    const artifact = multiarchRelease.releases.find(item => item.arch === expected);
    for (const requested of aliases) {
      response = await multiarchHandler(new Request(
        `${url}?arch=${requested}&build=${multiarchRelease.min_build}`,
      ));
      const result = await response.json();
      assert.equal(result.packages.length, 1, `DSM alias ${requested}`);
      assert.equal(result.packages[0].link.endsWith(`/${artifact.filename}`), true);
    }
  }

  response = await multiarchHandler(new Request(`${url}?arch=unknown&build=99999`));
  assert.equal((await response.json()).packages.length, 0);
  response = await multiarchHandler(new Request(`${url}?arch=armada38x&build=72805`));
  assert.equal((await response.json()).packages.length, 0);

  console.log("Tattler Edge bounded-body + multiarch + alias parity shim: PASS");
})().catch(e => { console.error(e); process.exitCode = 1; });
