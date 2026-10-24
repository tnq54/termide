const {test, expect} = require('@playwright/test');
const {pathToFileURL} = require('node:url');
const path = require('node:path');
const app = pathToFileURL(path.resolve(__dirname, '../app/src/main/assets/index.html')).href;
test.use({viewport:{width:390,height:844}});

test('offline demo renders Unicode and accepts touch control keys', async ({page}) => {
  const errors=[];
  page.on('pageerror', e=>errors.push(e.message));
  await page.goto(app);
  await page.waitForLoadState('networkidle');
  await page.screenshot({path:'test-results/connection.png',fullPage:true});
  await page.locator('#demo').click();
  await page.keyboard.type('unicode');
  await page.keyboard.press('Enter');
  await expect.poll(()=>page.evaluate(()=>{
    const b=term.buffer.active;
    return Array.from({length:b.length},(_,i)=>b.getLine(i).translateToString()).join('\n');
  })).toContain('Tiếng Việt');
  await page.locator('[data-key=save]').click();
  await expect(page.locator('#status')).toContainText('Ctrl+S nhận thành công');
  await page.setViewportSize({width:390,height:430});
  await expect.poll(()=>page.evaluate(()=>term.rows)).toBeLessThan(20);
  const terminal=await page.locator('#terminal').boundingBox();
  expect(terminal.height).toBeGreaterThan(100);
  await page.screenshot({path:'test-results/terminal-keyboard-size.png'});
  expect(errors).toEqual([]);
});

test('SSH form sends credentials once, clears secrets and preserves UTF8 frames', async ({page}) => {
  await page.addInitScript(()=>{
    window.sent=[]; window.inputs=[]; window.sizes=[];
    window.Native={connect:p=>sent.push(JSON.parse(p)),send:s=>inputs.push(s),resize:(c,r)=>sizes.push([c,r]),disconnect(){},importKey(){},clearKey(){}};
  });
  await page.goto(app);
  await page.locator('#host').fill('example.org');
  await page.locator('#user').fill('dev');
  await page.locator('#project').fill('/srv/test project');
  await page.locator('#password').fill('test-only-secret');
  await page.locator('.primary').click();
  expect(await page.evaluate(()=>sent[0].project)).toBe('/srv/test project');
  expect(await page.locator('#password').inputValue()).toBe('');
  await page.locator('#ctrl').click();
  await page.keyboard.type('s');
  expect(await page.evaluate(()=>inputs.at(-1))).toBe('\x13');
  // Deliberately split a multibyte character across transport chunks.
  await page.evaluate(()=>{mode='ssh';terminalOutput('8J8=');terminalOutput('mIA=');});
  await expect.poll(()=>page.evaluate(()=>term.buffer.active.getLine(0).translateToString())).toContain('😀');
  await page.setViewportSize({width:640,height:360});
  await expect.poll(()=>page.evaluate(()=>sizes.length)).toBeGreaterThan(0);
});
