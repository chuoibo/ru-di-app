/** The shipped account client talks to the Go front door on a disposable stack.
 * No fetch repair, phone identity, seed login or proof-retrieval endpoint.
 */
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";
import { BASE_URL, datTokenPhien, tokenPhienHienTai } from "../../dist-test/api.js";
import { loginAccount, getAccount, setDiscovery, reauthenticate, changePassword, logoutAll } from "../../dist-test/rudi/account.js";
import { timBanTheoUsername } from "../../dist-test/screens/ca-nhan/ban-be.js";
import { khoiPhucPhien } from "../../dist-test/phien.js";
const required=process.env.MOBILE_REQUIRE_E2E==="1";
test("tài khoản: UUID, client login, khôi phục phiên, quyền discovery, đổi mật khẩu và thu hồi",async t=>{
 const file=process.env.RUDI_TEST_ACCOUNT_WORLD;
 if(!file){if(required)assert.fail("missing isolated account world");t.skip("requires isolated account stack");return}
 const world=JSON.parse(readFileSync(file,"utf8"));
 const a=world.linh,b=world.quan;
 const password="isolated synthetic credential for linh";
 await assert.rejects(()=>loginAccount(a.username,"wrong synthetic credential"),/Tên tài khoản hoặc mật khẩu chưa đúng/);
 const session=await loginAccount(a.username,password);
 assert.equal(session.person_id,a.person_id);
 assert.match(session.person_id,/^[a-f0-9-]{14}4[a-f0-9-]{21}$/);
 assert.equal(session.issued_via,"password");
 assert.equal(tokenPhienHienTai(),session.token);
 const restored=await khoiPhucPhien();
 assert.equal(restored.person_id,a.person_id);
 assert.equal((await getAccount(a.person_id)).username,a.username);
 // The other actor header cannot confer their account's authority.
 assert.equal((await getAccount(b.person_id)).username,a.username);
 await setDiscovery(a.person_id,false);
 const bSession=await loginAccount(b.username,"isolated synthetic credential for quan");
 await assert.rejects(()=>timBanTheoUsername(a.username,b.person_id),/Chưa tìm thấy tài khoản/);
 datTokenPhien(session.token);
 await setDiscovery(a.person_id,true);
 datTokenPhien(bSession.token);
 const found=await timBanTheoUsername(a.username,b.person_id);
 assert.equal(found.person_id,a.person_id);
 datTokenPhien(session.token);
 await assert.rejects(()=>reauthenticate(a.person_id,{password:"wrong synthetic credential"}));
 await reauthenticate(a.person_id,{password});
 const nextPassword="changed isolated synthetic credential for linh";
 const rotated=await changePassword(a.person_id,nextPassword);
 assert.notEqual(rotated.token,session.token);
 assert.equal(rotated.person_id,a.person_id);
 const old=await fetch(`${BASE_URL}/people/me/account`,{headers:{Authorization:`Bearer ${session.token}`}});
 assert.equal(old.status,401);
 await assert.rejects(()=>loginAccount(a.username,password));
 const next=await loginAccount(a.username,nextPassword);
 assert.equal(next.person_id,a.person_id);
 await logoutAll(a.person_id);
 await assert.rejects(()=>getAccount(a.person_id));
 datTokenPhien(null);
});
