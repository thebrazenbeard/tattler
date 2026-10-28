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

  console.log("Tattler Edge bounded-body shim: 7 checks PASS");
})().catch(e => { console.error(e); process.exitCode = 1; });
