import { test } from 'node:test';
import assert from 'node:assert/strict';
import { readFile } from 'node:fs/promises';
import { execFileSync } from 'node:child_process';
const source = await readFile(new URL('./filters.js', import.meta.url), 'utf8');
const { mount } = await import('data:text/javascript;base64,' + Buffer.from(source).toString('base64'));

test('auto filters debounce, preserve canonical navigation, discard stale responses and clean up', async () => {
  const form = new EventTarget();
  form.action = 'https://example.test/plugins/test/pages/current';
  form.values = [['assessment', ''], ['assessment', 'updates_found'], ['updateType', 'minor'], ['cursor', 'old'], ['revision', '7']];
  globalThis.FormData = class { constructor(form) { return form.values; } };
  globalThis.location = new URL(form.action);
  const status = { textContent: '' };
  const coverage = { href: 'https://example.test/plugins/test/pages/coverage?cursor=old' };
  const results = { setAttribute() {}, removeAttribute() {} };
  const root = { querySelector: (selector) => selector === '[data-upgrade-filters]' ? form : selector === '[data-filter-status]' ? status : selector === 'header a' ? coverage : results, closest: () => null };
  const emit = (type, ctx) => {
    const event = new Event(type, { cancelable: true });
    event.detail = { ctx };
    form.dispatchEvent(event);
    return event;
  };
  let applied = 0;
  form.addEventListener('filters:apply', () => applied++);
  const stop = mount(root);
  emit('combobox:change');
  emit('combobox:change');
  await new Promise(resolve => setTimeout(resolve, 400));
  assert.equal(applied, 1);
  const ctx = { request: { body: form.values }, response: { status: 200 } };
  emit('htmx:config:request', ctx);
  assert.equal(ctx.push, '/plugins/test/pages/current?assessment=&assessment=updates_found&updateType=minor');
  emit('htmx:before:request', ctx);
  assert.equal(status.textContent, 'Updating results…');
  assert.equal(emit('htmx:before:swap', ctx).defaultPrevented, false);
  emit('htmx:after:swap', ctx);
  assert.equal(new URL(coverage.href).search, '?assessment=&assessment=updates_found&updateType=minor');
  emit('htmx:finally:request', ctx);
  assert.equal(status.textContent, '');
  form.values = [['assessment', '']];
  assert.equal(emit('htmx:before:swap', ctx).defaultPrevented, true);
  ctx.request.body = form.values;
  ctx.response.status = 503;
  emit('htmx:config:request', ctx);
  emit('htmx:finally:request', ctx);
  assert.match(status.textContent, /Previous results are shown/);
  emit('combobox:change');
  emit('compositionstart');
  await new Promise(resolve => setTimeout(resolve, 400));
  assert.equal(applied, 1);
  emit('compositionend');
  stop();
  await new Promise(resolve => setTimeout(resolve, 400));
  emit('combobox:change');
  assert.equal(applied, 1);
});

