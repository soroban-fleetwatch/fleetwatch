// Deterministic Chromium integration test. Requires Playwright + Chromium;
// never disables TLS verification and never contacts the live RPC endpoint.
'use strict';
const assert=require('node:assert/strict');
const fs=require('node:fs');
const http=require('node:http');
const path=require('node:path');
const {chromium}=require('playwright');

(async()=>{
  if(process.argv.length!==3)throw new Error('usage: node scripts/check-browser.cjs PATH_TO_FLEETWATCH_HTML');
  const app=fs.readFileSync(process.argv[2]);
  const fixture=JSON.parse(fs.readFileSync(path.join(__dirname,'../testdata/browser-rpc-entries.json')));
  const server=http.createServer((_req,res)=>{res.writeHead(200,{'Content-Type':'text/html'});res.end(app)});
  await new Promise(resolve=>server.listen(0,'127.0.0.1',resolve));
  let browser;
  try{
    browser=await chromium.launch({executablePath:process.env.CHROMIUM_PATH||'/usr/bin/chromium',headless:true,args:process.getuid?.()===0?['--no-sandbox']:[]});
    const page=await browser.newPage({acceptDownloads:true});
    const pageErrors=[];page.on('pageerror',e=>pageErrors.push(e.message));
    await page.addInitScript(()=>{window.captureFetchOptions=[];const original=window.fetch;window.fetch=function(url,options){window.captureFetchOptions.push({url:String(url),mode:options.mode,credentials:options.credentials,redirect:options.redirect});return original.apply(this,arguments)}});
    let mode='valid', calls=0;
    await page.route('https://soroban-testnet.stellar.org/**',async route=>{
      calls++;
      const req=route.request().postDataJSON();
      if(mode==='redirect'){await route.fulfill({status:302,headers:{location:'https://example.invalid/blocked'}});return}
      let result;
      if(req.method==='getNetwork')result={passphrase:mode==='wrong-network'?'wrong':fixture.network,protocolVersion:29};
      else if(req.method==='getLatestLedger')result={sequence:fixture.ledger,protocolVersion:29,closeTime:String(Math.floor(Date.now()/1000))};
      else if(req.method==='getLedgerEntries')result={latestLedger:fixture.ledger,entries:mode==='missing'?[]:fixture.entries.filter(e=>req.params.keys.includes(e.key)).map(e=>mode==='bad-xdr'?{...e,xdr:'bad-base64'}:e)};
      else throw new Error('unexpected RPC method');
      await route.fulfill({contentType:'application/json',body:JSON.stringify({jsonrpc:'2.0',id:req.id,result})});
    });
    await page.goto('http://127.0.0.1:'+server.address().port+'/FleetWatch.html');
    await page.waitForFunction(()=>document.querySelector('#status').textContent==='Partial diagnostic observation collected.',null,{timeout:30000});
    await page.locator('#auto').uncheck();
    let raw=JSON.parse(await page.locator('#raw').textContent());
    assert.equal(raw.schema_version,2);assert.equal(raw.snapshot.contracts.length,2);assert.equal(raw.source_authentication,'UNVERIFIED');assert.equal(raw.history_completeness,'UNKNOWN');assert.equal(raw.completeness,'PARTIAL');assert.equal(raw.snapshot.references[0].tag_hex,'666c65657477617463682d64656d6f');
    const opts=await page.evaluate(()=>window.captureFetchOptions);
    assert(opts.length>=4);assert(opts.every(o=>o.mode==='cors'&&o.credentials==='omit'&&o.redirect==='error'));
    const downloadPromise=page.waitForEvent('download');await page.locator('#export').click();const download=await downloadPromise;
    const downloaded=JSON.parse(fs.readFileSync(await download.path()));assert.deepEqual(downloaded,raw);
    const callsBefore=calls;await page.locator('#contracts').fill('INVALID_ADDRESS');await page.locator('#refresh').click();await page.waitForFunction(()=>document.querySelector('#status').textContent.includes('invalid Stellar contract address'));
    assert.equal(calls,callsBefore);assert(await page.locator('#export').isDisabled());assert.equal(await page.locator('#values').textContent(),'');
    await page.locator('#contracts').fill(fixture.watchlist.join('\n'));
    mode='missing';await page.locator('#refresh').click();await page.waitForFunction(()=>document.querySelector('#status').textContent==='Partial diagnostic observation collected.');raw=JSON.parse(await page.locator('#raw').textContent());assert.equal(raw.snapshot.contracts.length,0);assert(raw.errors.some(e=>e.startsWith('contract instance not observed:')));
    mode='bad-xdr';await page.locator('#refresh').click();await page.waitForFunction(()=>document.querySelector('#status').textContent==='Partial diagnostic observation collected.');raw=JSON.parse(await page.locator('#raw').textContent());assert.equal(raw.snapshot.contracts.length,0);assert(raw.errors.length>0);
    mode='wrong-network';await page.locator('#refresh').click();await page.waitForFunction(()=>document.querySelector('#status').textContent.includes('network passphrase mismatch'));assert((await page.locator('#time').textContent()).includes('stale'));
    let followed=false;await page.route('https://example.invalid/**',route=>{followed=true;return route.abort()});
    mode='redirect';await page.locator('#refresh').click();await page.waitForFunction(()=>document.querySelector('#status').textContent.includes('transport failed or timed out'));assert.equal(followed,false);
    assert.deepEqual(pageErrors,[]);
    console.log(JSON.stringify({browser:await browser.version(),playwright:require('playwright/package.json').version,tests:['typed-external-reference-capture','assurance-preserved','fetch-credentials-omitted','redirect-blocked-before-follow','raw-evidence-download','watchlist-validation-no-rpc','missing-entry-partial','malformed-xdr-partial','wrong-network-rejected','stale-values-labelled'],status:'PASS'}));
  }finally{if(browser)await browser.close();await new Promise(resolve=>server.close(resolve))}
})().catch(error=>{console.error(error);process.exitCode=1});
