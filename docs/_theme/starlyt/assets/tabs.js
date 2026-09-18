(() => {
  const groupElements = Array.from(document.querySelectorAll('[data-tabs]'));
  if (!groupElements.length) return;
  const rtl = document.documentElement.dir === 'rtl';

  const focusableSelector = [
    'a[href]',
    'button:not([disabled])',
    'input:not([disabled]):not([type="hidden"])',
    'select:not([disabled])',
    'textarea:not([disabled])',
    'summary',
    '[contenteditable]',
    '[tabindex]:not([tabindex="-1"])',
  ].join(',');
  const storagePrefix = 'starlyt-tabs:v1:';
  const activeBySync = new Map();
  const groupsBySync = new Map();

  const groups = groupElements.map((element, groupIndex) => {
    const items = Array.from(element.children).filter((child) => child.matches('.tab-item'));
    const sync = element.querySelector(':scope > .tabs-sync-key')?.textContent?.trim() || '';
    const group = {
      element,
      groupIndex,
      items,
      labels: items.map(
        (item) => item.querySelector(':scope > .tab-item-label > .tab-label-text')?.textContent || '',
      ),
      sync,
      persist: element.hasAttribute('data-tabs-persist') && Boolean(sync),
      tabs: [],
      panels: [],
    };
    if (sync) {
      const synced = groupsBySync.get(sync) || [];
      synced.push(group);
      groupsBySync.set(sync, synced);
    }
    return group;
  });

  for (const [sync, synced] of groupsBySync) {
    if (!synced.some((group) => group.persist)) continue;
    try {
      const stored = localStorage.getItem(storagePrefix + sync);
      if (stored) activeBySync.set(sync, stored);
    } catch {
      /* Persistence is optional; all panels remain available. */
    }
  }

  const persist = (sync, label) => {
    if (!groupsBySync.get(sync)?.some((group) => group.persist)) return;
    try {
      localStorage.setItem(storagePrefix + sync, label);
    } catch {
      /* Keep synchronized in-memory behavior when storage is unavailable. */
    }
  };

  const applySelection = (group, index, { focus = false, synchronize = false } = {}) => {
    const tab = group.tabs[index];
    const panel = group.panels[index];
    if (!tab || !panel) return;
    const previousTop = group.element.getBoundingClientRect().top;
    group.tabs.forEach((candidate, candidateIndex) => {
      const selected = candidateIndex === index;
      candidate.setAttribute('aria-selected', String(selected));
      candidate.tabIndex = selected ? 0 : -1;
      group.panels[candidateIndex].hidden = !selected;
    });
    if (focus) tab.focus();

    if (synchronize && group.sync) {
      const label = group.labels[index];
      activeBySync.set(group.sync, label);
      for (const receiver of groupsBySync.get(group.sync) || []) {
        if (receiver === group) continue;
        const receiverIndex = receiver.labels.indexOf(label);
        if (receiverIndex !== -1) applySelection(receiver, receiverIndex);
      }
      persist(group.sync, label);
      const offset = group.element.getBoundingClientRect().top - previousTop;
      if (offset) window.scrollTo({ top: window.scrollY + offset, behavior: 'instant' });
    }
  };

  groups.forEach((group) => {
    if (!group.items.length) return;
    if (group.sync && !activeBySync.has(group.sync)) activeBySync.set(group.sync, group.labels[0]);

    const wrapper = document.createElement('div');
    wrapper.className = 'tablist-wrapper';
    const tablist = document.createElement('div');
    tablist.className = 'tablist';
    tablist.setAttribute('role', 'tablist');
    tablist.setAttribute('aria-orientation', 'horizontal');
    wrapper.append(tablist);

    group.items.forEach((item, itemIndex) => {
      const heading = item.querySelector(':scope > .tab-item-label');
      const panel = item.querySelector(':scope > .tab-panel');
      if (!(heading instanceof HTMLElement) || !(panel instanceof HTMLElement)) return;
      const tabId = `starlyt-tab-${group.groupIndex}-${itemIndex}`;
      const panelId = `starlyt-tab-panel-${group.groupIndex}-${itemIndex}`;
      const tab = document.createElement('button');
      tab.type = 'button';
      tab.className = 'tab';
      tab.id = tabId;
      tab.append(...Array.from(heading.childNodes, (node) => node.cloneNode(true)));
      tab.setAttribute('role', 'tab');
      tab.setAttribute('aria-controls', panelId);
      panel.id = panelId;
      panel.setAttribute('role', 'tabpanel');
      panel.setAttribute('aria-labelledby', tabId);
      if (!panel.querySelector(focusableSelector)) panel.tabIndex = 0;
      heading.hidden = true;
      item.setAttribute('role', 'presentation');
      tablist.append(tab);
      group.tabs.push(tab);
      group.panels.push(panel);

      tab.addEventListener('click', () => {
        const index = group.tabs.indexOf(tab);
        if (index !== -1) applySelection(group, index, { focus: true, synchronize: true });
      });
      tab.addEventListener('keydown', (event) => {
        const index = group.tabs.indexOf(tab);
        let nextIndex;
        if (event.key === 'ArrowLeft')
          nextIndex = (index + (rtl ? 1 : -1) + group.tabs.length) % group.tabs.length;
        else if (event.key === 'ArrowRight')
          nextIndex = (index + (rtl ? -1 : 1) + group.tabs.length) % group.tabs.length;
        else if (event.key === 'Home') nextIndex = 0;
        else if (event.key === 'End') nextIndex = group.tabs.length - 1;
        else return;
        event.preventDefault();
        applySelection(group, nextIndex, { focus: true, synchronize: true });
      });
    });

    if (!group.tabs.length) return;
    group.element.prepend(wrapper);
    group.element.classList.add('is-enhanced');
    const synchronizedLabel = group.sync ? activeBySync.get(group.sync) : undefined;
    const initialIndex = synchronizedLabel ? group.labels.indexOf(synchronizedLabel) : 0;
    applySelection(group, initialIndex === -1 ? 0 : initialIndex);
  });
})();
