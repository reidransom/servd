(() => {
  const article = document.getElementById('article');
  const nav = document.getElementById('toc');
  if (!article || !nav) return;
  const details = nav.querySelector('details');
  const summary = nav.querySelector('summary');
  const currentLabel = nav.querySelector('.toc-current');
  const links = Array.from(nav.querySelectorAll('a[data-toc-target]'));
  if (!details || !summary || !currentLabel || !links.length) return;
  const headings = Array.from(article.querySelectorAll('h2[id], h3[id]')).filter(
    (heading) => heading.id && heading.textContent.trim() && !heading.closest('pre, code'),
  );
  const entries = links.map((link) => link.textContent.trim());
  const desktop = matchMedia('(min-width: 72rem)');
  let lastFocus = null;
  details.addEventListener('focusin', (event) => {
    lastFocus = event.target;
  });
  details.addEventListener('focusout', (event) => {
    if (event.relatedTarget instanceof Node && !details.contains(event.relatedTarget))
      lastFocus = null;
  });
  document.addEventListener(
    'pointerdown',
    (event) => {
      if (event.target instanceof Node && !details.contains(event.target)) lastFocus = null;
    },
    true,
  );
  const resize = () => {
    const currentFocus = details.contains(document.activeElement)
      ? document.activeElement
      : lastFocus;
    const hadFocus = currentFocus !== null;
    const summaryHadFocus = currentFocus === summary;
    details.open = desktop.matches;
    let focusTarget = null;
    if (!desktop.matches && hadFocus) focusTarget = summary;
    if (desktop.matches && summaryHadFocus) focusTarget = links[0];
    const expectedDesktop = desktop.matches;
    if (focusTarget)
      requestAnimationFrame(() => {
        if (desktop.matches === expectedDesktop) focusTarget.focus();
      });
  };
  let desktopState = desktop.matches;
  const updateBreakpoint = () => {
    if (desktop.matches === desktopState) return;
    desktopState = desktop.matches;
    resize();
  };
  resize();
  desktop.addEventListener('change', updateBreakpoint);
  new ResizeObserver(updateBreakpoint).observe(document.documentElement);
  nav.addEventListener('click', (event) => {
    if (!(event.target instanceof HTMLAnchorElement)) return;
    if (!desktop.matches) details.open = false;
    const target = document.getElementById(event.target.dataset.tocTarget);
    if (target) {
      target.tabIndex = -1;
      target.focus({ preventScroll: true });
    }
  });
  document.addEventListener('click', (event) => {
    if (!desktop.matches && event.target instanceof Node && !nav.contains(event.target)) {
      const hadFocus = details.contains(document.activeElement);
      details.open = false;
      if (hadFocus) summary.focus();
    }
  });
  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape' && !desktop.matches && details.open) {
      const hadFocus = details.contains(document.activeElement);
      details.open = false;
      if (hadFocus) summary.focus();
    }
  });
  let positions = [];
  let active = -1;
  let scheduled = false;
  const update = () => {
    scheduled = false;
    const threshold =
      scrollY + parseFloat(getComputedStyle(document.documentElement).scrollPaddingTop) + 1;
    let low = 0,
      high = positions.length;
    while (low < high) {
      const middle = (low + high) >>> 1;
      if (positions[middle] <= threshold) low = middle + 1;
      else high = middle;
    }
    let index = low;
    if (scrollY > 0 && innerHeight + scrollY >= document.documentElement.scrollHeight - 2)
      index = links.length - 1;
    if (index === active) return;
    if (active >= 0) links[active].removeAttribute('aria-current');
    active = index;
    links[index].setAttribute('aria-current', 'true');
    currentLabel.textContent = entries[index];
  };
  const measure = () => {
    positions = headings.map((heading) => heading.getBoundingClientRect().top + scrollY);
    update();
  };
  addEventListener(
    'scroll',
    () => {
      if (!scheduled) {
        scheduled = true;
        requestAnimationFrame(update);
      }
    },
    { passive: true },
  );
  addEventListener('resize', measure);
  new ResizeObserver(measure).observe(article);
  document.fonts.ready.then(measure);
  addEventListener('load', () => {
    if (location.hash) {
      let id;
      try {
        id = decodeURIComponent(location.hash.slice(1));
      } catch {
        return;
      }
      const target = document.getElementById(id);
      if (target && article.contains(target)) target.scrollIntoView();
    }
    measure();
  });
  measure();
})();
