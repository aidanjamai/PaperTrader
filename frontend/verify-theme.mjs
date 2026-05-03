import { chromium } from 'playwright';
import fs from 'fs';
import path from 'path';

const BASE_URL = 'http://localhost:3000';
const SHOT_DIR = path.join(process.cwd(), '.screenshots', 'editorial-theme');

if (!fs.existsSync(SHOT_DIR)) fs.mkdirSync(SHOT_DIR, { recursive: true });

const VIEWPORTS = [
  { name: 'mobile',   width: 375,  height: 812 },
  { name: 'desktop',  width: 1280, height: 900 },
];

const consoleErrors = [];
const networkFails  = [];

async function shot(page, filename) {
  const p = path.join(SHOT_DIR, filename);
  await page.screenshot({ path: p, fullPage: false });
  return p;
}

(async () => {
  const browser = await chromium.launch({ headless: true });

  for (const vp of VIEWPORTS) {
    console.log(`\n======  ${vp.name} (${vp.width}x${vp.height})  ======`);
    const ctx = await browser.newContext({
      viewport: { width: vp.width, height: vp.height },
    });
    const page = await ctx.newPage();

    page.on('console', msg => {
      if (['error', 'warning'].includes(msg.type())) {
        consoleErrors.push(`[${vp.name}] [${msg.type()}] ${msg.text()}`);
      }
    });
    page.on('requestfailed', req => {
      networkFails.push(`[${vp.name}] FAILED: ${req.url()}`);
    });

    // ── 1. Marketing home – light mode ──────────────────────────────────────
    console.log('  → / light mode');
    await page.goto(`${BASE_URL}/`, { waitUntil: 'networkidle' });
    await page.waitForTimeout(800);
    let p = await shot(page, `${vp.name}-home-light.png`);
    console.log(`     saved: ${p}`);

    // Inspect background color and key element colors
    const bodyBg = await page.evaluate(() => window.getComputedStyle(document.body).backgroundColor);
    console.log(`     body bg: ${bodyBg}`);

    // Check for purple (#667eea)
    const hasPurple = await page.evaluate(() => {
      const all = document.querySelectorAll('*');
      for (const el of all) {
        const s = window.getComputedStyle(el);
        const props = ['color', 'backgroundColor', 'borderColor', 'outlineColor', 'fill', 'stroke'];
        for (const p of props) {
          const v = s[p];
          if (v && (v.includes('102, 126, 234') || v.includes('#667eea'))) return el.outerHTML.slice(0, 200);
        }
      }
      return null;
    });
    console.log(`     purple found: ${hasPurple || 'none'}`);

    // Check for navy #0B2E5C
    const navyPresent = await page.evaluate(() => {
      const all = document.querySelectorAll('*');
      for (const el of all) {
        const s = window.getComputedStyle(el);
        const v = s.color;
        if (v && v.includes('11, 46, 92')) return el.outerHTML.slice(0, 200);
      }
      return null;
    });
    console.log(`     navy (0B2E5C) in color: ${navyPresent ? 'YES - ' + navyPresent.slice(0, 80) : 'not found via computed color'}`);

    // Check wordmark
    const wordmark = await page.evaluate(() => {
      const els = [...document.querySelectorAll('*')];
      const wm = els.find(e => e.textContent && e.textContent.trim().startsWith('PaperTrader'));
      return wm ? wm.outerHTML.slice(0, 300) : null;
    });
    console.log(`     wordmark el: ${wordmark ? wordmark.slice(0, 120) : 'NOT FOUND'}`);

    // Check H1
    const h1Text = await page.evaluate(() => {
      const h1 = document.querySelector('h1');
      return h1 ? h1.innerText : 'NO H1';
    });
    console.log(`     H1: "${h1Text}"`);

    // Check hero two-column layout
    const heroCols = await page.evaluate(() => {
      const hero = document.querySelector('.hero, [class*="hero"], section');
      if (!hero) return 'no hero section found';
      const cols = hero.querySelectorAll('[class*="col"], [class*="Col"]');
      return `hero cols count: ${cols.length}`;
    });
    console.log(`     ${heroCols}`);

    // Check mode-bar
    const modeBar = await page.evaluate(() => {
      const els = [...document.querySelectorAll('*')];
      const bar = els.find(e => {
        const t = e.textContent.trim();
        return t.includes('Light') && t.includes('Dark') && e.children.length <= 5;
      });
      return bar ? bar.outerHTML.slice(0, 300) : 'NOT FOUND';
    });
    console.log(`     mode-bar: ${modeBar.slice(0, 100)}`);

    // Check live-card
    const liveCard = await page.evaluate(() => {
      const els = [...document.querySelectorAll('*')];
      const card = els.find(e => e.textContent && e.textContent.includes('Portfolio') && e.textContent.includes('127'));
      return card ? card.outerHTML.slice(0, 300) : 'NOT FOUND';
    });
    console.log(`     live-card: ${liveCard.slice(0, 100)}`);

    // ── 2. Toggle dark mode ──────────────────────────────────────────────────
    if (vp.name === 'desktop') {
      console.log('  → toggling dark mode');
      // Click the Dark button
      const darkBtn = await page.$('button:has-text("Dark"), [class*="mode"] button:has-text("Dark"), [class*="toggle"] button:has-text("Dark")');
      if (darkBtn) {
        await darkBtn.click();
        await page.waitForTimeout(600);
        p = await shot(page, `${vp.name}-home-dark.png`);
        console.log(`     saved: ${p}`);

        const darkBg = await page.evaluate(() => window.getComputedStyle(document.body).backgroundColor);
        console.log(`     dark body bg: ${darkBg}`);

        // Check accent picker appears
        const accentPicker = await page.evaluate(() => {
          const els = [...document.querySelectorAll('*')];
          const picker = els.find(e => {
            const s = window.getComputedStyle(e);
            return e.children.length >= 4 &&
              [...e.children].some(c => {
                const bg = window.getComputedStyle(c).backgroundColor;
                return bg && bg !== 'rgba(0, 0, 0, 0)' && bg !== 'transparent';
              });
          });
          return picker ? `found - ${picker.className} children:${picker.children.length}` : 'NOT FOUND';
        });
        console.log(`     accent picker: ${accentPicker}`);

        // Check for slate blue #7BA3D6 in dark mode
        const slateBlue = await page.evaluate(() => {
          const all = document.querySelectorAll('*');
          for (const el of all) {
            const s = window.getComputedStyle(el);
            const props = ['color', 'backgroundColor', 'borderColor', 'fill'];
            for (const p of props) {
              const v = s[p];
              if (v && v.includes('123, 163, 214')) return `YES on <${el.tagName}>`;
            }
          }
          return 'not found via computed';
        });
        console.log(`     slate blue (7BA3D6): ${slateBlue}`);

        // ── 3. Test accent swatches ──────────────────────────────────────────
        console.log('  → testing accent swatches');
        // Try clicking colored swatches
        const swatchBtns = await page.$$('[class*="swatch"], [class*="accent"] button, [class*="color"] button');
        console.log(`     swatch buttons found: ${swatchBtns.length}`);
        if (swatchBtns.length > 0) {
          // Click first swatch (lime candidate)
          await swatchBtns[0].click();
          await page.waitForTimeout(400);
          p = await shot(page, `${vp.name}-home-dark-swatch1.png`);
          console.log(`     saved: ${p}`);
          if (swatchBtns.length > 2) {
            await swatchBtns[2].click();
            await page.waitForTimeout(400);
            p = await shot(page, `${vp.name}-home-dark-swatch3.png`);
            console.log(`     saved: ${p}`);
          }
        }

        // ── 4. Theme persistence after refresh ───────────────────────────────
        console.log('  → testing dark mode persistence (refresh)');
        await page.reload({ waitUntil: 'networkidle' });
        await page.waitForTimeout(800);
        const afterReloadBg = await page.evaluate(() => window.getComputedStyle(document.body).backgroundColor);
        const htmlClass = await page.evaluate(() => document.documentElement.className);
        const htmlDataTheme = await page.evaluate(() => document.documentElement.getAttribute('data-theme'));
        const localStorageTheme = await page.evaluate(() => localStorage.getItem('theme') || localStorage.getItem('colorMode') || localStorage.getItem('darkMode') || '(no theme key found)');
        console.log(`     after-reload body bg: ${afterReloadBg}`);
        console.log(`     html class: "${htmlClass}"`);
        console.log(`     html data-theme: "${htmlDataTheme}"`);
        console.log(`     localStorage theme: ${localStorageTheme}`);
        p = await shot(page, `${vp.name}-home-dark-persisted.png`);
        console.log(`     saved: ${p}`);
      } else {
        console.log('     Dark button NOT FOUND - cannot toggle');
        // Try to find any toggle
        const allBtns = await page.$$('button');
        const btnTexts = [];
        for (const b of allBtns.slice(0, 20)) {
          btnTexts.push(await b.innerText());
        }
        console.log(`     visible buttons: ${JSON.stringify(btnTexts)}`);
      }
    }

    // ── 5. Login page ────────────────────────────────────────────────────────
    console.log('  → /login');
    await page.goto(`${BASE_URL}/login`, { waitUntil: 'networkidle' });
    await page.waitForTimeout(600);
    p = await shot(page, `${vp.name}-login.png`);
    console.log(`     saved: ${p}`);

    const loginH = await page.evaluate(() => {
      for (const tag of ['h1','h2','h3']) {
        const el = document.querySelector(tag);
        if (el) return `<${tag}> "${el.innerText}"`;
      }
      return 'no heading found';
    });
    console.log(`     login heading: ${loginH}`);

    const googleBtn = await page.$('[class*="google"], button:has-text("Google"), a:has-text("Google")');
    console.log(`     Google button: ${googleBtn ? 'FOUND' : 'NOT FOUND'}`);

    const hasPurpleLogin = await page.evaluate(() => {
      const all = document.querySelectorAll('*');
      for (const el of all) {
        const s = window.getComputedStyle(el);
        for (const p of ['color','backgroundColor']) {
          const v = s[p];
          if (v && v.includes('102, 126, 234')) return el.outerHTML.slice(0, 200);
        }
      }
      return null;
    });
    console.log(`     purple on login: ${hasPurpleLogin || 'none'}`);

    await ctx.close();
  }

  await browser.close();

  console.log('\n======  CONSOLE ERRORS  ======');
  if (consoleErrors.length === 0) {
    console.log('  none');
  } else {
    consoleErrors.forEach(e => console.log(' ', e));
  }

  console.log('\n======  NETWORK FAILURES  ======');
  if (networkFails.length === 0) {
    console.log('  none');
  } else {
    networkFails.forEach(e => console.log(' ', e));
  }

})();
