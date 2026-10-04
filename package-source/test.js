const assert = require("assert");
const handler = require("./api/catalog");
const release = require("./release.json");

function invoke({ query = {}, body = undefined, headers = { host: "packages.test", "x-forwarded-proto": "https" } } = {}) {
  return new Promise((resolve, reject) => {
    const state = { headers: {}, status: 200, body: null };
    const res = {
      setHeader(k, v) { state.headers[k.toLowerCase()] = v; },
      status(code) { state.status = code; return this; },
      json(value) { state.body = value; resolve(state); }
    };
    try { handler({ query, body, headers }, res); } catch (err) { reject(err); }
  });
}

(async () => {
  const ok = await invoke({ body: { arch: release.arch, build: String(release.min_build), language: "enu" } });
  assert.strictEqual(ok.status, 200);
  assert.strictEqual(ok.body.packages.length, 1);
  assert.strictEqual(ok.body.packages[0].version, release.version);
  assert.strictEqual(ok.body.packages[0].link, `https://packages.test/releases/${release.filename}`);
  assert.strictEqual(ok.body.packages[0].qupgrade, true);
  assert.strictEqual(ok.body.packages[0].md5, release.md5);
  assert.strictEqual(ok.body.packages[0].size, release.size);

  const wrongArch = await invoke({ query: { arch: "x86_64", build: String(release.min_build) } });
  assert.deepStrictEqual(wrongArch.body.packages, []);

  const oldBuild = await invoke({ body: `arch=${release.arch}&build=${release.min_build - 1}&language=enu` });
  assert.deepStrictEqual(oldBuild.body.packages, []);

  console.log("package source tests: PASS");
})().catch(err => {
  console.error(err);
  process.exit(1);
});
