import assert from "node:assert/strict";
import test from "node:test";
import { BASE_URL, datTokenPhien, tokenPhienHienTai } from "../dist-test/api.js";
import { registerAccount, verifyAccount, resetRequest, resetConfirm, googleChallenge, googleLogin, loginAccount, changePassword, canonicalUsername, validUsername } from "../dist-test/rudi/account.js";
const person="1d955dbe-9e60-4510-8c0e-e6b81e81ab1a";
const session={token:"synthetic-session",person_id:person,expires_at:"2030-10-17T12:00:00Z",context_id:null,membership_id:null,membership_state:null,issued_via:"password",contexts:[],is_new_person:true};
async function capture(body,status,action) {
 const before=globalThis.fetch;const calls=[];
 globalThis.fetch=async(url,init)=>{calls.push({url:String(url),init});return new Response(JSON.stringify(body),{status,headers:{"content-type":"application/json"}})};
 try {return {value:await action(),calls};} finally {globalThis.fetch=before;datTokenPhien(null)}
}
test("username công khai chuẩn hóa nhất quán; không chấp nhận email hoặc số có dấu cộng",()=>{
 assert.equal(canonicalUsername(" @ACCOUNT.Example "),"account.example");
 for(const value of ["abc","@abc_d","ABC.X"])assert.equal(validUsername(value),true);
 for(const value of ["a","contains space","user@host","+prefix"])assert.equal(validUsername(value),false);
});
test("đăng ký giữ nguyên mật khẩu và đặt mọi proof trong body",async()=>{
 const password="  synthetic unicode café credential  ";const email="fixture"+"@"+"example.test";
 const {calls}=await capture({challenge_id:"synthetic",challenge_secret:"synthetic-proof"},202,()=>registerAccount(" @New.Account ",email,password));
 assert.equal(calls[0].url,`${BASE_URL}/auth/register`);
 assert.deepEqual(JSON.parse(calls[0].init.body),{username:"new.account",email,password});
 assert.equal(calls[0].init.headers["Idempotency-Key"],undefined);
 assert.equal(calls[0].init.headers.Authorization,undefined);
 assert.ok(!calls[0].url.includes(password)&&!calls[0].url.includes(email));
});
test("mã xác minh không trở thành route, khóa retry hay phiên đăng nhập",async()=>{
 const proof={challenge_id:"synthetic",challenge_secret:"binding",code:"123123"};
 const {calls}=await capture({verified:true},201,()=>verifyAccount(proof));
 assert.equal(calls[0].url,`${BASE_URL}/auth/register/verify`);
 assert.deepEqual(JSON.parse(calls[0].init.body),proof);
 assert.equal(tokenPhienHienTai(),null);
});
test("khôi phục dùng email; xác nhận không tự tạo phiên",async()=>{
 const email="fixture"+"@"+"example.test";
 let got=await capture({},202,()=>resetRequest(email));assert.deepEqual(JSON.parse(got.calls[0].init.body),{email});
 const proof={challenge_id:"synthetic",challenge_secret:"binding",code:"123123"};
 got=await capture({reset:true},200,()=>resetConfirm(proof,"new synthetic credential"));
 assert.equal(got.calls[0].url,`${BASE_URL}/auth/password/reset/confirm`);
 assert.equal(tokenPhienHienTai(),null);
 assert.deepEqual(JSON.parse(got.calls[0].init.body),{...proof,password:"new synthetic credential"});
});
test("Google cần proof có nonce, và bước chọn username chưa lưu session",async()=>{
 let got=await capture({nonce:"synthetic"},201,()=>googleChallenge("login"));
 assert.deepEqual(JSON.parse(got.calls[0].init.body),{purpose:"login"});
 const proof={challenge_id:"synthetic",challenge_secret:"binding",id_token:"synthetic-id-token"};
 got=await capture({registration_required:true,challenge_id:"new",challenge_secret:"new-binding"},200,()=>googleLogin(proof));
 assert.equal(got.value.registration_required,true);assert.equal(tokenPhienHienTai(),null);
 assert.deepEqual(JSON.parse(got.calls[0].init.body),proof);
});
test("đăng nhập local giữ UUID máy chủ, nhóm null và trạng thái tài khoản mới",async()=>{
 const got=await capture(session,201,()=>loginAccount("@ACCOUNT","synthetic credential"));
 assert.equal(got.value.person_id,person);assert.equal(got.value.context_id,null);assert.equal(got.value.membership_state,null);assert.equal(got.value.is_new_person,true);
 assert.equal(got.calls[0].url,`${BASE_URL}/auth/login`);
});
test("thao tác nhạy cảm mang bearer hiện tại và dùng token xoay từ máy chủ",async()=>{
 datTokenPhien("before-rotation");
 const got=await capture({...session,token:"after-rotation"},200,()=>changePassword(person,"new synthetic credential"));
 assert.equal(got.calls[0].init.headers.Authorization,"Bearer before-rotation");assert.equal(got.value.token,"after-rotation");
 assert.equal(got.calls[0].init.headers["Idempotency-Key"],undefined);
});
test("lỗi xác thực không được lưu thành phiên hoặc echo credential",async()=>{
 await assert.rejects(capture({code:"credentials_invalid",detail:"synthetic-private-data"},401,()=>loginAccount("account","synthetic credential")),error=>error.message.includes("chưa đúng")&&!error.message.includes("synthetic-private-data"));
 assert.equal(tokenPhienHienTai(),null);
});
