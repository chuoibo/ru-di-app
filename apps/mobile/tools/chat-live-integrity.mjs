/** Independently compare current rendered order with the actual server page. */
import fs from 'node:fs';
import path from 'node:path';
import puppeteer from 'puppeteer-core';
const root=process.env.CHAT_E2E_OUTPUT;
const f=JSON.parse(fs.readFileSync(process.env.CHAT_E2E_SESSIONS,'utf8'));
const run=JSON.parse(fs.readFileSync(path.join(root,'results.json'),'utf8')).run;
const response=await fetch(`${f.apiUrl}/contexts/${f.groupId}/messages?limit=100`,{headers:{Authorization:`Bearer ${f.users[0].token}`}});
if(!response.ok)throw Error(`Server verification ${response.status}`);
const expected=(await response.json()).messages.filter(m=>m.body?.startsWith(run+' user-')).map(m=>m.body);
const browser=await puppeteer.connect({defaultViewport:null,browserWSEndpoint:fs.readFileSync(path.join(root,'browser-endpoint'),'utf8')});
const clients=[];
for(const page of await browser.pages()){
 if(!await page.evaluate(()=>!!window.__chatE2ESeen))continue;
 const actual=await page.$$eval('[aria-label^="Tin nhắn: "]',(els,prefix)=>els.map(e=>e.getAttribute('aria-label').slice('Tin nhắn: '.length)).filter(t=>t.startsWith(prefix+' user-')),run);
 clients.push({count:actual.length,correctOrder:JSON.stringify(actual)===JSON.stringify(expected)});
}
const result={expectedMessages:expected.length,clients:clients.length,correctOrder:clients.every(c=>c.correctOrder),observations:clients};
fs.writeFileSync(path.join(root,'order.json'),JSON.stringify(result,null,2));console.log(result);browser.disconnect();
if(expected.length!==20||clients.length!==20||!result.correctOrder)process.exitCode=1;
