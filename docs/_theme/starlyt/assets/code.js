(() => {
  const messages = document.documentElement.dataset;
  const canCopy =
    navigator.clipboard?.writeText
    && messages.copyLabel
    && messages.copySuccess
    && messages.copyFailure;
  document.querySelectorAll('#article pre').forEach((pre) => {
    const code = pre.querySelector('code');
    if (!code) return;
    if (pre instanceof HTMLElement) pre.dir = 'ltr';
    if (!canCopy || pre.closest('.code-frame')) return;
    const semanticFrame = pre.closest('figure.highlight[data-code-frame]');
    const frame = semanticFrame
      || (pre.parentElement.classList.contains('highlight') ? pre.parentElement : document.createElement('div'));
    frame.classList.add('code-frame');
    if (!frame.contains(pre)) {
      pre.before(frame);
      frame.append(pre);
    }
    if (pre instanceof HTMLElement) pre.tabIndex = 0;
    const button = document.createElement('button');
    button.type = 'button';
    button.className = 'copy-button';
    button.title = messages.copyLabel;
    button.setAttribute('aria-label', messages.copyLabel);
    const icon = document.createElementNS('http://www.w3.org/2000/svg', 'svg');
    icon.setAttribute('viewBox', '0 0 24 24');
    icon.setAttribute('aria-hidden', 'true');
    icon.setAttribute('fill', 'none');
    icon.setAttribute('stroke', 'currentColor');
    icon.setAttribute('stroke-width', '1.75');
    const shape = document.createElementNS('http://www.w3.org/2000/svg', 'path');
    shape.setAttribute('d', 'M3 19a2 2 0 0 1-1-2V2a2 2 0 0 1 1-1h13a2 2 0 0 1 2 1');
    const rectangle = document.createElementNS('http://www.w3.org/2000/svg', 'rect');
    rectangle.setAttribute('x', '6');
    rectangle.setAttribute('y', '5');
    rectangle.setAttribute('width', '16');
    rectangle.setAttribute('height', '18');
    rectangle.setAttribute('rx', '1.5');
    rectangle.setAttribute('ry', '1.5');
    icon.append(shape, rectangle);
    button.append(icon);
    const feedback = document.createElement('span');
    feedback.className = 'copy-feedback';
    feedback.setAttribute('role', 'status');
    let timer;
    let pending = false;
    button.addEventListener('click', async () => {
      if (pending) return;
      pending = true;
      clearTimeout(timer);
      feedback.textContent = '';
      delete feedback.dataset.error;
      button.setAttribute('aria-busy', 'true');
      try {
        await navigator.clipboard.writeText(code.textContent);
        feedback.textContent = messages.copySuccess;
      } catch {
        feedback.textContent = messages.copyFailure;
        feedback.dataset.error = '';
      } finally {
        pending = false;
        button.removeAttribute('aria-busy');
        timer = setTimeout(() => {
          feedback.textContent = '';
        }, 2500);
      }
    });
    frame.append(button, feedback);
  });
})();
