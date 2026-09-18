export const viewports = [
  { name: 'phone', width: 390, height: 844 },
  { name: 'intermediate', width: 1024, height: 768 },
  { name: 'desktop', width: 1440, height: 900 },
  { name: 'nav-before', width: 799, height: 768 },
  { name: 'nav-after', width: 800, height: 768 },
  { name: 'toc-before', width: 1151, height: 900 },
  { name: 'toc-after', width: 1152, height: 900 },
];

export const modes = ['light', 'dark'];

export const installations = [
  { name: 'root', base: '' },
  { name: 'prefix', base: '/docs' },
];

export const pages = [
  { name: 'overview', path: '/' },
  { name: 'long-guide', path: '/guides/long-guide/' },
  { name: 'code', path: '/reference/code-examples/' },
];

export const urlFor = (installation, page) =>
  `${installation.base}${page.path}`.replace(/\/{2,}/g, '/');
