// Read the token locally; never place it in command arguments or output.
import { readFileSync } from 'node:fs';
import { createHash } from 'node:crypto';
import assert from 'node:assert/strict';
const cfg=JSON.parse(readFileSync(process.argv[2],'utf8'));
const device='http://'+cfg.device_listen, preview='http://'+cfg.preview_listen;
if ((cfg.transport||'usb')==='usb') {
 const statusResponse=await fetch(preview+'/status');assert.equal(statusResponse.status,200);
 const status=await statusResponse.json();assert(status.usb);assert.equal(status.sha256.length,64);
 const png=await fetch(preview+'/frame.png');assert.equal(png.status,200);
 const b=Buffer.from(await png.arrayBuffer());assert.equal(b.readUInt32BE(16),1404);assert.equal(b.readUInt32BE(20),1872);
 assert.equal((await fetch(preview+'/status',{method:'POST'})).status,405);
 const notesResponse=await fetch(preview+'/api/notes');assert.equal(notesResponse.status,200);
 const notes=await notesResponse.json();assert.equal(notes.notes.length,4);assert.equal(notes.revision.length,64);
 assert.equal((await fetch(preview+'/api/notes',{method:'POST',headers:{'Content-Type':'application/json'},body:'{}'})).status,403);
 assert.equal((await fetch(preview+'/v1/frame.bin')).status,404);
 await assert.rejects(fetch(device+'/v1/frame.bin',{signal:AbortSignal.timeout(1000)}));
 console.log('PASS: USB status, portrait preview, notes read, protected edits, read-only status, no LAN frame listener');
 process.exit(0);
}
const headers={Authorization:'Bearer '+cfg.device_token};
assert.equal((await fetch(device+'/v1/frame.bin')).status,401);
assert.equal((await fetch(device+'/status',{headers})).status,404);
const frame=await fetch(device+'/v1/frame.bin',{headers});assert.equal(frame.status,200);
const body=Buffer.from(await frame.arrayBuffer());assert.equal(body.length,1314144);
const hash=createHash('sha256').update(body).digest('hex');assert.equal(frame.headers.get('etag'),'"'+hash+'"');
const metadata=await (await fetch(device+'/v1/frame',{headers})).json();
assert.equal(metadata.width,1872);assert.equal(metadata.height,1404);
assert.equal(metadata.display_width,1404);assert.equal(metadata.display_height,1872);
assert.equal(metadata.orientation,'portrait');assert.equal(metadata.display_to_buffer_rotation_degrees,90);
const unchanged=await fetch(device+'/v1/frame.bin',{headers:{...headers,'If-None-Match':frame.headers.get('etag')}});
assert.equal(unchanged.status,304);assert.equal((await unchanged.arrayBuffer()).byteLength,0);
const status=await (await fetch(preview+'/status')).json();assert.equal(status.sha256,hash);assert(!status.last_device_pull.startsWith('0001'));
const png=await fetch(preview+'/frame.png');assert.equal(png.status,200);
const pngBytes=Buffer.from(await png.arrayBuffer());
assert.equal(pngBytes.readUInt32BE(16),1404);assert.equal(pngBytes.readUInt32BE(20),1872);
assert.equal((await fetch(device+'/v1/frame.bin',{method:'POST',headers})).status,405);
console.log('PASS: portrait PNG/metadata, authentication, route isolation, packed size, SHA256, 304, read-only API and device pull tracking');
