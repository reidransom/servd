(() => {
  const nav = document.getElementById('site-nav');
  const toggle = document.getElementById('menu-toggle');
  if (!nav || !toggle) return;
  const desktop = matchMedia('(min-width: 50rem)');
  const covered = [
    document.querySelector('header'),
    document.getElementById('main-content'),
    document.getElementById('toc-slot'),
  ];
  const groups = Array.from(nav.querySelectorAll('details'));
  let lastFocus = null;
  document.addEventListener('focusin', (event) => {
    lastFocus = event.target === toggle || nav.contains(event.target) ? event.target : null;
  });
  document.addEventListener(
    'pointerdown',
    (event) => {
      if (event.target !== toggle && !nav.contains(event.target)) lastFocus = null;
    },
    true,
  );
  const signature = JSON.stringify(
    Array.from(nav.querySelectorAll('a, summary'), (el) => [
      el.textContent.trim(),
      el.getAttribute('href'),
    ]),
  );
  const storageKey = 'starlyt-sidebar:' + nav.dataset.base;
  try {
    const saved = JSON.parse(sessionStorage.getItem(storageKey) || 'null');
    if (saved?.signature === signature && Array.isArray(saved.open)) {
      groups.forEach((group, index) => {
        if (typeof saved.open[index] === 'boolean') group.open = saved.open[index];
      });
      if (Number.isFinite(saved.scroll)) nav.scrollTop = saved.scroll;
    }
  } catch {
    /* Storage can be unavailable; native disclosures still work. */
  }
  const save = () => {
    try {
      sessionStorage.setItem(
        storageKey,
        JSON.stringify({
          signature,
          open: groups.map((group) => group.open),
          scroll: nav.scrollTop,
        }),
      );
    } catch {
      /* Session-only enhancement. */
    }
  };
  groups.forEach((group) => group.addEventListener('toggle', save));
  document.addEventListener('visibilitychange', () => {
    if (document.hidden) save();
  });
  const update = () => {
    const open = nav.matches(':popover-open') && !desktop.matches;
    toggle.setAttribute('aria-expanded', String(open));
    covered.forEach((element) => {
      if (element) element.inert = open;
    });
    if (open) {
      document.querySelector('#toc details')?.removeAttribute('open');
      nav.querySelector('a')?.focus();
    }
  };
  nav.addEventListener('toggle', update);
  update();
  nav.addEventListener('click', (event) => {
    if (
      event.target instanceof Element &&
      event.target.closest('a') &&
      nav.matches(':popover-open')
    )
      nav.hidePopover();
  });
  let desktopState = desktop.matches;
  const updateBreakpoint = () => {
    if (desktop.matches === desktopState) return;
    desktopState = desktop.matches;
    const currentFocus =
      document.activeElement === toggle || nav.contains(document.activeElement)
        ? document.activeElement
        : lastFocus;
    const focusWasInMenu = nav.contains(currentFocus);
    const focusWasOnToggle = currentFocus === toggle;
    let focusTarget = null;
    if (!desktop.matches && focusWasInMenu) focusTarget = toggle;
    if (desktop.matches && (focusWasInMenu || focusWasOnToggle))
      focusTarget = nav.querySelector('a');
    const expectedDesktop = desktop.matches;
    if (nav.matches(':popover-open')) nav.hidePopover();
    update();
    if (focusTarget instanceof HTMLElement)
      requestAnimationFrame(() => {
        if (desktop.matches === expectedDesktop) focusTarget.focus();
      });
  };
  desktop.addEventListener('change', updateBreakpoint);
  new ResizeObserver(updateBreakpoint).observe(document.documentElement);
  document.addEventListener('keydown', (event) => {
    if (!nav.matches(':popover-open') || desktop.matches) return;
    if (event.key === 'Escape') {
      event.preventDefault();
      nav.hidePopover();
      update();
      toggle.focus();
    } else if (event.key === 'Tab') {
      const focusable = [toggle, ...nav.querySelectorAll('a, summary, select, button')].filter(
        (el) => el instanceof HTMLElement && el.getClientRects().length,
      );
      const index = focusable.indexOf(document.activeElement);
      const next = event.shiftKey
        ? (index - 1 + focusable.length) % focusable.length
        : (index + 1) % focusable.length;
      event.preventDefault();
      const target = focusable[next];
      if (target instanceof HTMLElement) target.focus();
    }
  });
})();
