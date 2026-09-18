(() => {
  const root = document.getElementById('site-search');
  const inlineCorpus = document.getElementById('search-corpus');
  const toggle = document.getElementById('search-toggle');
  const dialog = document.getElementById('search-dialog');
  const close = document.getElementById('search-close');
  const input = document.getElementById('search-input');
  const status = document.getElementById('search-status');
  const results = document.getElementById('search-results');
  if (
    !root ||
    !(toggle instanceof HTMLButtonElement) ||
    !(dialog instanceof HTMLDialogElement) ||
    !(close instanceof HTMLButtonElement) ||
    !(input instanceof HTMLInputElement) ||
    !status ||
    !results ||
    !window.FlexSearch
  )
    return;

  const messages = {
    retry: root.dataset.searchRetry,
    empty: root.dataset.searchEmpty,
    loading: root.dataset.searchLoading,
    error: root.dataset.searchError,
    noResults: root.dataset.searchNoResults,
    oneResult: root.dataset.searchOneResult,
    results: root.dataset.searchResults,
  };
  if (Object.values(messages).some((message) => !message)) return;
  if (!inlineCorpus && !root.dataset.corpus) return;
  const resultCount = (count) =>
    (count === 1 ? messages.oneResult : messages.results).replace('{count}', String(count));

  const wordPattern = /[\p{L}\p{N}\p{M}]+/gu;
  const normalize = (value) => value.normalize('NFC').toLocaleLowerCase();
  const tokens = (value) => normalize(value).match(wordPattern) || [];
  const state = { phase: 'idle', records: [], index: null, opener: null };

  const setStatus = (message, retry = false) => {
    status.replaceChildren();
    const text = document.createElement('p');
    text.textContent = message;
    status.append(text);
    if (!retry) return;
    const button = document.createElement('button');
    button.type = 'button';
    button.textContent = messages.retry;
    button.addEventListener('click', load);
    status.append(button);
  };

  const safeURL = (value) => {
    try {
      const url = new URL(value, location.origin);
      return url.origin === location.origin ? url.href : '#';
    } catch {
      return '#';
    }
  };

  const matches = (terms, value) => {
    const source = tokens(value);
    return terms.every((term, index) =>
      source.some((word) => (index === terms.length - 1 ? word.startsWith(term) : word === term)),
    );
  };

  const exactFinal = (term, value) => tokens(value).includes(term);

  const excerpt = (body, terms) => {
    const final = terms.at(-1);
    for (const match of body.matchAll(wordPattern)) {
      const word = normalize(match[0]);
      if (!word.startsWith(final)) continue;
      const before = Array.from(body.slice(0, match.index)).length;
      const characters = Array.from(body);
      const start = Math.max(0, before - 72);
      const end = Math.min(characters.length, before + Array.from(match[0]).length + 96);
      return (start ? '…' : '') + characters.slice(start, end).join('') + (end < characters.length ? '…' : '');
    }
    return Array.from(body).slice(0, 180).join('') + (Array.from(body).length > 180 ? '…' : '');
  };

  const renderResults = () => {
    results.replaceChildren();
    const terms = tokens(input.value);
    if (!terms.length) {
      setStatus(messages.empty);
      return;
    }
    if (state.phase === 'loading') {
      setStatus(messages.loading);
      return;
    }
    if (state.phase === 'failed') {
      setStatus(messages.error, true);
      return;
    }
    if (state.phase !== 'ready') return;

    const candidates = state.index.search({ query: terms.at(-1), limit: 0, resolve: false });
    const ids = Array.isArray(candidates) ? candidates : candidates.result || [];
    const ranked = ids
      .map((id) => state.records[id])
      .filter((record) => record && matches(terms, record.all))
      .map((record) => {
        const heading = record.headings.find(
          (item) => matches(terms, item.text) || tokens(item.text).some((word) => word.startsWith(terms.at(-1))),
        );
        const tier = matches(terms, record.title) ? 0 : heading ? 1 : 2;
        const tierText = tier === 0 ? record.title : tier === 1 ? heading.text : record.body;
        return { record, heading, tier, exact: exactFinal(terms.at(-1), tierText) ? 0 : 1 };
      })
      .sort((a, b) =>
        a.tier - b.tier ||
        a.exact - b.exact ||
        a.record.url.localeCompare(b.record.url) ||
        (a.heading?.order ?? Number.MAX_SAFE_INTEGER) - (b.heading?.order ?? Number.MAX_SAFE_INTEGER),
      );

    if (!ranked.length) {
      setStatus(messages.noResults);
      return;
    }
    setStatus(resultCount(ranked.length));
    const list = document.createElement('ol');
    for (const { record, heading } of ranked) {
      const item = document.createElement('li');
      const link = document.createElement('a');
      link.href = safeURL(record.url + (heading?.id ? '#' + encodeURIComponent(heading.id) : ''));
      link.textContent = record.title || record.url;
      item.append(link);
      if (heading?.id) {
        const section = document.createElement('span');
        section.className = 'search-section';
        section.textContent = ` — ${heading.text}`;
        item.append(section);
      }
      const context = document.createElement('p');
      context.textContent = excerpt(record.body, terms);
      item.append(context);
      list.append(item);
    }
    results.append(list);
  };

  const load = async () => {
    state.phase = 'loading';
    results.replaceChildren();
    setStatus(messages.loading);
    try {
      let payload;
      if (inlineCorpus) {
        await Promise.resolve();
        payload = JSON.parse(inlineCorpus.textContent);
      } else {
        const response = await fetch(root.dataset.corpus, { credentials: 'same-origin' });
        if (!response.ok) throw new Error(`Search corpus returned ${response.status}`);
        payload = await response.json();
      }
      if (!Array.isArray(payload?.documents)) throw new Error('Search corpus has an invalid format');
      const index = new window.FlexSearch.Index({ encode: false, tokenize: 'forward', cache: false });
      state.records = payload.documents.map((document, id) => {
        const headings = Array.isArray(document.headings)
          ? document.headings
              .filter((heading) => heading && typeof heading.text === 'string')
              .map((heading, order) => ({ id: String(heading.id || ''), text: heading.text, order }))
          : [];
        const record = {
          title: typeof document.title === 'string' ? document.title : '',
          url: typeof document.url === 'string' ? document.url : '',
          body: typeof document.body === 'string' ? document.body : '',
          headings,
        };
        record.all = [record.title, record.body, ...headings.map((heading) => heading.text)].join(' ');
        index.add(id, normalize(record.all));
        return record;
      });
      state.index = index;
      state.phase = 'ready';
      renderResults();
    } catch {
      state.phase = 'failed';
      setStatus(messages.error, true);
    }
  };

  const open = (opener = toggle) => {
    state.opener = opener;
    if (!dialog.open) dialog.showModal();
    document.body.dataset.searchModalOpen = '';
    input.focus();
    if (state.phase === 'idle') load();
  };

  const restoreFocus = () => {
    delete document.body.dataset.searchModalOpen;
    if (state.opener?.isConnected && !state.opener.disabled && state.opener.getClientRects().length)
      state.opener.focus();
  };

  toggle.addEventListener('click', () => open());
  close.addEventListener('click', () => dialog.close());
  dialog.addEventListener('close', restoreFocus);
  dialog.addEventListener('click', (event) => {
    if (event.target === dialog) dialog.close();
  });
  input.addEventListener('input', renderResults);
  document.addEventListener('keydown', (event) => {
    if (event.key === 'Escape' && dialog.open) {
      event.preventDefault();
      dialog.close();
      return;
    }
    if ((event.ctrlKey || event.metaKey) && event.key.toLowerCase() === 'k') {
      event.preventDefault();
      dialog.open ? dialog.close() : open();
    }
  });
  toggle.disabled = false;
})();
