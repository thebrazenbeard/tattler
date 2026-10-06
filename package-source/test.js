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

function artifact(arch) {
  return release.releases.find(item => item.arch === arch);
}

(async () => {
  assert.deepStrictEqual(
    release.releases.map(item => item.arch).sort(),
    ["armv7", "armv8", "x86_64"]
  );

  for (const arch of ["x86_64", "armv7", "armv8"]) {
    const item = artifact(arch);
    const ok = await invoke({ body: { arch, build: String(release.min_build), language: "enu" } });
    assert.strictEqual(ok.status, 200);
    assert.strictEqual(ok.body.packages.length, 1);
    assert.strictEqual(ok.body.packages[0].version, release.version);
    assert.strictEqual(ok.body.packages[0].link, `https://packages.test/releases/${item.filename}`);
    assert.strictEqual(ok.body.packages[0].qupgrade, true);
    assert.strictEqual(ok.body.packages[0].md5, item.md5);
    assert.strictEqual(ok.body.packages[0].size, item.size);
  }

  const aliases = [
    ["armada38x", "armv7"],
    ["rtd1296", "armv8"],
    ["geminilake", "x86_64"]
  ];
  for (const [requested, expected] of aliases) {
    const item = artifact(expected);
    const ok = await invoke({ query: { arch: requested, build: String(release.min_build) } });
    assert.strictEqual(ok.body.packages.length, 1);
    assert.strictEqual(ok.body.packages[0].link, `https://packages.test/releases/${item.filename}`);
  }

  const unknownArch = await invoke({ query: { arch: "mystery-cpu", build: String(release.min_build) } });
  assert.deepStrictEqual(unknownArch.body.packages, []);

  const oldBuild = await invoke({ body: `arch=armada38x&build=${release.min_build - 1}&language=enu` });
  assert.deepStrictEqual(oldBuild.body.packages, []);

  console.log("package source tests: PASS");
})().catch(err => {
  console.error(err);
  process.exit(1);
});
