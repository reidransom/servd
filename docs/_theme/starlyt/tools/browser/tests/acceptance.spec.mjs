import { expect, test } from '@playwright/test';
import { installations, modes, pages, urlFor, viewports } from '../matrix.mjs';

const waitForLayout = async (page) => {
  await page.evaluate(async () => {
    await document.fonts.ready;
    await new Promise((resolve) => requestAnimationFrame(() => requestAnimationFrame(resolve)));
  });
};

for (const installation of installations) {
  for (const pageEntry of pages) {
    for (const mode of modes) {
      for (const viewport of viewports) {
        test(`${installation.name} ${pageEntry.name} ${mode} ${viewport.name} contains the layout`, async ({ page }) => {
          await page.setViewportSize(viewport);
          await page.addInitScript((selectedMode) => {
            localStorage.setItem('starlight-theme', selectedMode);
          }, mode);
          await page.goto(urlFor(installation, pageEntry));
          await waitForLayout(page);

          await expect(page.locator('html')).toHaveAttribute('data-theme', mode);
          const geometry = await page.evaluate(() => {
            const root = document.documentElement;
            const header = document.querySelector('header');
            const title = document.getElementById('page-title');
            const article = document.getElementById('article');
            const headerRect = header?.getBoundingClientRect();
            const titleRect = title?.getBoundingClientRect();
            const articleRect = article?.getBoundingClientRect();
            const localOverflow = Array.from(document.querySelectorAll('#article pre, #article table')).map((element) => {
              const rect = element.getBoundingClientRect();
              return {
                tag: element.tagName,
                clientWidth: element.clientWidth,
                scrollWidth: element.scrollWidth,
                overflowX: getComputedStyle(element).overflowX,
                left: rect.left,
                right: rect.right,
              };
            });
            const popoverOpen = document.getElementById('site-nav')?.matches(':popover-open') ?? false;
            return {
              viewportWidth: innerWidth,
              pageWidth: root.scrollWidth,
              headerBottom: headerRect?.bottom ?? 0,
              titleTop: titleRect?.top ?? -1,
              articleLeft: articleRect?.left ?? -1,
              articleRight: articleRect?.right ?? innerWidth + 1,
              popoverOpen,
              mainInert: document.getElementById('main-content')?.inert ?? false,
              localOverflow,
            };
          });

          expect(geometry.pageWidth, 'page-wide horizontal overflow').toBeLessThanOrEqual(geometry.viewportWidth + 1);
          expect(geometry.titleTop, 'title hidden behind fixed header').toBeGreaterThanOrEqual(geometry.headerBottom - 1);
          expect(geometry.articleLeft, 'article starts outside viewport').toBeGreaterThanOrEqual(-1);
          expect(geometry.articleRight, 'article ends outside viewport').toBeLessThanOrEqual(geometry.viewportWidth + 1);
          expect(geometry.popoverOpen, 'stale navigation overlay').toBe(false);
          expect(geometry.mainInert, 'content left inert').toBe(false);
          for (const local of geometry.localOverflow) {
            expect(local.left, `${local.tag} starts outside viewport`).toBeGreaterThanOrEqual(-1);
            expect(local.right, `${local.tag} ends outside viewport`).toBeLessThanOrEqual(geometry.viewportWidth + 1);
            if (local.scrollWidth > local.clientWidth + 1) {
              expect(['auto', 'scroll'], `${local.tag} wide content must scroll locally`).toContain(local.overflowX);
            }
          }

          const headerControl = viewport.width < 800 ? page.locator('#menu-toggle') : page.locator('header .site-title');
          await expect(headerControl).toBeVisible();
          const controlRect = await headerControl.boundingBox();
          expect(controlRect).not.toBeNull();
          expect(controlRect.x).toBeGreaterThanOrEqual(0);
          expect(controlRect.x + controlRect.width).toBeLessThanOrEqual(viewport.width + 1);
        });
      }
    }
  }

  test(`${installation.name} mobile navigation traps focus and restores it on Escape`, async ({ page }) => {
    await page.setViewportSize({ width: 390, height: 844 });
    await page.goto(urlFor(installation, pages[0]));
    await page.locator('#menu-toggle').focus();
    await page.keyboard.press('Enter');
    await expect(page.locator('#site-nav')).toHaveCSS('display', 'block');
    await expect.poll(() => page.locator('#site-nav').evaluate((nav) => nav.matches(':popover-open'))).toBe(true);
    await expect.poll(() => page.evaluate(() => document.getElementById('site-nav')?.contains(document.activeElement))).toBe(true);
    expect(await page.evaluate(() => document.activeElement?.matches(':focus-visible'))).toBe(true);
    await expect.poll(() => page.locator('#main-content').evaluate((main) => main.inert)).toBe(true);

    await page.keyboard.press('Escape');
    await expect.poll(() => page.locator('#site-nav').evaluate((nav) => nav.matches(':popover-open'))).toBe(false);
    await expect(page.locator('#menu-toggle')).toBeFocused();
    await expect.poll(() => page.locator('#main-content').evaluate((main) => main.inert)).toBe(false);
  });

  test(`${installation.name} navigation breakpoint removes overlay state`, async ({ page }) => {
    await page.setViewportSize({ width: 799, height: 768 });
    await page.goto(urlFor(installation, pages[0]));
    await page.locator('#menu-toggle').click();
    await expect.poll(() => page.locator('#site-nav').evaluate((nav) => nav.matches(':popover-open'))).toBe(true);
    await page.setViewportSize({ width: 800, height: 768 });
    await expect.poll(() => page.locator('#site-nav').evaluate((nav) => nav.matches(':popover-open'))).toBe(false);
    await expect(page.locator('#site-nav')).toBeVisible();
    await expect(page.locator('#site-nav a').first()).toBeFocused();
    await page.setViewportSize({ width: 799, height: 768 });
    await expect.poll(() => page.evaluate(() => matchMedia('(min-width: 50rem)').matches)).toBe(false);
    await expect(page.locator('#menu-toggle')).toBeFocused();
    await expect.poll(() => page.locator('#main-content').evaluate((main) => main.inert)).toBe(false);
  });

  test(`${installation.name} global controls move without duplicate focus`, async ({ page }) => {
    await page.setViewportSize({ width: 800, height: 768 });
    await page.goto(urlFor(installation, pages[0]));
    await expect(page.locator('header #color-mode')).toBeVisible();
    await page.locator('#color-mode').focus();
    await page.setViewportSize({ width: 799, height: 768 });
    await expect(page.locator('#menu-toggle')).toBeFocused();
    await expect(page.locator('#color-mode')).toHaveCount(1);
    await page.locator('#menu-toggle').click();
    await expect(page.locator('#site-nav #color-mode')).toBeVisible();
    await page.setViewportSize({ width: 800, height: 768 });
    await expect(page.locator('header #color-mode')).toBeVisible();
    await expect(page.locator('#color-mode')).toHaveCount(1);
    await expect.poll(() => page.locator('#main-content').evaluate((main) => main.inert)).toBe(false);
  });

  test(`${installation.name} disclosure state survives reload`, async ({ page }) => {
    await page.setViewportSize({ width: 1024, height: 768 });
    await page.goto(urlFor(installation, pages[0]));
    const disclosure = page.locator('#site-nav details').first();
    const initial = await disclosure.evaluate((details) => details.open);
    await disclosure.locator('summary').click();
    await expect.poll(() => disclosure.evaluate((details) => details.open)).toBe(!initial);
    await page.reload();
    await expect.poll(() => disclosure.evaluate((details) => details.open)).toBe(!initial);
  });

  test(`${installation.name} TOC navigates and cleans up at its breakpoint`, async ({ page }) => {
    await page.setViewportSize({ width: 1151, height: 900 });
    await page.goto(urlFor(installation, pages[1]));
    const details = page.locator('#toc details');
    await expect(details).not.toHaveAttribute('open', '');
    await details.locator('summary').click();
    const link = page.locator('#toc a[data-toc-target="maintain-the-examples"]');
    await link.click();
    await expect(page).toHaveURL(/#maintain-the-examples$/);
    await expect(page.locator('#maintain-the-examples')).toBeFocused();
    const position = await page.locator('#maintain-the-examples').evaluate((heading) => ({
      top: heading.getBoundingClientRect().top,
      headerBottom: document.querySelector('header').getBoundingClientRect().bottom,
    }));
    expect(position.top).toBeGreaterThanOrEqual(position.headerBottom - 1);

    await details.locator('summary').click();
    await details.locator('summary').focus();
    await page.setViewportSize({ width: 1152, height: 900 });
    await expect.poll(() => page.evaluate(() => matchMedia('(min-width: 72rem)').matches)).toBe(true);
    await expect(details).toHaveAttribute('open', '');
    await expect(page.locator('#toc a').first()).toBeFocused();
    await page.setViewportSize({ width: 1151, height: 900 });
    await expect.poll(() => page.evaluate(() => matchMedia('(min-width: 72rem)').matches)).toBe(false);
    await expect(details).not.toHaveAttribute('open', '');
    await expect(details.locator('summary')).toBeFocused();
  });

  test(`${installation.name} color preference persists`, async ({ page }) => {
    await page.setViewportSize({ width: 1024, height: 768 });
    await page.goto(urlFor(installation, pages[0]));
    await page.locator('#color-mode').selectOption('light');
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
    await expect.poll(() => page.evaluate(() => localStorage.getItem('starlight-theme'))).toBe('light');
    await page.reload();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'light');
    await page.locator('#color-mode').selectOption('dark');
    await page.reload();
    await expect(page.locator('html')).toHaveAttribute('data-theme', 'dark');
  });

  test(`${installation.name} clipboard reports exact success and honest denial`, async ({ browser }) => {
    const successContext = await browser.newContext({ viewport: { width: 1024, height: 768 } });
    await successContext.addInitScript(() => {
      Object.defineProperty(navigator, 'clipboard', {
        configurable: true,
        value: { writeText: async (text) => { window.__copiedText = text; } },
      });
    });
    const successPage = await successContext.newPage();
    await successPage.goto(urlFor(installation, pages[2]));
    const source = await successPage.locator('#article code').first().textContent();
    await successPage.locator('.copy-button').first().click();
    await expect(successPage.locator('.copy-feedback').first()).toHaveText('Copied!');
    await expect.poll(() => successPage.evaluate(() => window.__copiedText)).toBe(source);
    await successContext.close();

    const deniedContext = await browser.newContext({ viewport: { width: 1024, height: 768 } });
    await deniedContext.addInitScript(() => {
      Object.defineProperty(navigator, 'clipboard', {
        configurable: true,
        value: { writeText: async () => { throw new DOMException('Denied', 'NotAllowedError'); } },
      });
    });
    const deniedPage = await deniedContext.newPage();
    await deniedPage.goto(urlFor(installation, pages[2]));
    await deniedPage.locator('.copy-button').first().click();
    const feedback = deniedPage.locator('.copy-feedback').first();
    await expect(feedback).toHaveText('Copy failed');
    await expect(feedback).toHaveAttribute('data-error', '');
    await deniedContext.close();
  });

  test(`${installation.name} remains readable without JavaScript`, async ({ browser }) => {
    const context = await browser.newContext({ javaScriptEnabled: false, viewport: { width: 390, height: 844 } });
    const page = await context.newPage();
    await page.goto(urlFor(installation, pages[0]));
    await expect(page.locator('#article')).toContainText('Documentation workflow');
    await expect(page.locator('#article img')).toBeVisible();
    await expect(page.locator('.tab-panel')).toHaveCount(2);
    for (const panel of await page.locator('.tab-panel').all()) await expect(panel).toBeVisible();
    await page.locator('#menu-toggle').click();
    await expect(page.locator('#site-nav a').first()).toBeVisible();
    await page.goto(urlFor(installation, pages[2]));
    await expect(page.locator('#article code').first()).toContainText('console.log(greet("Starlyt"));');
    await expect(page.locator('.copy-button')).toHaveCount(0);
    await context.close();
  });
}
