// Run app.py --no-open --port 8792. Only fictional mail is imported.
const {chromium,firefox,webkit}=require('playwright');
const {default:AxeBuilder}=require('@axe-core/playwright');
const assert=require('node:assert/strict');
(async()=>{
 for(const [name,type] of Object.entries({chromium,firefox,webkit})){
  const browser=await type.launch({headless:true});
  try{
   const context=await browser.newContext(),page=await context.newPage(),errors=[],external=[];
   page.on('pageerror',error=>errors.push(error.message));
   context.on('request',request=>{if(!request.url().startsWith('http://127.0.0.1:8792')&&!request.url().startsWith('data:'))external.push(request.url())});
   await page.goto(process.env.POSTBOX_URL||'http://127.0.0.1:8792');
   assert.equal(await page.locator('#deleteMode').inputValue(),'session');
   assert.equal(await page.locator('header').evaluate(e=>getComputedStyle(e).color),'rgb(255, 255, 255)');
   assert.match(await page.locator('header').evaluate(e=>getComputedStyle(e).backgroundImage),/hero-purple/);
   await page.keyboard.press('Tab');assert.equal(await page.evaluate(()=>document.activeElement.textContent),'Skip to messages');
   await page.keyboard.press('Enter');assert.equal(await page.evaluate(()=>document.activeElement.id),'workspace');
   const fixture='From: Ada <ada@example.invalid>\r\nSubject: Private synthetic subject\r\nMIME-Version: 1.0\r\nContent-Type: multipart/mixed; boundary=test\r\n\r\n--test\r\nContent-Type: text/html\r\n\r\n<p>Private synthetic body</p><script>window.alert(1)</script><style>@import url(https://example.invalid/css);p{background:url(https://example.invalid/pixel)}</style><img src="https://example.invalid/tracker"><a href="https://example.invalid/link">link</a>\r\n--test\r\nContent-Type: text/plain\r\nContent-Disposition: attachment; filename=proof.txt\r\n\r\nPrivate synthetic attachment\r\n--test--\r\n';
   await page.locator('#files').setInputFiles({name:'fixture.eml',mimeType:'message/rfc822',buffer:Buffer.from(fixture)});
   await page.locator('#subject').filter({hasText:'Private synthetic subject'}).waitFor();
   await page.locator('#search').fill('body:synthetic has:attachment');assert.match(await page.locator('#count').textContent(),/1 of 1/);
   await page.locator('#attachments button').click();await page.locator('#previewContent pre').waitFor();
   assert.match(await page.locator('#previewContent').textContent(),/Private synthetic attachment/);
   await page.keyboard.press('Escape');assert.equal(await page.locator('#previewDialog').evaluate(e=>e.open),false);
   const accessibility=await new AxeBuilder({page}).withTags(['wcag2a','wcag2aa','wcag21aa','wcag22aa']).analyze();
   assert.deepEqual(accessibility.violations.map(v=>({id:v.id,nodes:v.nodes.map(n=>n.target)})),[],`${name} accessibility`);
   for(const width of [1280,640,320]){await page.setViewportSize({width,height:900});const sizes=await page.evaluate(()=>({width:innerWidth,scroll:document.documentElement.scrollWidth}));assert(sizes.scroll<=sizes.width+1,`${name} reflow ${JSON.stringify(sizes)}`)}
   await page.setViewportSize({width:640,height:900});await page.addStyleTag({content:'html{font-size:200%}'});const sizes=await page.evaluate(()=>({width:innerWidth,scroll:document.documentElement.scrollWidth}));assert(sizes.scroll<=sizes.width+1,`${name} 200% text reflow`);
   await page.locator('#clear').click();await page.locator('#count').filter({hasText:'0 of 0'}).waitFor();
   assert.equal(await page.locator('#body').textContent(),'');assert.equal(await page.locator('#subject').textContent(),'');assert.equal(await page.locator('#meta').textContent(),'');assert.equal(await page.locator('#previewContent').textContent(),'');assert.equal(await page.locator('#search').inputValue(),'');
   assert(!await page.locator('body').textContent().then(t=>t.includes('Private synthetic')));
   assert.deepEqual(external,[],`${name} outbound requests`);assert.deepEqual(errors,[],`${name} browser errors`);
   console.log(`${name}: import, search, sandbox/zero outbound requests, preview/Escape, keyboard skip link, axe, 320px/200% reflow and complete Clear passed.`);
  }finally{await browser.close()}
 }
})().catch(e=>{console.error(e);process.exit(1)});
