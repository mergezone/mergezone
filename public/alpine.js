let diffsLib = null;
let diffsLibPromise = null;

async function ensureDiffsLib() {
  if (diffsLib) return diffsLib;
  if (diffsLibPromise) return diffsLibPromise;
  diffsLibPromise = import('https://esm.sh/@pierre/diffs@1.2.12').then(lib => {
    diffsLib = lib;
    return lib;
  });
  return diffsLibPromise;
}

let zoomInstance = null;
let zoomLibPromise = null;

async function ensureZoom() {
  if (zoomLibPromise) return zoomLibPromise;
  zoomLibPromise = import('https://esm.sh/medium-zoom@1.1.0').then(lib => {
    return lib.default;
  });
  return zoomLibPromise;
}

async function attachZoom(selector) {
  // Detach any existing instance before resolving the factory, otherwise
  // ensureZoom() returns the stale instance instead of the factory function.
  if (zoomInstance) {
    zoomInstance.detach();
    zoomInstance = null;
  }
  const mediumZoom = await ensureZoom();
  zoomInstance = mediumZoom(selector, {
    background: 'rgba(0,0,0,0.8)',
    margin: 48,
    zIndex: 99999,
  });
  return zoomInstance;
}

function getDiffTheme() {
  const cls = document.documentElement.classList;
  if (cls.contains('midnight')) return 'pierre-dark';
  if (cls.contains('purple')) return 'pierre-dark';
  if (cls.contains('parchment')) return 'pierre-light';
  return 'pierre-light';
}

const DIFF_WRAPPER_ID = 'context-pane-diffs';

async function loadPRDiffs(body, owner, repo, prNumber, signal) {
  const lib = await ensureDiffsLib();

  const resp = await fetch(`/${owner}/${repo}/pull/${prNumber}/diff`, { signal });
  if (!resp.ok) throw new Error('Failed to fetch diff: ' + resp.status);

  const diffText = await resp.text();
  const patches = lib.parsePatchFiles(diffText);

  // The virtualizer renders against a scroll root (the context pane body) and
  // measures a dedicated content wrapper that holds every rendered file diff.
  const wrapper = document.createElement('div');
  wrapper.id = DIFF_WRAPPER_ID;
  body.appendChild(wrapper);

  const virtualizer = new lib.Virtualizer({
    // Extra viewport size in pixels rendered above and below the viewport.
    overscrollSize: 1000,
    // IntersectionObserver root margin in pixels for visibility tracking.
    intersectionObserverMargin: 4000,
    // Keep disabled in production; it logs size changes for metric tuning.
    resizeDebugging: false,
  });
  virtualizer.setup(body, wrapper);

  const instances = [];

  try {
    for (const patch of patches) {
      for (const fileDiffMeta of patch.files) {
        const fileDiff = new lib.VirtualizedFileDiff({
          theme: getDiffTheme(),
          diffStyle: 'unified',
          overflow: 'scroll',
          disableLineNumbers: false,
          diffIndicators: 'bars',
        }, virtualizer);

        fileDiff.render({ fileDiff: fileDiffMeta, containerWrapper: wrapper });
        instances.push(fileDiff);
      }
    }
  } catch (err) {
    cleanupDiffs(instances);
    try { virtualizer.cleanUp(); } catch (e) {}
    wrapper.remove();
    throw err;
  }

  return { instances, virtualizer, wrapper };
}

function cleanupDiffs(instances) {
  for (const inst of instances) {
    try { inst.cleanUp(); } catch (e) {}
  }
}