test('A/B/A overlap rejects stale responses before pinned HTMX history processing', async (t) => {
  const form = new EventTarget();
  form.action = 'https://example.test/plugins/test/pages/current';
  const originalFormData = globalThis.FormData;
  const originalLocation = globalThis.location;
  globalThis.FormData = class { constructor(form) { return form.values; } };
  globalThis.location = new URL(form.action);
  t.after(() => { globalThis.FormData = originalFormData; globalThis.location = originalLocation; });
  const status = { textContent: '' };
  const attributes = new Map();
  const results = { setAttribute: (name, value) => attributes.set(name, value), removeAttribute: (name) => attributes.delete(name) };
  const coverage = { href: 'https://example.test/plugins/test/pages/coverage?search=previous' };
  let contentURL = '/plugins/test/page-content/current?search=previous';
  const slot = { getAttribute: () => contentURL, setAttribute: (_, value) => { contentURL = value; } };
  const root = { querySelector: (selector) => selector === '[data-upgrade-filters]' ? form : selector === '[data-filter-status]' ? status : selector === 'header a' ? coverage : results, closest: () => slot };
  const emit = (type, ctx) => {
    const event = new Event(type, { cancelable: true });
    event.detail = { ctx };
    form.dispatchEvent(event);
    return event;
  };
  const stop = mount(root);
  t.after(stop);
  const start = (search) => {
    form.values = [['assessment', 'updates_found'], ['search', search]];
    const ctx = { request: { body: form.values } };
    emit('htmx:config:request', ctx);
    assert.equal(emit('htmx:before:request', ctx).defaultPrevented, false, 'latest request must start');
    return ctx;
  };
  const dependency = JSON.parse(execFileSync('go', ['mod', 'download', '-json', 'github.com/araihu/goshtoso'], { cwd: new URL('../', import.meta.url), encoding: 'utf8' }));
  assert.equal(dependency.Version, 'v0.4.4', 'requalify lifecycle order when updating Goshtoso');
  const runtime = await readFile(`${dependency.Dir}/assets/js/runtime/htmx.org/4.0.0/htmx.min.js`, 'utf8');
  const responseStart = runtime.indexOf('if(e.text=await r.text(),!this.#L(t,"htmx:after:request"');
  const responseEnd = runtime.indexOf('}catch(r){', responseStart);
  const swapStart = runtime.indexOf('async swap(e){');
  const historyStart = runtime.indexOf('this.#_e(e)', swapStart);
  const preSwapEvent = runtime.indexOf('"htmx:before:swap"', swapStart);
  assert.ok(responseStart > 0 && responseEnd > responseStart);
  assert.ok(historyStart > swapStart && preSwapEvent > historyStart, 'HTMX pushes history before before:swap');
  // Execute the pinned response coordinator; redirect/status rules are inert for fixture 200 responses.
  const responseCode = runtime.slice(responseStart, responseEnd)
    .replaceAll('this.#L(', 'dispatch(')
    .replaceAll('this.#de(e)', 'false')
    .replaceAll('this.#fe(e)', 'undefined')
    .replaceAll('this.swap(e)', 'swap(e)');
  assert.ok(!responseCode.includes('this.#'), 'requalify changed HTMX response helpers');
  const receive = new (Object.getPrototypeOf(async () => {}).constructor)('e', 't', 'r', 'dispatch', 'swap', responseCode);
  const history = [];
  let swaps = 0;
  const complete = async (ctx) => {
    ctx.status = 'issuing';
    ctx.response = { status: 200 };
    ctx.hx = {};
    try {
      await receive(ctx, form, { text: async () => '<section>results</section>' },
        (_, type, detail) => !emit(type, detail.ctx).defaultPrevented,
        async (response) => {
          // Match the pinned swap ordering above, before the cancellable before:swap event.
          if (response.push) history.push(['pushState', response.push]);
          else if (response.replace) history.push(['replaceState', response.replace]);
          if (emit('htmx:before:swap', response).defaultPrevented) return;
          swaps++;
          emit('htmx:after:swap', response);
        });
    } finally { emit('htmx:finally:request', ctx); }
  };
  const a1 = start('A');
  const b = start('B');
  b.push = false;
  b.replace = '/old-B';
  const a2 = start('A');
  for (const old of [a1, b]) {
    await complete(old);
    assert.deepEqual(history, [], 'stale response must perform zero pushState/replaceState before swap');
    assert.equal(swaps, 0, 'stale response must not swap results');
    assert.equal(status.textContent, 'Updating results…');
    assert.equal(attributes.get('aria-busy'), 'true');
    assert.equal(coverage.href, 'https://example.test/plugins/test/pages/coverage?search=previous');
    assert.equal(contentURL, '/plugins/test/page-content/current?search=previous');
  }
  const previousCoverage = coverage.href;
  const previousContent = contentURL;
  for (const old of [a1, b]) {
    for (const response of [undefined, { status: 503 }, { status: 200 }]) {
      old.response = response;
      assert.equal(emit('htmx:before:swap', old).defaultPrevented, true, 'old context must not swap, even when its URL matches A2');
      emit('htmx:after:swap', old);
      emit('htmx:finally:request', old);
      assert.equal(status.textContent, 'Updating results…', 'old completion must not publish a failure or success');
      assert.equal(attributes.get('aria-busy'), 'true', 'latest A request remains pending');
      assert.equal(coverage.href, previousCoverage, 'old context must not update coverage navigation');
      assert.equal(contentURL, previousContent, 'old context must not change the host refresh URL');
      assert.equal(old.push, false, 'old context must not push browser history');
    }
    assert.equal(emit('htmx:before:request', old).defaultPrevented, true, 'late old start must be cancelled');
  }
  await complete(a2);
  assert.deepEqual(history, [['pushState', '/plugins/test/pages/current?assessment=updates_found&search=A']]);
  assert.equal(swaps, 1);
  assert.equal(new URL(coverage.href).searchParams.get('search'), 'A');
  assert.equal(new URL(contentURL, location.origin).searchParams.get('search'), 'A');
  assert.equal(attributes.has('aria-busy'), false);
  assert.equal(status.textContent, '');
  emit('htmx:finally:request', a1);
  assert.equal(status.textContent, '', 'old completion stays ignored after A2 succeeds');
  const pending = start('pending');
  form.values = [['assessment', 'updates_found'], ['search', 'unsent-edit']];
  const historyBefore = history.length;
  const swapsBefore = swaps;
  const coverageBefore = coverage.href;
  const contentBefore = contentURL;
  await complete(pending);
  assert.equal(history.length, historyBefore, 'unsent edits must reject history before before:swap');
  assert.equal(swaps, swapsBefore);
  assert.equal(coverage.href, coverageBefore);
  assert.equal(contentURL, contentBefore);
  assert.equal(status.textContent, 'Updating results…');
  assert.equal(attributes.get('aria-busy'), 'true');
  const failed = start('C');
  failed.response = { status: 503 };
  emit('htmx:finally:request', failed);
  assert.match(status.textContent, /Previous results are shown/, 'latest request errors must remain visible');
  assert.equal(attributes.has('aria-busy'), false);
});
