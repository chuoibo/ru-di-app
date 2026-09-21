/** Exercise structured poll creation, peer tallies, changes and closure in real UI. */
import fs from 'node:fs';
import path from 'node:path';
import puppeteer from 'puppeteer-core';
const root=process.env.CHAT_E2E_OUTPUT;
if(!root?.startsWith('/tmp/'))throw Error('Synthetic artifacts must stay outside the checkout.');
const fixture=JSON.parse(fs.readFileSync(process.env.CHAT_E2E_SESSIONS,'utf8'));
const browser=await puppeteer.connect({defaultViewport:null,browserWSEndpoint:fs.readFileSync(path.join(root,'browser-endpoint'),'utf8')});
const selected=new Map();
for(const p of await browser.pages()){
 const index=await p.evaluate(()=>window.__chatE2EUserIndex).catch(()=>undefined);
 if(index===2||index===3)selected.set(index,p);
}
const a=selected.get(2),b=selected.get(3);
if(!a||!b)throw Error('Expected real authenticated user 03 and 04.');
const question=`Tờ hẹn thử nghiệm ${Date.now()}: tối nay ăn gì?`;
const steps=[];
const save=()=>fs.writeFileSync(path.join(root,'poll.json'),JSON.stringify({question,mockResponses:false,steps},null,2));
const clickText=async(p,text)=>{
 const handle=await p.waitForFunction(value=>[...document.querySelectorAll('[role="button"]')].find(el=>el.textContent.trim()===value),{timeout:10000},text);
 await handle.asElement().click();
};
const cardText=(p)=>p.evaluate(text=>[...document.querySelectorAll('div')].find(el=>el.children.length===0&&el.textContent===text&&el.parentElement?.querySelector('[role="radio"]'))?.parentElement?.innerText??'',question);
const select=async(p,label)=>{
 const handle=await p.waitForFunction((text,value)=>{
  const card=[...document.querySelectorAll('div')].find(el=>el.children.length===0&&el.textContent===text&&el.parentElement?.querySelector('[role="radio"]'))?.parentElement;
  return card?.querySelector(`[aria-label="Bỏ phiếu ${value}"]`);
 },{timeout:10000},question,label);
 await handle.asElement().scrollIntoView();await handle.asElement().click();
};
const waitTotal=(p,total)=>p.waitForFunction((text,count)=>{
 const card=[...document.querySelectorAll('div')].find(el=>el.children.length===0&&el.textContent===text&&el.parentElement?.querySelector('[role="radio"]'))?.parentElement;
 return !!card&&[...card.querySelectorAll('div')].some(el=>el.children.length===0&&el.textContent===`${count} phiếu`);
},{timeout:10000},question,total);
try{
 await a.click('[aria-label="Thêm vào cuộc trò chuyện"]');
 await a.click('[aria-label="Bình chọn"]');
 await a.type('[aria-label="Câu hỏi bình chọn"]',question);
 await a.type('[aria-label="Lựa chọn 1"]','Bún bò');
 await a.type('[aria-label="Lựa chọn 2"]','Phở');
 const created=a.waitForResponse(response=>response.request().method()==='POST'&&response.url().endsWith(`/contexts/${fixture.groupId}/messages`));
 await clickText(a,'Gửi bình chọn');
 const response=await created,body=await response.json();
 await b.waitForSelector('[aria-label="Bỏ phiếu Bún bò"]',{timeout:10000});
 const rawVisible=await b.evaluate(()=>[...document.querySelectorAll('[aria-label^="Tin nhắn: "]')].some(el=>el.getAttribute('aria-label').includes('/vote Tờ hẹn thử nghiệm')));
 steps.push({name:'structured_poll_create',pass:response.status()===201&&Boolean(body.vote?.id)&&!rawVisible,status:response.status(),voteId:body.vote?.id,rawCommandVisible:rawVisible});save();
 await select(a,'Bún bò');await waitTotal(b,1);
 const began=Date.now();await select(b,'Phở');await waitTotal(a,2);
 const latency=Date.now()-began;
 steps.push({name:'peer_ballot_live',pass:true,peerObservationMs:latency,observer:await cardText(a),peer:await cardText(b)});save();
 await select(b,'Bún bò');
 await a.waitForFunction(text=>{
  const card=[...document.querySelectorAll('div')].find(el=>el.children.length===0&&el.textContent===text&&el.parentElement?.querySelector('[role="radio"]'))?.parentElement;
  return !!card&&[...card.querySelectorAll('[role="radio"]')][0]?.textContent.includes('2 phiếu');
 },{timeout:10000},question);
 steps.push({name:'peer_changes_existing_ballot',pass:true,observer:await cardText(a)});save();
 await a.screenshot({path:path.join(root,'07-poll-live.png')});
 await clickText(a,'Chốt bình chọn');await clickText(a,'Đóng bình chọn');
 await b.waitForFunction(text=>{
  const card=[...document.querySelectorAll('div')].find(el=>el.children.length===0&&el.textContent===text)?.parentElement;
  return card?.innerText.includes('đã đóng')&&card.innerText.includes('Xem các phiếu');
 },{timeout:10000},question);
 await b.evaluate(text=>{const card=[...document.querySelectorAll('div')].find(el=>el.children.length===0&&el.textContent===text)?.parentElement;[...card.querySelectorAll('[role="button"]')].find(el=>el.textContent.trim()==='Xem các phiếu')?.click()},question);
 await b.waitForFunction(text=>{const card=[...document.querySelectorAll('div')].find(el=>el.children.length===0&&el.textContent===text)?.parentElement;const radios=[...card.querySelectorAll('[role="radio"]')];return radios.length===2&&radios.every(el=>el.getAttribute('aria-disabled')==='true')},{timeout:10000},question);
 steps.push({name:'peer_sees_closed_poll',pass:true,observer:await cardText(b)});save();
 await b.screenshot({path:path.join(root,'08-poll-closed-peer.png')});
}catch(error){steps.push({name:'poll_flow_error',pass:false,error:error.message});save();process.exitCode=1}
finally{console.log(JSON.stringify(steps,null,2));browser.disconnect()}
