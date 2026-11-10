// Runs the unmodified pinned lookup and report writer against a loopback OCI fixture.
// External networking is disabled by the container test; no GitHub credential is used.
import assert from 'node:assert/strict';
import { createServer } from 'node:http';
import { once } from 'node:events';
import { getConfig } from '/usr/local/renovate/dist/config/defaults.js';
import { GlobalConfig } from '/usr/local/renovate/dist/config/global.js';
import { add as addHostRule } from '/usr/local/renovate/dist/util/host-rules.js';
import { lookupUpdates } from '/usr/local/renovate/dist/workers/repository/process/lookup/index.js';
import { addExtractionStats, addRepositoryStatus, exportStats } from '/usr/local/renovate/dist/instrumentation/reporting.js';
import { readFile } from 'node:fs/promises';
const calls=[];
const cases={current:['1.0.0'],upgrade:['1.0.0','2.0.0'],filtered:['1.0.0','2.0.0'],unusable:['edge','stable'],pin_missing:['1.0.0'],missing:null};
const server=createServer((req,res)=>{
 calls.push(req.url);
 const path=new URL(req.url,'http://localhost').pathname;
 const name=path.split('/')[2];
 if(path==='/v2/' || path==='/v2'){res.writeHead(200);res.end('{}');return;}
 if(path.endsWith('/tags/list') && cases[name]){res.setHeader('Content-Type','application/json');res.end(JSON.stringify({name,tags:cases[name]}));return;}
 if(path.includes('/manifests/') && cases[name] && name!=='pin_missing'){res.setHeader('Content-Type','application/vnd.oci.image.manifest.v1+json');res.setHeader('Docker-Content-Digest','sha256:'+'a'.repeat(64));res.end(JSON.stringify({schemaVersion:2,config:{mediaType:'application/vnd.oci.image.config.v1+json',digest:'sha256:'+'b'.repeat(64),size:2},layers:[]}));return;}
 if(path.includes('/blobs/')){res.setHeader('Content-Type','application/json');res.end(JSON.stringify({created:'2025-01-01T00:00:00Z',config:{Labels:{}}}));return;}
 res.writeHead(404);res.end('{}');
});
server.listen(0,'127.0.0.1');await once(server,'listening');
const host='127.0.0.1:'+server.address().port;
GlobalConfig.set({platform:'local',localDir:'/tmp',cacheDir:'/tmp/cache',internalHostAccess:'allow'});
addHostRule({matchHost:host,hostType:'docker',insecureRegistry:true});
const rows=[];const receipt=[];
try{
 for(const name of Object.keys(cases)){
  const config={...getConfig(),manager:'dockerfile',datasource:'docker',versioning:'docker',depName:name,packageName:host+'/'+name,currentValue:'1.0.0',registryUrls:['http://'+host],packageRules:[],pinDigests:false,fetchChangeLogs:'off',updatePinnedDependencies:true,rangeStrategy:'replace',minimumReleaseAge:null};
  if(name==='filtered')config.allowedVersions='<=1.0.0';
  if(name==='pin_missing')config.pinDigests=true;
  const before=calls.length;
  const result=(await lookupUpdates(config)).unwrapOrThrow();
  const summary={name,registryCalls:calls.slice(before),updates:result.updates?.map(u=>({newValue:u.newValue,updateType:u.updateType})),warnings:result.warnings,skipReason:result.skipReason??null,currentVersion:result.currentVersion??null};
  receipt.push(summary);
  rows.push({depName:config.depName,packageName:config.packageName,datasource:'docker',currentValue:'1.0.0',...result});
 }
 assert.equal(receipt[0].updates.length,0);assert.equal(receipt[0].warnings.length,0);assert.equal(receipt[0].skipReason,null);
 assert.equal(receipt[1].updates.length,1);
 for(const name of ['filtered','unusable','pin_missing']){
  const r=receipt.find(r=>r.name===name);assert.equal(r.updates.length,0);assert.equal(r.warnings.length,0);assert.equal(r.skipReason,null);
 }
 assert.ok(receipt.find(r=>r.name==='missing').warnings.length>0);
 const reportConfig={reportType:'file',reportPath:'/tmp/empty-report.json',repository:'example/empty-proof',reportFormatting:false};
 addRepositoryStatus(reportConfig,'onboarded');addExtractionStats(reportConfig,{packageFiles:{dockerfile:[{packageFile:'Dockerfile',deps:rows}]}});await exportStats(reportConfig);
 const report=JSON.parse(await readFile('/tmp/empty-report.json','utf8'));
 const serialized=report.repositories['example/empty-proof'].packageFiles.dockerfile[0].deps;
 for(const row of serialized.filter(d=>['current','filtered','unusable','pin_missing'].includes(d.depName))){assert.equal(row.updates.length,0);assert.equal(row.warnings.length,0);assert.equal(row.skipReason,undefined);}
 console.log('PROOF:'+JSON.stringify({passed:true,renovate:'44.197.0',nativeReportSerialized:true,network:'container has no external network; loopback fixture only',cases:receipt}));
}finally{server.closeAllConnections();await new Promise(resolve=>server.close(resolve));}
