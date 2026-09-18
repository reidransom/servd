(() => {
  const root = document.documentElement;
  const system = matchMedia('(prefers-color-scheme: dark)');
  const parse = (value) => (value === 'light' || value === 'dark' ? value : 'auto');
  let preference = 'auto';
  try {
    preference = parse(localStorage.getItem('starlight-theme'));
  } catch {
    /* Keep an in-memory preference if storage is unavailable. */
  }
  const apply = () => {
    root.dataset.theme = preference === 'auto' ? (system.matches ? 'dark' : 'light') : preference;
  };
  apply();
  system.addEventListener('change', apply);
  document.addEventListener('DOMContentLoaded', () => {
    const picker = document.getElementById('mode-picker');
    const select = document.getElementById('color-mode');
    if (!picker || !(select instanceof HTMLSelectElement)) return;
    const icon = picker.querySelector('svg path');
    const icons = {
      auto: 'M4 4h16v12H4z M1 20h22 M9 16v4m6-4v4',
      light:
        'M12 3V1m0 22v-2M3 12H1m22 0h-2M5.6 5.6 4.2 4.2m15.6 15.6-1.4-1.4m-12.8 0-1.4 1.4M19.8 4.2l-1.4 1.4M16 12a4 4 0 1 1-8 0 4 4 0 0 1 8 0',
      dark: 'M20.8 14.2A9 9 0 0 1 9.8 3.2 9 9 0 1 0 20.8 14.2Z',
    };
    const updatePicker = () => {
      select.value = preference;
      icon?.setAttribute('d', icons[preference]);
    };
    updatePicker();
    select.addEventListener('change', () => {
      preference = parse(select.value);
      apply();
      updatePicker();
      try {
        if (preference === 'auto') localStorage.removeItem('starlight-theme');
        else localStorage.setItem('starlight-theme', preference);
      } catch {
        /* This tab still honors changes when persistence is denied. */
      }
    });
    picker.hidden = false;
  });
})();
