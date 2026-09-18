(() => {
  document.addEventListener('DOMContentLoaded', () => {
    const controls = document.getElementById('global-controls');
    const header = document.querySelector('header');
    const nav = document.getElementById('site-nav');
    const menuToggle = document.getElementById('menu-toggle');
    if (!controls || !header || !nav) return;

    const desktop = matchMedia('(min-width: 50rem)');
    const placeControls = () => {
      const focused = controls.contains(document.activeElement) ? document.activeElement : null;
      (desktop.matches ? header : nav).append(controls);
      if (focused instanceof HTMLElement) {
        if (desktop.matches) focused.focus();
        else menuToggle?.focus();
      }
    };

    placeControls();
    desktop.addEventListener('change', placeControls);
  });
})();
