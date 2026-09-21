// Synthetic HTTP transport for native chat UI probes; not backend/E2E evidence.
// Never point a personal account or production app at this fixture.
import http from 'node:http';
import { mkdirSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join } from 'node:path';
import { appendFileSync, writeFileSync } from 'node:fs';
const dir=process.env.CHAT_NATIVE_EVIDENCE_DIR ?? join(tmpdir(),'rudi-chat-native-fixture');
mkdirSync(dir,{recursive:true});
// repo-guard: allow=long-number reason=synthetic-native-fixture-ids
const group='11111111-1111-4111-8111-111111111111', me='22222222-2222-4222-8222-222222222222', peer='33333333-3333-4333-8333-333333333333';
const groups=[{id:group,display_name:'QA Chat 21-09',my_state:'active',my_role:'admin',membership_id:'qa-member',member_count:2,unread_count:0,kind:'group',theme:'mac-dinh'}];
const session={token:'synthetic-native-only',person_id:me,expires_at:'2030-01-01T00:00:00Z',context_id:group,membership_state:'active',membership_id:'qa-member',issued_via:'otp',is_new_person:false,profile:{display_name:'QA Tôi'},contexts:groups};
let messages=Array.from({length:50},(_,i)=>({id:`qa-${String(i+1).padStart(3,'0')}`,context_id:group,author_id:i%4===0?me:peer,kind:'text',body:['Hẹn hội ở cổng nhé.','Mình mang nước, bạn mang áo khoác nhé.','Đi cùng nhau, về có chuyện để kể.','Tin thử nghiệm native.'][i%4],image_url:null,card:null,cursor:`qa-${String(i+1).padStart(3,'0')}`,created_at:new Date(Date.UTC(2026,8,21,9,i)).toISOString(),reactions:[]}));
const attempts=new Map();let failNext=false;let serial=51;
function send(res,body,status=200){res.writeHead(status,{'content-type':'application/json'});res.end(body===null?'':JSON.stringify(body));}
writeFileSync(`${dir}/fixture-events.jsonl`,'');
http.createServer(async(req,res)=>{
 const u=new URL(req.url,'http://localhost'); const path=u.pathname;
 let body={};let raw='';for await(const b of req)raw+=b;try{body=JSON.parse(raw||'{}')}catch{}
 appendFileSync(`${dir}/fixture-events.jsonl`,JSON.stringify({at:Date.now(),method:req.method,path,message_id:body.message_id??null,send_text:body.body??null,query:u.search})+'\n');
 if(path==='/control/fail-next'){failNext=true;return send(res,{ok:true});}
 if(path==='/control/new'){const id=`qa-${serial++}`;const msg={...messages.at(-1),id,cursor:id,created_at:new Date().toISOString(),body:'Tin mới khi bạn đang đọc lịch sử',author_id:peer};messages.push(msg);return send(res,msg);}
 if(path==='/auth/otp/request')return send(res,{challenge_id:'qa-native-challenge',expires_in_seconds:600,resend_after_seconds:60});
 if(path==='/auth/otp/verify')return send(res,session);
 if(path==='/people/me/contexts')return send(res,{contexts:groups});
 if(path==='/people/me')return send(res,{person_id:me,display_name:'QA Tôi',bio:'',interests:[],vibes:[],stats:{contexts:1,friends:0,outings:0}});
 if(path.endsWith('/members'))return send(res,{members:[{id:'qa-member',context_id:group,person_id:me,display_name:'QA Tôi',state:'active',role:'admin'},{id:'qa-peer',context_id:group,person_id:peer,display_name:'QA Bạn',state:'active',role:'member'}]});
 if(path.endsWith('/read-mark'))return send(res,null,204);
 if(path.endsWith('/messages')&&req.method==='GET'){
  let list=messages;
  if(u.searchParams.has('after')){const i=messages.findIndex(m=>m.cursor===u.searchParams.get('after'));list=messages.slice(i+1);}
  if(u.searchParams.has('before'))list=[];
  return send(res,{context_id:group,messages:[...list].reverse(),has_more:false,next_cursor:list.at(-1)?.cursor??u.searchParams.get('after')??null});
 }
 if(path.endsWith('/messages')&&req.method==='POST'){
  if(failNext){failNext=false;return setTimeout(()=>send(res,{code:'qa_timeout',detail:'Mạng thử nghiệm gián đoạn. Bấm Thử lại.'},503),7000);}
  const key=req.headers['idempotency-key'];let msg=attempts.get(key);
  if(!msg){const id=`qa-${serial++}`;msg={id,cursor:id,context_id:group,author_id:me,kind:body.kind??'text',body:body.body,image_url:body.image_url??null,created_at:new Date().toISOString(),card:null,reactions:[],intent:null};attempts.set(key,msg);messages.push(msg);}
  return send(res,msg,201);
 }
 if(path.includes('places'))return send(res,{places:[],total:0});
 if(path.includes('outings'))return send(res,{outings:[]});
 if(path.includes('friends'))return send(res,{friends:[],requests:[]});
 return send(res,{code:'qa_route_absent',detail:'Đường này ngoài lát kiểm thử native.'},404);
}).listen(20177,'127.0.0.1',()=>console.log('SYNTHETIC ONLY HTTP :20177'));
