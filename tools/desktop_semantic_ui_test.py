from pathlib import Path
import subprocess,tempfile,unittest
ROOT=Path(__file__).resolve().parents[1]
class SemanticUIRuntimeTests(unittest.TestCase):
    def test_grouping_identity_semantics_and_evidence_filters(self):
        from ui_javascript_test import extract_script
        text=(ROOT/"desktop"/"frontend"/"dist"/"index.html").read_text(encoding="utf-8")
        source=extract_script(text).split("for(const id of ['view-filter'")[0]
        assertions='''
const assert=require("assert");
const c={kind:"tcp_session",protocol:"tcp",direction:"outbound",state:"ESTABLISHED",local:"127.0.0.1:5000",remote:"8.8.8.8:443",process:{pid:71,name:"browser"},tracking_key:"one",age_seconds:20};
const c2={...c,local:"127.0.0.1:5001",tracking_key:"two",age_seconds:10};
const same={...c,tracking_key:"one"};
let groups=groupRows([c,c2]); assert.strictEqual(groups.length,1); assert.strictEqual(groups[0].items.length,2);
assert.strictEqual(groupKeys(groups[0].items),2);
groups=groupRows([c,same]); assert.strictEqual(groupKeys(groups[0].items),1);
grouped=false; assert.strictEqual(groupRows([c,c2]).length,2);
assert(remoteNonLoopback(c));
assert(!remoteNonLoopback({...c,kind:"udp_endpoint",remote:"0.0.0.0:0"}));
assert(loopback("[::1]:443"));assert(loopback("127.0.0.1:80"));
assert(!loopback("0.0.0.0:80"));assert(unknownOwner({}));
assert(!unknownOwner(c));
console.log("semantic UI data invariants PASS");
'''
        with tempfile.TemporaryDirectory() as tmp:
            f=Path(tmp)/"behavior.js"
            f.write_text(source+assertions,encoding="utf-8")
            run=subprocess.run(["node",str(f)],capture_output=True,text=True)
        self.assertEqual(run.returncode,0,run.stdout+run.stderr)
if __name__=="__main__":
    unittest.main()
