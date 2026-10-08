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
  if (zoomInstance) return zoomInstance;
  if (zoomLibPromise) return zoomLibPromise;
  zoomLibPromise = import('https://esm.sh/medium-zoom@1.1.0').then(lib => {
    return lib.default;
  });
  return zoomLibPromise;
}

async function attachZoom(selector) {
  const mediumZoom = await ensureZoom();
  if (zoomInstance) zoomInstance.detach();
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

async function loadPullRequestDiffs(container, owner, repository, pullRequestNumber, signal) {
  const lib = await ensureDiffsLib();

  const resp = await fetch(`/${owner}/${repository}/pull/${pullRequestNumber}/diff`, { signal });
  if (!resp.ok) throw new Error('Failed to fetch diff: ' + resp.status);

  const diffText = await resp.text();
  const patches = lib.parsePatchFiles(diffText);

  const instances = [];

  for (const patch of patches) {
    for (const fileDiffMeta of patch.files) {
      const fileDiff = new lib.FileDiff({
        theme: getDiffTheme(),
        diffStyle: 'unified',
        overflow: 'scroll',
        disableLineNumbers: false,
        diffIndicators: 'bars',
      });

      fileDiff.render({ fileDiff: fileDiffMeta, containerWrapper: container });
      instances.push(fileDiff);
    }
  }

  return instances;
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
    pullRequestNumber: null,
    pullRequestTitle: '',
    pullRequestStatusClass: '',
    pullRequestURL: '',
    pullRequestState: '',
    pullRequestAge: '',
    pullRequestAuthorName: '',
    pullRequestAuthorURL: '',
    pullRequestScope: '',
    pullRequestScopeURL: '',
    bodyHtml: '',
    loadingBody: false,
    diffInstances: [],
    loadingDiffs: false,
    _abortController: null,
    _owner: '',
    _repository: '',
    _stashedDiffs: [],

    async openPane(pullRequestNumber, dataset = {}) {
      if (pullRequestNumber === this.pullRequestNumber) {
        if (this.open) {
          this._stashDiffs();
          this.open = false;
        } else {
          this._openWithDeferredDiffs();
        }
        return;
      }

      this._cleanup();

      this.pullRequestNumber = pullRequestNumber;

      this.pullRequestTitle = dataset.pullRequestTitle || 'Pull Request';
      this.pullRequestStatusClass = dataset.pullRequestStatusClass || '';
      this.pullRequestURL = dataset.pullRequestURL || '#';
      this.pullRequestState = dataset.pullRequestState || '';
      this.pullRequestAge = dataset.pullRequestAge || '';
      this.pullRequestAuthorName = dataset.pullRequestAuthorName || '';
      this.pullRequestAuthorURL = dataset.pullRequestAuthorURL || '#';
      this.pullRequestScope = dataset.pullRequestScope || '';
      this.pullRequestScopeURL = dataset.pullRequestScopeURL || '#';

      const card = document.querySelector(`.pull-request[data-pull-request-number="${pullRequestNumber}"]`);
      const owner = card?.closest('[data-owner]')?.dataset.owner;
      const repository = card?.closest('[data-repository]')?.dataset.repository;

      if (!owner || !repository) {
        const pathParts = window.location.pathname.split('/').filter(Boolean);
        if (pathParts.length >= 2) {
          this._owner = pathParts[0];
          this._repository = pathParts[1];
        }
      } else {
        this._owner = owner;
        this._repository = repository;
      }

      this.bodyHtml = this._bodySkeletonHtml();
      this.loadingBody = true;
      this.loadingDiffs = true;

      this.open = true;

      const selected = document.querySelector('.pull-request.selected');
      if (selected) selected.classList.remove('selected');
      card?.classList.add('selected');

      this._fetchBodyAndDiffs(pullRequestNumber);
    },

    closePane() {
      this._stashDiffs();
      this._cleanup();
      if (zoomInstance) {
        zoomInstance.detach();
        zoomInstance = null;
      }

      this.open = false;
    },

    openPrRef(event) {
      const link = event.target.closest('.pull-request-reference');
      if (!link) return;
      event.preventDefault();

      const pullRequestNumber = parseInt(link.dataset.pullRequestNumber, 10);
      if (isNaN(pullRequestNumber)) return;

      const card = document.querySelector(`.pull-request[data-pull-request-number="${pullRequestNumber}"]`);
      if (card) {
        this.openPane(pullRequestNumber, card.dataset);
        return;
      }

      const pathParts = window.location.pathname.split('/').filter(Boolean);
      const owner = pathParts[0] || this._owner;
      const repository = pathParts[1] || this._repository;
      if (!owner || !repository) return;

      link.classList.add('loading');

      this._fetchBodyRaw(pullRequestNumber).then(html => {
        link.classList.remove('loading');

        this._cleanup();
        this.pullRequestNumber = pullRequestNumber;
        this.pullRequestTitle = 'Pull Request #' + pullRequestNumber;
        this.pullRequestStatusClass = '';
        this.pullRequestURL = `https://github.com/${owner}/${repository}/pull/${pullRequestNumber}`;
        this.pullRequestState = '';
        this.pullRequestAge = '';
        this.pullRequestAuthorName = '';
        this.pullRequestAuthorURL = '#';
        this.pullRequestScope = '';
        this.pullRequestScopeURL = '#';
        this._owner = owner;
        this._repository = repository;

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

      body.querySelectorAll('diffs-container').forEach(el => {
        el.remove();
        this._stashedDiffs.push(el);
      });
    },

    _unstashDiffs() {
      if (this._stashedDiffs.length === 0) return false;

      const container = document.getElementById('context-pane-content');
      const body = container?.querySelector('.context-pane-body');
      if (!body) return false;

      this._stashedDiffs.forEach(el => body.appendChild(el));
      this._stashedDiffs = [];
      return true;
    },

    _cleanup() {
      if (this._abortController) {
        this._abortController.abort();
        this._abortController = null;
      }

      if (this.diffInstances.length) {
        cleanupDiffs(this.diffInstances);
        this.diffInstances = [];
      }

      this._stashedDiffs = [];

      const container = document.getElementById('context-pane-content');
      if (container) {
        container.querySelectorAll('diffs-container').forEach(el => el.remove());
      }

      const selected = document.querySelector('.pull-request.selected');
      if (selected) selected.classList.remove('selected');
    },

    async rerenderDiffs() {
      if (!this.open || !this.pullRequestNumber || !this._owner || !this._repository) return;
      if (!this.diffInstances.length) return;

      const container = document.getElementById('context-pane-content');
      const body = container?.querySelector('.context-pane-body');
      if (!body) return;

      cleanupDiffs(this.diffInstances);
      this.diffInstances = [];
      body.querySelectorAll('diffs-container').forEach(el => el.remove());

      this._showDiffSkeleton(body);

      const tempDiffContainer = document.createElement('div');
      try {
        const instances = await loadPullRequestDiffs(tempDiffContainer, this._owner, this._repository, this.pullRequestNumber);

        body.querySelectorAll('.diff-skeleton').forEach(el => el.remove());
        Array.from(tempDiffContainer.children).forEach(el => body.appendChild(el));
        this.diffInstances = instances;
      } catch (e) {
        body.querySelectorAll('.diff-skeleton').forEach(el => el.remove());
      }
    },

    async _fetchBodyAndDiffs(pullRequestNumber) {
      this._abortController = new AbortController();
      const signal = this._abortController.signal;

      const container = document.getElementById('context-pane-content');
      const target = container?.querySelector('.context-pane-body') || container;

      if (target) {
        target.querySelectorAll('diffs-container').forEach(el => el.remove());
        this._showDiffSkeleton(target);
      }

      const tempDiffContainer = document.createElement('div');

      const bodyPromise = this._fetchBodyRaw(pullRequestNumber, signal);
      const diffsPromise = this._fetchDiffsRaw(pullRequestNumber, signal, tempDiffContainer);

      try {
        const [bodyHtml, diffInstances] = await Promise.all([bodyPromise, diffsPromise]);

        if (signal.aborted) return;

        if (target) {
          target.querySelectorAll('.diff-skeleton, diffs-container').forEach(el => el.remove());
          Array.from(tempDiffContainer.children).forEach(el => target.appendChild(el));
        }

        // Swap in body at the same time
        this.bodyHtml = bodyHtml;
        this.diffInstances = diffInstances;
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
        }
      }
    },

    async _fetchBodyRaw(pullRequestNumber, signal) {
      const response = await fetch(`/${this._owner}/${this._repository}?fragment=pr-detail&number=${pullRequestNumber}`, { signal });
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

    async _fetchDiffsRaw(pullRequestNumber, signal, target) {
      if (!this._owner || !this._repository) return [];
      const instances = await loadPullRequestDiffs(target, this._owner, this._repository, pullRequestNumber, signal);
      return instances;
    },

    _openWithDeferredDiffs() {
      const container = document.getElementById('context-pane-content');
      const body = container?.querySelector('.context-pane-body');
      const pane = document.getElementById('context-pane');

      this._showDiffSkeleton(body);

      this.open = true;

      const hasStashed = this._stashedDiffs.length > 0;
      const replaceSkeleton = () => {
        if (body && this.open) {
          const skeleton = body.querySelector('.diff-skeleton');
          if (skeleton) skeleton.remove();

          if (hasStashed) {
            this._unstashDiffs();
          }
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
    origRepository: '',
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
      this.origRepository = document.querySelector('.title-input[data-part="repository"]')?.value.trim() || '';
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
      const repository = (document.querySelector('.title-input[data-part="repository"]')?.value || '').trim();
      const scope = (document.querySelector('.title-input[data-part="scope"]')?.value || '').trim();

      if (owner === this.origOwner && repository === this.origRepository && scope === this.origScope) return;

      if (repository) {
        let url = `/${owner}/${repository}`;
        if (owner === this.origOwner && repository === this.origRepository && scope !== this.origScope) {
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
