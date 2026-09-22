/** Real OTP/browser regressions for chat drafts, route safety, theme and keyboard. */
import fs from 'node:fs';
import path from 'node:path';
import assert from 'node:assert/strict';
import puppeteer from 'puppeteer-core';
const fixture=JSON.parse(fs.readFileSync(process.env.CHAT_E2E_SESSIONS,'utf8'));
const output=process.env.CHAT_E2E_OUTPUT;
if(!output?.startsWith('/tmp/'))throw Error('Synthetic evidence must stay outside checkout.');
fs.mkdirSync(output,{recursive:true,mode:0o700});
const web=process.env.CHAT_E2E_WEB??'http://127.0.0.1:8178';
const result={mockResponses:false,steps:[],errors:[]};
const save=()=>fs.writeFileSync(path.join(output,'recovery.json'),JSON.stringify(result,null,2));
// repo-guard: allow=long-number reason=public-chrome-version
const browser=await puppeteer.launch({executablePath:process.env.CHROME_PATH??'/home/lakiet/.cache/puppeteer/chrome/linux-148.0.7778.97/chrome-linux64/chrome',headless:true,args:['--no-sandbox','--disable-background-timer-throttling']});
const p=await browser.newPage();await p.setViewport({width:430,height:932});
p.on('pageerror',e=>result.errors.push(e.message));
const sleep=ms=>new Promise(r=>setTimeout(r,ms));
const fill=async(sel,text)=>{await p.click(sel);await p.keyboard.down('Control');await p.keyboard.press('KeyA');await p.keyboard.up('Control');await p.keyboard.type(text)};
const button=async text=>{const handle=await p.waitForFunction(value=>[...document.querySelectorAll('[role="button"],button')].find(el=>el.textContent.trim()===value),{timeout:10000},text);await handle.asElement().scrollIntoView();await handle.asElement().click()};
const clickVisible=async selector=>{const handle=await p.waitForFunction(selector=>[...document.querySelectorAll(selector)].find(el=>{const r=el.getBoundingClientRect();return r.width>0&&r.height>0&&r.top>=0&&r.left>=0&&r.right<=innerWidth&&!el.closest('[inert]')}),{timeout:10000},selector);await handle.asElement().click()};
const closeTray=async()=>{await p.keyboard.press('Escape');await p.waitForSelector('[aria-label="Đóng khay công cụ"]',{hidden:true})};
const open=async label=>{await p.click('[aria-label="Thêm vào cuộc trò chuyện"]');await p.click(`[aria-label="${label}"]`)};
const enter=async()=>{await p.waitForSelector('[aria-label="Mở nhóm Phòng kiểm thử đồng thời"]');await p.click('[aria-label="Mở nhóm Phòng kiểm thử đồng thời"]');await p.waitForSelector('[aria-label="Ô soạn tin"]')};
const step=(name,data={})=>{result.steps.push({name,pass:true,...data});save()};
const colorProbe=async label=>p.evaluate(label=>{
 const node=[...document.querySelectorAll('div')].find(el=>el.children.length===0&&el.textContent===label);if(!node)throw Error('Missing label '+label);
 let ancestor=node;let bg;while(ancestor){bg=getComputedStyle(ancestor).backgroundColor;if(bg!=='rgba(0, 0, 0, 0)'&&bg!=='transparent')break;ancestor=ancestor.parentElement}
 const foreground=getComputedStyle(node).color;
 const luminance=rgb=>{const vals=rgb.match(/[\d.]+/g).slice(0,3).map(Number).map(x=>{x/=255;return x<=.04045?x/12.92:((x+.055)/1.055)**2.4});return vals[0]*.2126+vals[1]*.7152+vals[2]*.0722};
 const a=luminance(foreground),b=luminance(bg);return{foreground,background:bg,contrast:(Math.max(a,b)+.05)/(Math.min(a,b)+.05)};
},label);
try{
 await p.goto(web+`/groups/${fixture.groupId}/chat`);await p.waitForSelector('[aria-label="Ô số điện thoại"]');assert.ok(p.url().endsWith('/login'));assert.equal(await p.evaluate(()=>document.body.innerText.includes('Team Đà Lạt')),false);step('signed_out_real_chat_redirects_login');
 await p.type('[aria-label="Ô số điện thoại"]',fixture.users[Number(process.env.CHAT_E2E_USER_INDEX??6)].phone);
 let accepted=false;for(let attempt=0;attempt<3;attempt++){const pending=p.waitForResponse(r=>r.request().method()==='POST'&&r.url().endsWith('/auth/otp/request'));await p.click('[data-testid="login-gui-ma"]');const response=await pending;if(response.status()===429){await sleep(61000);continue}assert.ok([200,202].includes(response.status()));accepted=true;break}assert.ok(accepted,'OTP rate limit remained active');await p.waitForSelector('[data-testid="otp-input"]');await p.type('[data-testid="otp-input"]','000000');
 await p.waitForSelector('[role="tab"][aria-label="Tin nhắn"]');await p.click('[role="tab"][aria-label="Tin nhắn"]');await enter();
 await open('Bình chọn');await button('Gửi bình chọn');await p.waitForFunction(()=>document.body.innerText.includes('Thêm câu hỏi và ít nhất hai lựa chọn.'));
 const footer=await p.evaluate(()=>{const el=[...document.querySelectorAll('[role="button"]')].find(e=>e.textContent.trim()==='Gửi bình chọn');const r=el.getBoundingClientRect();return{top:r.top,bottom:r.bottom,height:innerHeight}});assert.ok(footer.top>=0&&footer.bottom<=footer.height);step('poll_validation_and_submit_visible',footer);
 await closeTray();await open('Tờ hẹn');assert.equal(await p.evaluate(()=>document.body.innerText.includes('Thêm câu hỏi và ít nhất hai lựa chọn.')),false);await closeTray();await open('Bình chọn');
 const question=`Nháp chưa gửi ${Date.now()}`;await fill('[aria-label="Câu hỏi bình chọn"]',question);await fill('[aria-label="Lựa chọn 1"]','Bờ hồ');await fill('[aria-label="Lựa chọn 2"]','Quán nhỏ');await button('Thêm lựa chọn');await fill('[aria-label="Lựa chọn 3"]','Công viên');assert.equal(await p.evaluate(()=>document.body.innerText.includes('Thêm câu hỏi và ít nhất hai lựa chọn.')),false);step('validation_clears_after_correction_and_stays_in_poll');
 await closeTray();await clickVisible('[aria-label="Quay lại"]');await enter();await open('Bình chọn');
 const drafts=await p.evaluate(()=>['Câu hỏi bình chọn','Lựa chọn 1','Lựa chọn 2','Lựa chọn 3'].map(label=>document.querySelector(`[aria-label="${label}"]`).value));assert.deepEqual(drafts,[question,'Bờ hồ','Quán nhỏ','Công viên']);assert.ok(await p.evaluate(()=>document.body.innerText.includes('Đã khôi phục bản nháp')));step('poll_draft_restores_all_fields_after_leaving_room');await p.screenshot({path:path.join(output,'01-restored-poll.png')});
 await closeTray();await open('Tờ hẹn');const prompt=`Nháp AI ${Date.now()}`;await fill('[aria-label="Lời nhờ lập kế hoạch"]',prompt);await closeTray();await clickVisible('[aria-label="Quay lại"]');await enter();await open('Tờ hẹn');assert.equal(await p.$eval('[aria-label="Lời nhờ lập kế hoạch"]',el=>el.value),prompt);step('ai_prompt_restores_after_leaving_room');
 await button('Tự tạo kèo');await p.waitForSelector('[aria-label="Ô ngân sách một người"]');await p.goBack();await p.waitForSelector('[aria-label="Ô soạn tin"]');if(!await p.$('[aria-label="Lời nhờ lập kế hoạch"]'))await open('Tờ hẹn');assert.equal(await p.$eval('[aria-label="Lời nhờ lập kế hoạch"]',el=>el.value),prompt);step('manual_creation_route_is_explicit_and_retains_ai_draft');
 for(const scheme of ['dark','light']){
  await p.emulateMediaFeatures([{name:'prefers-color-scheme',value:scheme}]);await sleep(500);
  assert.equal(await p.evaluate(()=>matchMedia('(prefers-color-scheme: dark)').matches),scheme==='dark');
  const form=await colorProbe('Bạn muốn rủ hội đi đâu?');assert.ok(form.contrast>=4.5,JSON.stringify(form));assert.equal(form.foreground,scheme==='dark'?'rgb(244, 241, 234)':'rgb(31, 34, 48)');await p.screenshot({path:path.join(output,`02-plan-${scheme}.png`)});
  await button('Tự tạo kèo');await p.waitForSelector('[aria-label="Ô ngân sách một người"]');const create=await colorProbe('Kèo mới');assert.ok(create.contrast>=4.5,JSON.stringify(create));await p.goBack();await p.waitForSelector('[aria-label="Ô soạn tin"]');
  if(await p.$('[aria-label="Đóng khay công cụ"]'))await p.click('[aria-label="Đóng khay công cụ"]');
  await p.click('[aria-label="Cài đặt nhóm"]');await p.waitForSelector('[role="dialog"]');const settings=await colorProbe('Màu bong bóng');assert.ok(settings.contrast>=4.5,JSON.stringify(settings));await p.keyboard.press('Escape');await p.waitForSelector('[role="dialog"]',{hidden:true});await open('Tờ hẹn');step(`system_${scheme}_palette_consistent`,{form,create,settings});
 }
 await p.keyboard.press('Escape');await p.setViewport({width:320,height:740});await p.focus('[aria-label="Cài đặt nhóm"]');await p.keyboard.press('Enter');await p.waitForSelector('[role="dialog"]');await sleep(200);
 const tabs=[];for(let i=0;i<14;i++){await p.keyboard.press('Tab');tabs.push(await p.evaluate(()=>({inside:!!document.activeElement.closest('[role="dialog"]'),label:document.activeElement.getAttribute('aria-label')})))}assert.ok(tabs.every(t=>t.inside));
 const sheet=await p.evaluate(()=>{const dialog=document.querySelector('[role="dialog"]');const colors=[...dialog.querySelectorAll('[role="radio"]')].map(el=>{const r=el.getBoundingClientRect();return{left:r.left,right:r.right,top:r.top,bottom:r.bottom}});return{colors,backgroundInert:!!document.querySelector('[aria-label="Ô soạn tin"]').closest('[inert]'),width:innerWidth}});assert.ok(sheet.backgroundInert);assert.equal(sheet.colors.length,5);assert.ok(sheet.colors.every(r=>r.left>=0&&r.right<=sheet.width));await p.screenshot({path:path.join(output,'03-sheet-320.png')});
 const cdp=await p.createCDPSession();const ax=await cdp.send('Accessibility.getFullAXTree');fs.writeFileSync(path.join(output,'sheet-ax.json'),JSON.stringify(ax));const exposedComposer=ax.nodes.some(n=>!n.ignored&&n.name?.value==='Ô soạn tin');assert.equal(exposedComposer,false);
 await p.keyboard.press('Escape');await p.waitForSelector('[role="dialog"]',{hidden:true});assert.equal(await p.evaluate(()=>document.activeElement.getAttribute('aria-label')),'Cài đặt nhóm');step('sheet_focus_trap_escape_restore_ax_and_320_palette',{tabs,...sheet,exposedComposer});
 await p.reload();await p.waitForSelector('[aria-label="Ô số điện thoại"]');assert.ok(p.url().endsWith('/login'));step('runtime_session_reload_never_renders_demo');
 assert.deepEqual(result.errors,[]);
}catch(error){result.steps.push({name:'flow_error',pass:false,message:error.message,stack:error.stack});process.exitCode=1;await p.screenshot({path:path.join(output,'failure.png')}).catch(()=>{});}
finally{save();console.log(JSON.stringify(result,null,2));await browser.close()}
