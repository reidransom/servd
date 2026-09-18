(() => {
  const anchorLabel = document.documentElement.dataset.anchorLabel;
  if (!anchorLabel) return;
  document.querySelectorAll('#article :is(h1, h2, h3, h4, h5, h6)[id]').forEach((heading) => {
    if (
      heading.parentElement.classList.contains('sl-heading-wrapper') ||
      !heading.textContent.trim()
    )
      return;
    const badgeContainer = heading.nextElementSibling;
    const headingBadge =
      badgeContainer instanceof HTMLParagraphElement &&
      badgeContainer.childElementCount === 1 &&
      badgeContainer.firstElementChild?.matches('.sl-badge') &&
      badgeContainer.textContent.trim() === badgeContainer.firstElementChild.textContent.trim()
        ? badgeContainer.firstElementChild
        : null;
    const wrapper = document.createElement('div');
    wrapper.className = 'sl-heading-wrapper level-' + heading.tagName.toLowerCase();
    const link = document.createElement('a');
    link.className = 'sl-anchor-link';
    link.href = '#' + encodeURIComponent(heading.id);
    link.setAttribute('aria-label', anchorLabel.replace('{title}', () => heading.textContent.trim()));
    const icon = document.createElement('span');
    icon.className = 'sl-anchor-icon';
    const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    svg.setAttribute('viewBox', '0 0 24 24');
    svg.setAttribute('aria-hidden', 'true');
    const path = document.createElementNS('http://www.w3.org/2000/svg', 'path');
    path.setAttribute('fill', 'none');
    path.setAttribute('stroke', 'currentColor');
    path.setAttribute('stroke-width', '2');
    path.setAttribute(
      'd',
      'm10 13 4-4 M8 16l-1 1a4 4 0 0 1-6-6l5-5a4 4 0 0 1 6 0 M16 8l1-1a4 4 0 0 1 6 6l-5 5a4 4 0 0 1-6 0',
    );
    svg.append(path);
    icon.append(svg);
    link.append(icon);
    heading.before(wrapper);
    wrapper.append(heading);
    if (headingBadge) {
      wrapper.append(headingBadge);
      badgeContainer.remove();
    }
    wrapper.append(link);
  });
})();
