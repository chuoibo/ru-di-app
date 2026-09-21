/** Real Expo UI sessions against an isolated Go/PostgreSQL stack; no API mocks. */
import fs from 'node:fs';
import path from 'node:path';
import puppeteer from 'puppeteer-core';
const fixture=JSON.parse(fs.readFileSync(process.env.CHAT_E2E_SESSIONS,'utf8'));
const output=process.env.CHAT_E2E_OUTPUT;
if(!output?.startsWith('/tmp/'))throw Error('Runtime artifacts must stay outside the checkout.');
fs.mkdirSync(output,{recursive:true,mode:0o700});
const web=process.env.CHAT_E2E_WEB??'http://127.0.0.1:8177';
const count=Number(process.env.CHAT_E2E_USERS??20),run=`E2E-${Date.now()}`;
const result={run,users:count,mockResponses:false,startedAt:new Date().toISOString(),steps:[],clients:[],requests:[]};
const save=()=>fs.writeFileSync(path.join(output,'results.json'),JSON.stringify(result,null,2));
const sleep=ms=>new Promise(r=>setTimeout(r,ms));
// repo-guard: allow=long-number reason=public-chrome-version
const browser=await puppeteer.launch({executablePath:process.env.CHROME_PATH??'/home/lakiet/.cache/puppeteer/chrome/linux-148.0.7778.97/chrome-linux64/chrome',headless:true,args:['--no-sandbox','--disable-background-timer-throttling','--disable-backgrounding-occluded-windows','--disable-renderer-backgrounding']});
fs.writeFileSync(path.join(output,'browser-endpoint'),browser.wsEndpoint(),{mode:0o600});
const pages=[],composer='[aria-label="Ô soạn tin"]';
async function api(user,route){const r=await fetch(fixture.apiUrl+route,{headers:{Authorization:`Bearer ${user.token}`}});if(!r.ok)throw Error(`Verification API ${r.status}`);return r.json()}
async function fill(p,text){await p.click(composer);await p.keyboard.down('Control');await p.keyboard.press('KeyA');await p.keyboard.up('Control');await p.keyboard.type(text)}
async function send(p,text){await fill(p,text);await p.click('[aria-label="Gửi tin nhắn"]')}
try{
for(let i=0;i<count;i++){
 const context=await browser.createBrowserContext(),p=await context.newPage();pages.push(p);await p.setViewport({width:430,height:932});
 const cdp=await p.createCDPSession();await cdp.send('Emulation.setFocusEmulationEnabled',{enabled:true});
 result.clients.push({index:i,errors:[],personId:fixture.users[i].personId});
 p.on('pageerror',e=>{result.clients[i].errors.push(e.message);save()});
 p.on('response',r=>{if(r.url().startsWith(fixture.apiUrl))result.requests.push({client:i,path:new URL(r.url()).pathname,method:r.request().method(),status:r.status(),at:Date.now()})});
 await p.goto(web+'/login');await p.waitForSelector('[aria-label="Ô số điện thoại"]');await p.type('[aria-label="Ô số điện thoại"]',fixture.users[i].phone??`0900000${100+i}`);
 let logged=false;
 for(let attempt=0;attempt<3&&!logged;attempt++){
  const pending=p.waitForResponse(r=>r.url().endsWith('/auth/otp/request')&&r.request().method()==='POST');await p.click('[data-testid="login-gui-ma"]');const r=await pending;
  if(r.status()===429){console.log(`OTP limit respected for user ${i}; waiting 61 seconds`);await sleep(61000);continue}
  if(r.status()!==200&&r.status()!==202)throw Error(`OTP request ${i}: ${r.status()}`);
  await p.waitForSelector('[data-testid="otp-input"]');await p.type('[data-testid="otp-input"]','000000');
  await p.waitForSelector('[role="tab"][aria-label="Tin nhắn"]');await p.click('[role="tab"][aria-label="Tin nhắn"]');
  await p.waitForSelector('[aria-label="Mở nhóm Phòng kiểm thử đồng thời"]');await p.click('[aria-label="Mở nhóm Phòng kiểm thử đồng thời"]');await p.waitForSelector(composer);logged=true;
 }
 if(!logged)throw Error(`Login failed ${i}`);
 await p.evaluate(index=>{window.__chatE2EUserIndex=index},i);
 result.clients[i].visibility=await p.evaluate(()=>document.visibilityState);console.log(`Actual UI user ${i+1}/${count} ready`);save();
}
result.steps.push({name:'distinct_authenticated_UI_sessions',pass:true,count});
await Promise.all(pages.map(p=>p.evaluate(prefix=>{window.__chatE2ESeen={};const scan=()=>{for(const el of document.querySelectorAll('[aria-label^="Tin nhắn: "]')){const text=el.getAttribute('aria-label').slice('Tin nhắn: '.length);if(text.includes(prefix)&&!window.__chatE2ESeen[text])window.__chatE2ESeen[text]=Date.now()}};new MutationObserver(scan).observe(document.body,{subtree:true,childList:true,attributes:true});scan()},run)));
const texts=pages.map((_,i)=>`${run} user-${String(i+1).padStart(2,'0')} chào cả nhóm 🌿`);
await Promise.all(pages.map((p,i)=>fill(p,texts[i])));const started=Date.now();await Promise.all(pages.map(p=>p.click('[aria-label="Gửi tin nhắn"]')));await sleep(12000);
const observations=await Promise.all(pages.map(p=>p.evaluate(()=>({seen:window.__chatE2ESeen,labels:Array.from(document.querySelectorAll('[aria-label^="Tin nhắn: "]')).map(e=>e.getAttribute('aria-label').slice('Tin nhắn: '.length))}))));
const stored=await api(fixture.users[0],`/contexts/${fixture.groupId}/messages?limit=100`),persisted=stored.messages.filter(m=>texts.includes(m.body));
const delivery=observations.flatMap((o,i)=>texts.map(text=>({receiver:i,text,delayMs:o.seen[text]===undefined?null:o.seen[text]-started,copies:o.labels.filter(x=>x===text).length})));
const delays=delivery.filter(d=>d.delayMs!==null).map(d=>d.delayMs).sort((a,b)=>a-b);
result.steps.push({name:'simultaneous_group_send',pass:persisted.length===count&&delivery.every(d=>d.delayMs!==null&&d.copies===1),messages:count,expectedUIDeliveries:count*count,persisted:persisted.length,missingUIDeliveries:delivery.filter(d=>d.delayMs===null).length,duplicateUIDeliveries:delivery.filter(d=>d.copies>1).length,renderP50Ms:delays[Math.floor(delays.length*.5)],renderP95Ms:delays[Math.floor(delays.length*.95)],renderP99Ms:delays[Math.floor(delays.length*.99)],delivery});save();
await pages[0].screenshot({path:path.join(output,'01-group-concurrent.png')});
const receiver=pages[count-1];await receiver.setOfflineMode(true);const offlineText=`${run} reconnect sau mất mạng`;await send(pages[0],offlineText);await sleep(5500);
const absentOffline=!(await receiver.evaluate(t=>document.body.innerText.includes(t),offlineText));await receiver.setOfflineMode(false);let recovered=true;
try{await receiver.waitForFunction(t=>document.body.innerText.includes(t),{timeout:15000},offlineText)}catch{recovered=false}
result.steps.push({name:'offline_reconnect_receive',pass:absentOffline&&recovered,absentOffline,recovered});save();
await fill(pages[0],'/');await pages[0].screenshot({path:path.join(output,'02-slash-menu.png')});
await fill(pages[0],`/plan ${run} cả nhóm muốn đi cà phê Đà Lạt tối nay, hãy phác lịch trình`);
await pages[0].click('[aria-label="Gửi tin nhắn"]');
await pages[0].waitForFunction(()=>document.body.innerText.includes('Chỉ lời nhờ trong ô này được gửi cho AI.'),{timeout:10000});
const capability=await api(fixture.users[0],`/contexts/${fixture.groupId}/chat-capabilities`);
const consentUI=await pages[0].evaluate(()=>document.body.innerText.includes('Lịch sử chat không được chia sẻ.'));
if(capability.ai.plan.available){
 const pendingPlan=pages[0].waitForResponse(r=>r.url().endsWith(`/contexts/${fixture.groupId}/ai-invocations`)&&r.request().method()==='POST');
 await pages[0].evaluate(()=>Array.from(document.querySelectorAll('[role="button"]')).find(el=>el.textContent.trim()==='Gửi lời nhờ cho AI')?.click());
 const response=await pendingPlan, invocation=await response.json();
 result.steps.push({name:'slash_plan_explicit_invocation',pass:response.status()<300&&consentUI,status:response.status(),invocationId:invocation.id,jobStatus:invocation.status,modelCompletionProven:false});
}else{
 const manual=await pages[0].evaluate(()=>document.body.innerText.includes('Tự tạo kèo'));
 result.steps.push({name:'slash_plan_unavailable_honest_recovery',pass:consentUI&&manual,modelCompletionProven:false,reason:capability.ai.plan.reason});
}
save();
await pages[0].screenshot({path:path.join(output,'03-plan-result.png')});
console.log(JSON.stringify(result.steps.map(({delivery,uiText,companion,...step})=>step),null,2));
}catch(e){result.fatal=e.message;console.error(e.message);process.exitCode=1;await pages.at(-1)?.screenshot({path:path.join(output,'failure.png')}).catch(()=>{})}
finally{if(result.steps.some(step=>step.pass===false)||result.clients.some(client=>client.errors.length>0))process.exitCode=1;result.finishedAt=new Date().toISOString();save();if(process.env.CHAT_E2E_KEEP_BROWSER==='1'){console.log('Browser retained for additional actual UI tests.');await new Promise(resolve=>{const poll=setInterval(()=>{if(fs.existsSync(path.join(output,'stop'))){clearInterval(poll);resolve()}},1000)})}await browser.close()}
