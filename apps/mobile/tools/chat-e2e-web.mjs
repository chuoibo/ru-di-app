/** Serve an Expo web export on loopback for the real-browser chat harness. */
import http from 'node:http';
import {readFile} from 'node:fs/promises';
import path from 'node:path';
const root=path.resolve(process.argv[2]??'');
const port=Number(process.argv[3]??8177);
if(!root.startsWith('/tmp/')||!Number.isInteger(port)||port<1024||port>65535)throw Error('Expected an export directory outside the checkout and an unprivileged port.');
const mime={'.html':'text/html','.js':'application/javascript','.css':'text/css','.ttf':'font/ttf','.png':'image/png','.jpg':'image/jpeg','.ico':'image/x-icon'};
http.createServer(async(req,res)=>{
 try{
  const pathname=decodeURIComponent(new URL(req.url,'http://localhost').pathname);
  let file=path.resolve(root,'.'+pathname);
  if(!file.startsWith(root+'/'))file=path.join(root,'index.html');
  let bytes;try{bytes=await readFile(file)}catch{file=path.join(root,'index.html');bytes=await readFile(file)}
  res.setHeader('Content-Type',mime[path.extname(file)]??'application/octet-stream');res.setHeader('Cache-Control','no-store');res.end(bytes);
 }catch{res.writeHead(500);res.end()}
}).listen(port,'127.0.0.1',()=>console.log(`Expo test export: http://127.0.0.1:${port}`));