document.addEventListener('alpine:init', () => {
  Alpine.store('app', {
    themes: ['white', 'parchment', 'purple', 'midnight'],
    theme: 'midnight',

    init() {
      const saved = localStorage.getItem('theme');
      if (saved && this.themes.includes(saved)) {
        this.theme = saved;
      } else {
        this.theme = window.matchMedia('(prefers-color-scheme: dark)').matches
          ? 'midnight' : 'parchment';
      }
      document.documentElement.className = this.theme;
    },

    cycleTheme() {
      const wasDark = ['purple', 'midnight'].includes(this.theme);
      const idx = this.themes.indexOf(this.theme);
      this.theme = this.themes[(idx + 1) % this.themes.length];
      const isDark = ['purple', 'midnight'].includes(this.theme);
      document.documentElement.className = this.theme;
      localStorage.setItem('theme', this.theme);

      if (wasDark !== isDark) {
        Alpine.store('contextPane').rerenderDiffs();
      }
    },

    _loadAvatars() {
      const sidebar = document.querySelector('.aside-column');
      if (!sidebar) return;
      sidebar.querySelectorAll('img[data-src]').forEach(img => {
        const src = img.dataset.src;
        const preloadImg = new Image();
        preloadImg.onload = () => {
          img.src = src;
          img.removeAttribute('data-src');
          img.classList.add('loaded');
        };
        preloadImg.src = src;
      });
    },

    _loadContextPaneImages() {
      const container = document.getElementById('context-pane-content');
      if (!container) return;
      container.querySelectorAll('img[data-src]').forEach(img => {
        const src = img.dataset.src;
        const preloadImg = new Image();
        preloadImg.onload = () => {
          img.src = src;
          img.removeAttribute('data-src');
          img.classList.add('loaded');
        };
        preloadImg.src = src;
      });
    }
  });

  Alpine.store('contextPane', {
    open: false,
    prNumber: null,
    prTitle: '',
    prStatusClass: '',
    prUrl: '',
    prState: '',
    prAge: '',
    prAuthorName: '',
    prAuthorUrl: '',
    prScope: '',
    prScopeUrl: '',
    bodyHtml: '',
    loadingBody: false,
    diffInstances: [],
    diffVirtualizer: null,
    diffWrapper: null,
    loadingDiffs: false,
    _abortController: null,
    _owner: '',
    _repo: '',
    _stashedDiffs: null,
    _stashedDiffsTheme: null,

    async openPane(prNumber, dataset = {}) {
      if (prNumber === this.prNumber) {
        if (this.open) {
          this._stashDiffs();
          this.open = false;
        } else {
          this._openWithDeferredDiffs();
        }
        return;
      }

      this._cleanup();

      this.prNumber = prNumber;

      this.prTitle = dataset.prTitle || 'Pull Request';
      this.prStatusClass = dataset.prStatusClass || '';
      this.prUrl = dataset.prUrl || '#';
      this.prState = dataset.prState || '';
      this.prAge = dataset.prAge || '';
      this.prAuthorName = dataset.prAuthorName || '';
      this.prAuthorUrl = dataset.prAuthorUrl || '#';
      this.prScope = dataset.prScope || '';
      this.prScopeUrl = dataset.prScopeUrl || '#';

      const card = document.querySelector(`.pull-request[data-pr-number="${prNumber}"]`);
      const owner = card?.closest('[data-owner]')?.dataset.owner;
      const repo = card?.closest('[data-repo]')?.dataset.repo;

      if (!owner || !repo) {
        const pathParts = window.location.pathname.split('/').filter(Boolean);
        if (pathParts.length >= 2) {
          this._owner = pathParts[0];
          this._repo = pathParts[1];
        }
      } else {
        this._owner = owner;
        this._repo = repo;
      }

      this.bodyHtml = this._bodySkeletonHtml();
      this.loadingBody = true;
      this.loadingDiffs = true;

      this.open = true;

      const selected = document.querySelector('.pull-request.selected');
      if (selected) selected.classList.remove('selected');
      card?.classList.add('selected');

      this._fetchBodyAndDiffs(prNumber);
    },

    closePane() {
      // Keep the diff session (wrapper + virtualizer + instances) alive so
      // reopening the same pull request is instant. The session is fully torn
      // down in _cleanup() when a different pull request is opened.
      this._stashDiffs();
      if (this._abortController) {
        this._abortController.abort();
        this._abortController = null;
      }
      if (zoomInstance) {
        zoomInstance.detach();
        zoomInstance = null;
      }

      const selected = document.querySelector('.pull-request.selected');
      if (selected) selected.classList.remove('selected');

      this.open = false;
    },

    openPrRef(event) {
      const link = event.target.closest('.pr-ref');
      if (!link) return;
      event.preventDefault();

      const prNumber = parseInt(link.dataset.prNumber, 10);
      if (isNaN(prNumber)) return;

      const card = document.querySelector(`.pull-request[data-pr-number="${prNumber}"]`);
      if (card) {
        this.openPane(prNumber, card.dataset);
        return;
      }

      const pathParts = window.location.pathname.split('/').filter(Boolean);
      const owner = pathParts[0] || this._owner;
      const repo = pathParts[1] || this._repo;
      if (!owner || !repo) return;

      link.classList.add('loading');

      this._fetchBodyRaw(prNumber).then(html => {
        link.classList.remove('loading');

        this._cleanup();
        this.prNumber = prNumber;
        this.prTitle = 'Pull Request #' + prNumber;
        this.prStatusClass = '';
        this.prUrl = `https://github.com/${owner}/${repo}/pull/${prNumber}`;
        this.prState = '';
        this.prAge = '';
        this.prAuthorName = '';
        this.prAuthorUrl = '#';
        this.prScope = '';
        this.prScopeUrl = '#';
        this._owner = owner;
        this._repo = repo;

        this.bodyHtml = html;
        this.open = true;
      }).catch(() => {
        link.classList.remove('loading');
      });
    },

    _stashDiffs() {
      const container = document.getElementById('context-pane-content');
      const body = container?.querySelector('.context-pane-body');
      if (!body) return;

      const wrapper = body.querySelector('#' + DIFF_WRAPPER_ID);
      if (!wrapper) return;

      wrapper.remove();
      this._stashedDiffs = wrapper;
      this._stashedDiffsTheme = getDiffTheme();
    },

    _unstashDiffs() {
      if (!this._stashedDiffs) return false;

      const container = document.getElementById('context-pane-content');
      const body = container?.querySelector('.context-pane-body');
      if (!body) return false;

      body.appendChild(this._stashedDiffs);
      this._stashedDiffs = null;
      this._stashedDiffsTheme = null;
      return true;
    },

    _teardownDiffSession() {
      if (this.diffInstances.length) {
        cleanupDiffs(this.diffInstances);
        this.diffInstances = [];
      }

      if (this.diffVirtualizer) {
        try { this.diffVirtualizer.cleanUp(); } catch (e) {}
        this.diffVirtualizer = null;
      }

      this.diffWrapper = null;
      this._stashedDiffs = null;
      this._stashedDiffsTheme = null;

      const container = document.getElementById('context-pane-content');
      if (container) {
        container.querySelectorAll('#' + DIFF_WRAPPER_ID).forEach(el => el.remove());
        container.querySelectorAll('diffs-container').forEach(el => el.remove());
      }
    },

    _cleanup() {
      if (this._abortController) {
        this._abortController.abort();
        this._abortController = null;
      }

      this._teardownDiffSession();

      const selected = document.querySelector('.pull-request.selected');
      if (selected) selected.classList.remove('selected');
    },

    async rerenderDiffs() {
      if (!this.open || !this.prNumber || !this._owner || !this._repo) return;
      if (!this.diffInstances.length) return;

      const container = document.getElementById('context-pane-content');
      const body = container?.querySelector('.context-pane-body');
      if (!body) return;

      this._teardownDiffSession();
      this._showDiffSkeleton(body);

      await this._loadDiffsIntoBody(body);
    },

    async _loadDiffsIntoBody(body) {
      if (!body) return;
      try {
        const session = await loadPRDiffs(body, this._owner, this._repo, this.prNumber);
        body.querySelectorAll('.diff-skeleton').forEach(el => el.remove());
        this.diffInstances = session.instances;
        this.diffVirtualizer = session.virtualizer;
        this.diffWrapper = session.wrapper;
      } catch (e) {
        body.querySelectorAll('.diff-skeleton').forEach(el => el.remove());
      }
    },

    async _fetchBodyAndDiffs(prNumber) {
      this._abortController = new AbortController();
      const signal = this._abortController.signal;

      const container = document.getElementById('context-pane-content');
      const target = container?.querySelector('.context-pane-body') || container;

      if (target) {
        target.querySelectorAll('#' + DIFF_WRAPPER_ID).forEach(el => el.remove());
        target.querySelectorAll('diffs-container').forEach(el => el.remove());
        target.querySelectorAll('.diff-skeleton').forEach(el => el.remove());
        this._showDiffSkeleton(target);
      }

      const bodyPromise = this._fetchBodyRaw(prNumber, signal);
      const diffsPromise = this._fetchDiffsRaw(prNumber, signal);

      try {
        const [bodyHtml, session] = await Promise.all([bodyPromise, diffsPromise]);

        if (signal.aborted) return;

        if (target) {
          target.querySelectorAll('.diff-skeleton').forEach(el => el.remove());
        }

        // Swap in body at the same time
        this.bodyHtml = bodyHtml;
        if (session) {
          this.diffInstances = session.instances;
          this.diffVirtualizer = session.virtualizer;
          this.diffWrapper = session.wrapper;
        }
        this.loadingBody = false;
        this.loadingDiffs = false;

        setTimeout(() => {
          Alpine.store('app')._loadContextPaneImages();
          const body = document.querySelector('.context-pane-body');
          if (body) attachZoom(body.querySelectorAll('img'));
        }, 0);
      } catch (err) {
        if (err.name === 'AbortError') return;
        console.error('[fetch]', err);
        this.bodyHtml = '<div style="color: var(--color-text-muted); font-size: 0.875rem; text-align: center;">Failed to load content</div>';
        this.loadingBody = false;
        this.loadingDiffs = false;
        if (target) {
          target.querySelectorAll('.diff-skeleton').forEach(el => el.remove());
          target.querySelectorAll('#' + DIFF_WRAPPER_ID).forEach(el => el.remove());
        }
      }
    },

    async _fetchBodyRaw(prNumber, signal) {
      const response = await fetch(`/${this._owner}/${this._repo}?part=pr-detail&number=${prNumber}`, { signal });
      if (!response.ok) throw new Error('Failed to fetch body');

      const html = await response.text();

      const parser = new DOMParser();
      const doc = parser.parseFromString(html, 'text/html');
      const bodyElement = doc.querySelector('.context-pane-body');

      if (bodyElement) {
        const meta = bodyElement.querySelector('.context-pane-meta');
        let bodyContent = '';
        let node = bodyElement.firstChild;
        while (node) {
          if (node !== meta && !(node.classList && node.classList.contains('context-pane-meta'))) {
            bodyContent += node.outerHTML || node.textContent;
          }
          node = node.nextSibling;
        }
        return bodyContent;
      }
      return html;
    },

    async _fetchDiffsRaw(prNumber, signal) {
      if (!this._owner || !this._repo) return null;
      const container = document.getElementById('context-pane-content');
      const body = container?.querySelector('.context-pane-body');
      if (!body) return null;
      return await loadPRDiffs(body, this._owner, this._repo, prNumber, signal);
    },

    _openWithDeferredDiffs() {
      const container = document.getElementById('context-pane-content');
      const body = container?.querySelector('.context-pane-body');
      const pane = document.getElementById('context-pane');

      const canRestore = this._stashedDiffs != null && this._stashedDiffsTheme === getDiffTheme();

      const card = document.querySelector(`.pull-request[data-pr-number="${this.prNumber}"]`);

      if (!canRestore) {
        // No stashed session (e.g. closed mid-load) or the theme changed while
        // closed, so fall back to a fresh load of body + diffs.
        this._cleanup();
        this.bodyHtml = this._bodySkeletonHtml();
        this.loadingBody = true;
        this.loadingDiffs = true;
        this.open = true;
        card?.classList.add('selected');
        this._fetchBodyAndDiffs(this.prNumber);
        return;
      }

      this._showDiffSkeleton(body);

      this.open = true;
      card?.classList.add('selected');

      const replaceSkeleton = () => {
        if (body && this.open) {
          const skeleton = body.querySelector('.diff-skeleton');
          if (skeleton) skeleton.remove();

          this._unstashDiffs();
        }
        setTimeout(() => {
          Alpine.store('app')._loadContextPaneImages();
          const body = document.querySelector('.context-pane-body');
          if (body) attachZoom(body.querySelectorAll('img'));
        }, 0);
      };

      this._onTransitionComplete(pane, replaceSkeleton);
    },

    _onTransitionComplete(pane, callback) {
      let handled = false;

      const onTransitionEnd = (e) => {
        if (handled || (pane && e.target !== pane)) return;
        handled = true;
        if (pane) pane.removeEventListener('transitionend', onTransitionEnd);
        callback();
      };

      if (pane) {
        pane.addEventListener('transitionend', onTransitionEnd);
      }

      setTimeout(() => {
        if (!handled) {
          handled = true;
          if (pane) pane.removeEventListener('transitionend', onTransitionEnd);
          callback();
        }
      }, 100);
    },

    _showDiffSkeleton(body) {
      if (!body) return;
      const skeleton = document.createElement('div');
      skeleton.className = 'diff-skeleton skeleton';
      skeleton.style.cssText = 'display: flex; flex-direction: column; gap: 0.75rem;';

      const heights = [100, 140, 80, 120, 160];
      for (const h of heights) {
        const block = document.createElement('div');
        block.className = 'skeleton-line';
        block.style.cssText = `
          width: 100%;
          height: ${h}px;
          background-color: var(--color-decorative);
          border-radius: 4px;
        `;
        skeleton.appendChild(block);
      }
      body.appendChild(skeleton);
    },

    _bodySkeletonHtml() {
      return `
        <div class="body-skeleton skeleton">
          <div class="skeleton-paragraph">
            <div class="skeleton-line"></div>
            <div class="skeleton-line"></div>
            <div class="skeleton-line"></div>
            <div class="skeleton-line"></div>
            <div class="skeleton-line short"></div>
          </div>
          <div class="skeleton-paragraph">
            <div class="skeleton-line"></div>
            <div class="skeleton-line"></div>
            <div class="skeleton-line"></div>
            <div class="skeleton-line medium"></div>
          </div>
          <div class="skeleton-paragraph">
            <div class="skeleton-line"></div>
            <div class="skeleton-line"></div>
            <div class="skeleton-line short"></div>
          </div>
        </div>
      `;
    }
  });

  // Filter panel store
  Alpine.store('filterPanel', {
    open: false,

    openPanel() {
      this._moveFiltersToPanel();
      this.open = true;
      document.body.classList.add('filter-pane-open');
    },

    close() {
      this.open = false;
      document.body.classList.remove('filter-pane-open');
    },

    _moveFiltersToPanel() {
      const panel = document.querySelector('.filter-panel-body');
      if (!panel) return;

      const scopesSection = document.getElementById('scopes-section');
      const contribsSection = document.getElementById('contributors-section');

      if (scopesSection && scopesSection.parentElement !== panel) {
        panel.appendChild(scopesSection);
      }
      if (contribsSection && contribsSection.parentElement !== panel) {
        panel.appendChild(contribsSection);
      }
    }
  });

  Alpine.data('nav', () => ({
    origOwner: '',
    origRepo: '',
    origScope: '',
    _fontsReady: false,

    init() {
      document.fonts.ready.then(() => {
        this._fontsReady = true;
        this._resizeAllInputs();
      });

      this.$nextTick(() => {
        this._captureOriginals();
        this._setupInputs();
      });

      // Load sidebar avatars on initial render
      this.$nextTick(() => {
        Alpine.store('app')._loadAvatars();
      });

      document.addEventListener('htmx:afterSwap', () => {
        this.$nextTick(() => {
          Alpine.store('app')._loadAvatars();
          this._captureOriginals();
          this._setupInputs();
          this._syncScopeFromUrl();
          if (this._fontsReady) {
            this._resizeAllInputs();
          } else {
            document.fonts.ready.then(() => this._resizeAllInputs());
          }
        });
      });
    },

    _captureOriginals() {
      this.origOwner = document.querySelector('.title-input[data-part="owner"]')?.value.trim() || '';
      this.origRepo = document.querySelector('.title-input[data-part="repo"]')?.value.trim() || '';
      this.origScope = document.querySelector('.title-input[data-part="scope"]')?.value.trim() || '';
    },

    _setupInputs() {
      const inputs = document.querySelectorAll('.title-input');
      inputs.forEach(input => {
        this._resize(input);
        const newInput = input.cloneNode(true);
        input.parentNode.replaceChild(newInput, input);

        newInput.addEventListener('input', () => this._resize(newInput));
        newInput.addEventListener('change', () => this._resize(newInput));
        newInput.addEventListener('focus', () => newInput.select());
        newInput.addEventListener('keydown', (ev) => {
          if (ev.key === 'Enter' && newInput.hasAttribute('data-part')) {
            ev.preventDefault();
            this._navigate();
          }
        });
        newInput.addEventListener('blur', () => {
          setTimeout(() => {
            if (document.activeElement?.classList.contains('title-input')) return;
            this._navigate();
          }, 0);
        });
      });
    },

    _resize(input) {
      input.style.width = '1px';
      if (input.value) {
        input.style.width = input.scrollWidth + 'px';
      } else if (input.placeholder) {
        const prev = input.value;
        input.value = input.placeholder;
        input.style.width = input.scrollWidth + 'px';
        input.value = prev;
      }
    },

    _resizeAllInputs() {
      document.querySelectorAll('.title-input').forEach(input => this._resize(input));
    },

    _navigate() {
      const owner = (document.querySelector('.title-input[data-part="owner"]')?.value || '').trim();
      const repo = (document.querySelector('.title-input[data-part="repo"]')?.value || '').trim();
      const scope = (document.querySelector('.title-input[data-part="scope"]')?.value || '').trim();

      if (owner === this.origOwner && repo === this.origRepo && scope === this.origScope) return;

      if (repo) {
        let url = `/${owner}/${repo}`;
        if (owner === this.origOwner && repo === this.origRepo && scope !== this.origScope) {
          if (scope) {
            url += `/${scope}`;
          }
          const params = new URLSearchParams(window.location.search);
          const paramStr = params.toString();
          if (paramStr) {
            url += '?' + paramStr;
          }
        }
        htmx.ajax('GET', url, {
          target: 'main',
          swap: 'innerHTML'
        });
      }
    },

    _syncScopeFromUrl() {
      const parts = window.location.pathname.split('/').filter(Boolean);
      if (parts.length > 2) {
        const curScope = parts.slice(2).join('/');
        if (curScope !== this.origScope) {
          const inp = document.querySelector('.title-input[data-part="scope"]');
          if (inp) {
            inp.value = curScope;
            this.origScope = curScope;
            inp.dispatchEvent(new Event('input'));
          }
        }
      } else {
        const inp = document.querySelector('.title-input[data-part="scope"]');
        if (inp && inp.value) {
          inp.value = '';
          this.origScope = '';
          inp.dispatchEvent(new Event('input'));
        }
      }
    }
  }));
});
