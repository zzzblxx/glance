// Against an isolated demo server only; never saves edits or contacts hardware.
const { chromium } = require(process.env.PLAYWRIGHT_MODULE || 'playwright');
const assert = require('node:assert/strict');
(async () => {
  const browser = await chromium.launch({headless:true});
  try {
    const page = await browser.newPage();
    const errors = [];
    page.on('pageerror', e => errors.push(e.message));
    await page.goto('http://127.0.0.1:18788');
    await page.waitForFunction(() => !document.querySelector('#note-0').disabled);
    await page.evaluate(() => document.fonts.load('26px GlanceNoteLatin', 'Aa09'));
    for (const width of [1440, 1024, 800, 390, 320]) {
      await page.setViewportSize({width,height:1100});
      for (const value of ['一二三四五六七'.repeat(4), 'a'.repeat(56), 'iiiiWWWW123456'.repeat(4), '中文ABC中文123中文ABC中文123', '一二三四五六，七八九。十', '一二三四五六七\n八九十']) {
        for (let i=0;i<4;i++) {
          await page.locator('#note-'+i).fill(value);
          const rows = await page.locator('#note-'+i).evaluate(el => {
            const cs = getComputedStyle(el), mirror = document.createElement('div');
            for (const k of ['boxSizing','width','padding','border','font','letterSpacing','whiteSpace','wordBreak','lineBreak','overflowWrap']) mirror.style[k] = cs[k];
            mirror.style.position='absolute'; mirror.style.left='0'; mirror.style.top='0';
            for (const ch of el.value) { const s=document.createElement('span'); s.textContent=ch; mirror.append(s); }
            document.body.append(mirror);
            const lines=[];
            let y=null;
            for (const s of mirror.children) {
              if(s.textContent==='\n') continue;
              const top=s.getBoundingClientRect().top;
              if(y!==top) {lines.push('');y=top;}
              lines[lines.length-1]+=s.textContent;
            }
            mirror.remove(); return lines;
          });
          let expected=[''], units=0;
          for (const ch of value) {
            if(ch==='\n') {expected.push('');units=0;continue;}
            const n=ch.codePointAt(0)<128?1:2;
            if(units+n>14) {expected.push('');units=0;}
            expected[expected.length-1]+=ch;units+=n;
          }
          assert.deepEqual(rows,expected,`wrap at viewport ${width}, note ${i}, text ${value}`);
        }
      }
      assert.equal(await page.evaluate(()=>document.documentElement.scrollWidth<=innerWidth),true,`page overflow at ${width}`);
    }
    await page.setViewportSize({width:1440,height:1100});
    for(let i=0;i<4;i++) await page.locator('#note-'+i).fill('一二三四五六七八九十'.repeat(3).slice(0,28));
    await page.locator('#notes-form').screenshot({path:'var/notes-editor-seven-columns.png'});
    assert.deepEqual(errors,[]);
    console.log('PASS: all four inputs match seven-CJK / fourteen-ASCII wrapping, punctuation, mixed text and newlines at five viewport sizes.');
  } finally { await browser.close(); }
})().catch(e=>{console.error(e);process.exit(1)});
