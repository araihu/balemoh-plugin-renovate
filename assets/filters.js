// Compose Goshtoso's selection event with the host's results fragment route.
export function mount(root) {
  const form = root.querySelector('[data-upgrade-filters]');
  if (!form) return;
  const status = root.querySelector('[data-filter-status]');
  const coverage = root.querySelector('header a');
  const listeners = new AbortController();
  const requests = new WeakMap();
  let timer;
  let composing = false;
  let latestRequest;
  const canonical = (values) => {
    const url = new URL(form.action);
    url.search = new URLSearchParams(values).toString();
    url.searchParams.delete('cursor');
    url.searchParams.delete('revision');
    return url.pathname + url.search;
  };
  const current = () => canonical(new FormData(form));
  const isCurrentRequest = (ctx) => ctx === latestRequest && requests.get(ctx) === current();
  const listen = (type, handler) => form.addEventListener(type, handler, { signal: listeners.signal });
  const schedule = () => {
    clearTimeout(timer);
    timer = setTimeout(() => form.dispatchEvent(new Event('filters:apply', { bubbles: true })), 350);
  };
  listen('combobox:change', schedule);
  listen('input', (event) => {
    if (event.target.name === 'search' && !composing && !event.isComposing) schedule();
  });
  listen('compositionstart', () => { composing = true; clearTimeout(timer); });
  listen('compositionend', () => { composing = false; schedule(); });
  listen('submit', () => clearTimeout(timer));
  listen('htmx:config:request', (event) => {
    const ctx = event.detail.ctx;
    const url = canonical(ctx.request.body);
    requests.set(ctx, url);
    latestRequest = ctx;
    ctx.push = url === location.pathname + location.search ? false : url;
  });
  listen('htmx:before:request', (event) => {
    if (!isCurrentRequest(event.detail.ctx)) { event.preventDefault(); return; }
    status.textContent = 'Updating results…';
    root.querySelector('.image-upgrades-results')?.setAttribute('aria-busy', 'true');
  });
  const rejectStaleResponse = (event) => {
    if (!isCurrentRequest(event.detail.ctx)) {
      event.detail.ctx.push = false;
      event.preventDefault();
    }
  };
  // HTMX 4 pushes history before before:swap; cancel stale responses earlier.
  listen('htmx:after:request', rejectStaleResponse);
  listen('htmx:before:swap', rejectStaleResponse);
  listen('htmx:after:swap', (event) => {
    if (!isCurrentRequest(event.detail.ctx)) { event.detail.ctx.push = false; return; }
    const url = new URL(coverage.href);
    url.search = new URL(requests.get(event.detail.ctx), location.origin).search;
    coverage.href = url.href;
    // PluginSlot reloads its content on host refresh; retain the applied query there too.
    const slot = root.closest('[data-plugin-slot]');
    if (slot) {
      const content = new URL(slot.getAttribute('hx-get'), location.origin);
      content.search = url.search;
      slot.setAttribute('hx-get', content.pathname + content.search);
    }
  });
  listen('htmx:finally:request', (event) => {
    if (!isCurrentRequest(event.detail.ctx)) return;
    root.querySelector('.image-upgrades-results')?.removeAttribute('aria-busy');
    status.textContent = event.detail.ctx.response?.status === 200
      ? ''
      : 'Could not update results. Previous results are shown. Change a filter or press Enter to retry.';
  });
  return () => { clearTimeout(timer); listeners.abort(); };
}
