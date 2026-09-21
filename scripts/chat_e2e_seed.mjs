import fs from 'node:fs';
import{randomUUID}from'node:crypto';
// All identities and OTPs are synthetic; never use against a shared database.
const connection=process.argv[2];
if(!connection?.startsWith('/tmp/rudi-chat-e2e.'))throw Error('Expected isolated stack connection path');
const c=JSON.parse(fs.readFileSync(connection));
const file=c.root+'/sessions.json';const state={apiUrl:c.apiUrl,users:[],groupId:null,dmIds:[],outcomes:[]};
if(new URL(c.apiUrl).hostname!=='127.0.0.1'||!c.root.startsWith('/tmp/rudi-chat-e2e.'))throw Error('Expected isolated loopback stack');
if(fs.existsSync(file))throw Error('Refusing to overwrite existing synthetic sessions');
function save(){
  fs.writeFileSync(file+'.tmp',JSON.stringify(state,null,2),{mode:0o600});
  fs.renameSync(file+'.tmp',file);
}
async function req(method,path,body,user,retry=true){const headers={'Content-Type':'application/json','Idempotency-Key':randomUUID()};if(user)headers.Authorization='Bearer '+user.token;const r=await fetch(c.apiUrl+path,{method,headers,body:body===undefined?undefined:JSON.stringify(body)});const text=await r.text();let j;try{j=JSON.parse(text)}catch{j=text}state.outcomes.push({method,path,status:r.status});if(r.status===429&&retry){console.log('Rate limit honored; waiting 61s');await new Promise(resolve=>setTimeout(resolve,61000));return req(method,path,body,user,false)}if(!r.ok)throw Error(method+' '+path+' '+r.status+' '+JSON.stringify(j));return j}
for(let i=0;i<22;i++){
// repo-guard: allow=long-number reason=synthetic-isolated-otp-fixtures
const phone='+'+[84,900000100+i].join('');const challenge=await req('POST','/auth/otp/request',{phone});
const session=await req('POST','/auth/otp/verify',{phone,challenge_id:challenge.challenge_id,code:'000000'});const u={index:i,personId:session.person_id,token:session.token,expiresAt:session.expires_at,phone,name:'Chat Test '+String(i+1).padStart(2,'0')};
await req('PATCH','/people/me',{display_name:u.name},u);state.users.push(u);if(i===0){const group=await req('POST','/contexts',{display_name:'Phòng kiểm thử đồng thời'},u);state.groupId=group.id;}
else if(i<20){const membership=await req('POST','/contexts/'+state.groupId+'/members',{person_id:u.personId},state.users[0]);await req('POST','/memberships/'+membership.id+'/accept',{},u)}
save();console.log('Ready users:',state.users.length);
}
for(let i=0;i<4;i+=2){
  const edge=await req('POST','/friends/requests',{addressee_id:state.users[i+1].personId},state.users[i]);
  await req('POST','/friends/requests/'+edge.id+'/respond',{decision:'accept'},state.users[i+1]);
  const dm=await req('POST','/people/'+state.users[i+1].personId+'/dm',{},state.users[i]);
  state.dmIds.push(dm.id);
}
save();console.log('Fixtures ready:',file);
