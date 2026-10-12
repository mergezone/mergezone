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
let zoomFactory = null;
let zoomLibPromise = null;

async function ensureZoom() {
  if (zoomFactory) return zoomFactory;
  if (!zoomLibPromise) {
    zoomLibPromise = import('https://esm.sh/medium-zoom@1.1.0').then(lib => {
      const factory = lib.default ?? lib.mediumZoom;
      if (typeof factory !== 'function') {
        throw new TypeError('medium-zoom did not export a factory function');
      }
      zoomFactory = factory;
      return zoomFactory;
    }).catch(error => {
      zoomLibPromise = null;
      throw error;
    });
  }
  return zoomLibPromise;
}

async function attachZoom(selector) {
  try {
    const mediumZoom = await ensureZoom();
    if (zoomInstance) {
      try { zoomInstance.detach(); } catch (e) {}
      zoomInstance = null;
    }
    zoomInstance = mediumZoom(selector, {
      background: 'rgba(0,0,0,0.8)',
      margin: 48,
      zIndex: 99999,
    });
    return zoomInstance;
  } catch (error) {
    zoomInstance = null;
    console.warn('[context-pane] image zoom unavailable', error);
    return null;
  }
}

function discussionErrorHtml() {
  return '<p class="context-pane-error">Discussion could not be loaded. <a href="#" data-discussion-retry>Try again</a>.</p>';
}

function discussionSkeletonHtml() {
  return `<div class="discussion-comment discussion-comment-skeleton" aria-hidden="true">
    <div class="discussion-skeleton-block skeleton-block"></div>
    <div class="discussion-skeleton-block skeleton-block"></div>
  </div>`;
}

async function loadPullRequestDiscussion(container, owner, repository, number, signal) {
  const response = await fetch(`/${owner}/${repository}?fragment=pr-discussion&number=${number}`, {
    headers: { 'HX-Request': 'true' },
    signal,
  });
  if (!response.ok) throw new Error(`Failed to load discussion (${response.status})`);

  const html = await response.text();
  const parsed = new DOMParser().parseFromString(html, 'text/html');
  const comments = parsed.querySelector('.discussion-comments');
  if (!comments) throw new Error('Discussion fragment is missing its container');
  comments.dataset.loaded = 'true';
  container.replaceWith(comments);
}

function enhanceContextPaneImages() {
  try {
    Alpine.store('app')._loadContextPaneImages();
  } catch (error) {
    console.warn('[context-pane] image loading unavailable', error);
  }
  const body = document.querySelector('.context-pane-body');
  if (body) attachZoom(body.querySelectorAll('img'));
}

function getDiffTheme() {
  return document.documentElement.classList.contains('dark')
    ? 'pierre-dark'
    : 'pierre-light';
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
    themes: ['light', 'dark'],
    theme: 'dark',
    filterSidebarWidth: 384,
    contextPaneWidth: 384,
    filterSidebarMinWidth: 256,
    filterSidebarMaxWidth: 640,
    contextPaneMinWidth: 320,
    contextPaneMaxWidth: 640,
    _paneResize: null,

    init() {
      const saved = localStorage.getItem('theme');
      if (saved === 'light' || saved === 'white' || saved === 'parchment') {
        this.theme = 'light';
      } else if (saved === 'dark' || saved === 'purple' || saved === 'midnight') {
        this.theme = 'dark';
      } else {
        this.theme = window.matchMedia('(prefers-color-scheme: dark)').matches
          ? 'dark' : 'light';
      }
      document.documentElement.className = this.theme;
      localStorage.setItem('theme', this.theme);

      const rootStyle = getComputedStyle(document.documentElement);
      const remSize = parseFloat(rootStyle.fontSize) || 16;
      const cssLength = (name, fallback) => {
        const value = rootStyle.getPropertyValue(name).trim();
        const amount = parseFloat(value);
        if (!Number.isFinite(amount)) return fallback;
        return value.endsWith('rem') ? amount * remSize : amount;
      };
      this.filterSidebarMinWidth = cssLength('--filter-sidebar-min-width', 256);
      this.filterSidebarMaxWidth = cssLength('--filter-sidebar-max-width', 640);
      this.contextPaneMinWidth = cssLength('--context-pane-min-width', 320);
      this.contextPaneMaxWidth = cssLength('--context-pane-max-width', 640);
      const savedFilterWidth = parseFloat(localStorage.getItem('filter-sidebar-width'));
      const savedContextWidth = parseFloat(localStorage.getItem('context-pane-width'));
      this.setPaneWidth('filter', Number.isFinite(savedFilterWidth)
        ? savedFilterWidth : cssLength('--filter-sidebar-width', 384));
      this.setPaneWidth('context', Number.isFinite(savedContextWidth)
        ? savedContextWidth : cssLength('--context-pane-width', 384));
    },

    cycleTheme() {
      const wasDark = this.theme === 'dark';
      this.theme = wasDark ? 'light' : 'dark';
      const isDark = this.theme === 'dark';
      document.documentElement.className = this.theme;
      localStorage.setItem('theme', this.theme);

      if (wasDark !== isDark) {
        Alpine.store('contextPane').rerenderDiffs();
      }
    },

    setPaneWidth(kind, requestedWidth, persist = false) {
      const minimum = kind === 'filter' ? this.filterSidebarMinWidth : this.contextPaneMinWidth;
      const maximum = Math.max(minimum, kind === 'filter' ? this.filterSidebarMaxWidth : this.contextPaneMaxWidth);
      const width = Math.round(Math.min(maximum, Math.max(minimum, requestedWidth)));
      const property = kind === 'filter' ? '--filter-sidebar-width' : '--context-pane-width';
      const storageKey = kind === 'filter' ? 'filter-sidebar-width' : 'context-pane-width';

      if (kind === 'filter') this.filterSidebarWidth = width;
      else this.contextPaneWidth = width;

      document.documentElement.style.setProperty(property, `${width}px`);
      if (persist) localStorage.setItem(storageKey, String(width));
      return width;
    },

    startPaneResize(kind, event) {
      if (event.button !== 0 || window.innerWidth < 1024) return;
      event.preventDefault();
      this.endPaneResize();

      const selector = kind === 'filter' ? '.aside-column' : '#context-pane';
      const element = document.querySelector(selector);
      if (!element || (kind === 'context' && !Alpine.store('contextPane').open)) return;

      const state = {
        kind,
        pointerId: event.pointerId,
        startX: event.clientX,
        startWidth: element.getBoundingClientRect().width,
      };
      this._paneResize = state;
      document.body.classList.add('pane-resizing');

      try { event.currentTarget.setPointerCapture(event.pointerId); } catch (e) {}

      state.move = (moveEvent) => {
        if (moveEvent.pointerId !== state.pointerId) return;
        const delta = moveEvent.clientX - state.startX;
        const width = kind === 'filter'
          ? state.startWidth + delta
          : state.startWidth - delta;
        this.setPaneWidth(kind, width);
      };
      state.end = (endEvent) => {
        if (endEvent.pointerId !== state.pointerId) return;
        this.endPaneResize(true);
      };

      window.addEventListener('pointermove', state.move);
      window.addEventListener('pointerup', state.end);
      window.addEventListener('pointercancel', state.end);
    },

    adjustPaneWidth(kind, delta) {
      const current = kind === 'filter' ? this.filterSidebarWidth : this.contextPaneWidth;
      this.setPaneWidth(kind, current + delta, true);
    },

    endPaneResize(persist = false) {
      const state = this._paneResize;
      if (!state) return;
      window.removeEventListener('pointermove', state.move);
      window.removeEventListener('pointerup', state.end);
      window.removeEventListener('pointercancel', state.end);
      document.body.classList.remove('pane-resizing');
      if (persist) {
        const width = state.kind === 'filter' ? this.filterSidebarWidth : this.contextPaneWidth;
        const key = state.kind === 'filter' ? 'filter-sidebar-width' : 'context-pane-width';
        localStorage.setItem(key, String(width));
      }
      this._paneResize = null;
    },

    measureSidebarMeta(row) {
      const meta = row?.querySelector('.sidebar-panel-meta');
      if (!meta) return;
      row.style.setProperty('--sidebar-meta-width', `${meta.scrollWidth}px`);
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
    pullRequestComments: '0',
    pullRequestAge: '',
    pullRequestAuthorName: '',
    pullRequestAuthorURL: '',
    pullRequestScope: '',
    pullRequestScopeURL: '',
    bodyHtml: '',
    diffInstances: [],
    loadingDiffs: false,
    changesLoaded: false,
    _discussionController: null,
    _discussionSkeleton: '',
    _owner: '',
    _repository: '',
    _stashedDiffs: [],
    _discussionByPullRequest: new Map(),
    _seedByPullRequest: new Map(),

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

      if (this.pullRequestNumber) {
        const currentDiscussion = document.querySelector('.context-pane-discussion .discussion-comments');
        if (currentDiscussion?.dataset.loaded === 'true') {
          this._discussionByPullRequest.set(this.pullRequestNumber, currentDiscussion.innerHTML);
        }
      }
      this._cleanup();
      this._resetDiscussion();

      let seed = {};
      try {
        const card = document.querySelector(`.pull-request[data-pull-request-number="${pullRequestNumber}"]`);
        const seedNode = card?.querySelector('.pull-request-seed script[type="application/json"]');
        seed = JSON.parse(seedNode?.textContent || dataset.pullRequestSeed || '{}');
        if (Object.keys(seed).length) this._seedByPullRequest.set(pullRequestNumber, seed);
      } catch (error) {
        console.warn('[context-pane] invalid pull request seed', error);
      }

      this.pullRequestNumber = seed.number || pullRequestNumber;
      this.pullRequestTitle = seed.title || dataset.pullRequestTitle || 'Pull Request';
      this.pullRequestStatusClass = seed.statusClass || dataset.pullRequestStatusClass || '';
      this.pullRequestURL = seed.url || dataset.pullRequestURL || '#';
      this.pullRequestState = seed.state || dataset.pullRequestState || '';
      this.pullRequestComments = String(seed.comments ?? dataset.pullRequestComments ?? '0');
      this.pullRequestAge = seed.age || dataset.pullRequestAge || '';
      this.pullRequestAuthorName = seed.authorName || dataset.pullRequestAuthorName || '';
      this.pullRequestAuthorURL = seed.authorURL || dataset.pullRequestAuthorURL || '#';
      this.pullRequestScope = seed.scope || dataset.pullRequestScope || '';
      this.pullRequestScopeURL = seed.scopeURL || dataset.pullRequestScopeURL || '#';
      this._seedByPullRequest.set(pullRequestNumber, seed);

      this._owner = '';
      this._repository = '';
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
      this._owner = seed.owner || this._owner;
      this._repository = seed.repository || this._repository;

      this.bodyHtml = seed.bodyHtml || '';
      this.loadingBody = false;
      this._seedDiscussion(this.pullRequestNumber);
      this.loadingDiffs = false;
      this.changesLoaded = false;

      this.open = true;

      const selected = document.querySelector('.pull-request.selected');
      if (selected) selected.classList.remove('selected');
      card?.classList.add('selected');

      setTimeout(enhanceContextPaneImages, 0);
    },

    closePane() {
      this._stashDiffs();
      this._cleanup();
      this.open = false;
    },

    openPrRef(event) {
      const retry = event.target.closest('[data-discussion-retry]');
      if (retry) {
        event.preventDefault();
        const container = retry.closest('.discussion-comments');
        if (container) {
          container.dataset.loading = 'false';
          container.dataset.loaded = 'false';
        }
        this.loadDiscussion();
        return;
      }

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

      const seeded = this._seedByPullRequest.get(pullRequestNumber);
      if (seeded) {
        this.openPane(pullRequestNumber, { pullRequestSeed: JSON.stringify(seeded) });
        return;
      }

      const pathParts = window.location.pathname.split('/').filter(Boolean);
      const owner = pathParts[0] || this._owner;
      const repository = pathParts[1] || this._repository;
      if (!owner || !repository) return;

      if (this.pullRequestNumber) {
        const currentDiscussion = document.querySelector('.context-pane-discussion .discussion-comments');
        if (currentDiscussion?.dataset.loaded === 'true') {
          this._discussionByPullRequest.set(this.pullRequestNumber, currentDiscussion.innerHTML);
        }
      }
      this._cleanup();
      this._resetDiscussion();
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
      const seed = this._seedByPullRequest.get(pullRequestNumber);
      if (seed) {
        this.pullRequestTitle = seed.title || this.pullRequestTitle;
        this.pullRequestStatusClass = seed.statusClass || '';
        this.pullRequestURL = seed.url || this.pullRequestURL;
        this.pullRequestState = seed.state || '';
        this.pullRequestComments = String(seed.comments ?? 0);
        this.pullRequestAge = seed.age || '';
        this.pullRequestAuthorName = seed.authorName || '';
        this.pullRequestAuthorURL = seed.authorURL || '#';
        this.pullRequestScope = seed.scope || '';
        this.pullRequestScopeURL = seed.scopeURL || '#';
        this.bodyHtml = seed.bodyHtml || '';
        this.loadingBody = false;
        this._seedDiscussion(pullRequestNumber);
        this.open = true;
        setTimeout(enhanceContextPaneImages, 0);
        return;
      }

      this.bodyHtml = '';
      this.loadingBody = false;
      this.loadingDiffs = false;
      this.changesLoaded = false;
      this.open = true;
      setTimeout(enhanceContextPaneImages, 0);
    },

    _stashDiffs() {
      const container = document.getElementById('context-pane-content');
      const diffs = container?.querySelector('.context-pane-diffs');
      if (!diffs) return;

      diffs.querySelectorAll('diffs-container').forEach(el => {
        el.remove();
        this._stashedDiffs.push(el);
      });
    },

    _unstashDiffs() {
      if (this._stashedDiffs.length === 0) return false;

      const container = document.getElementById('context-pane-content');
      const diffs = container?.querySelector('.context-pane-diffs');
      if (!diffs) return false;

      this._stashedDiffs.forEach(el => diffs.appendChild(el));
      this._stashedDiffs = [];
      return true;
    },

    _cleanup() {
      if (this._discussionController) {
        this._discussionController.abort();
        this._discussionController = null;
      }

      if (this.diffInstances.length) {
        cleanupDiffs(this.diffInstances);
        this.diffInstances = [];
      }
      this.changesLoaded = false;
      this.loadingDiffs = false;

      if (zoomInstance) {
        try { zoomInstance.detach(); } catch (error) {
          console.warn('[context-pane] image zoom cleanup failed', error);
        }
        zoomInstance = null;
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
      if (!this.changesLoaded || !this.diffInstances.length) return;

      const container = document.getElementById('context-pane-content');
      const diffs = container?.querySelector('.context-pane-diffs');
      if (!diffs) return;

      cleanupDiffs(this.diffInstances);
      this.diffInstances = [];
      diffs.querySelectorAll('diffs-container').forEach(el => el.remove());

      this._showDiffSkeleton(diffs);

      const tempDiffContainer = document.createElement('div');
      try {
        const instances = await loadPullRequestDiffs(tempDiffContainer, this._owner, this._repository, this.pullRequestNumber);

        diffs.querySelectorAll('.diff-skeleton').forEach(el => el.remove());
        Array.from(tempDiffContainer.children).forEach(el => diffs.appendChild(el));
        this.diffInstances = instances;
        this.loadingDiffs = false;
      } catch (error) {
        console.warn('[context-pane] could not refresh diff', error);
        diffs.querySelectorAll('.diff-skeleton').forEach(el => el.remove());
        this.loadingDiffs = false;
      }
    },

    async loadChanges() {
      if (!this.open || this.changesLoaded || this.loadingDiffs || !this.pullRequestNumber || !this._owner || !this._repository) return;
      const container = document.querySelector('.context-pane-diffs');
      if (!container) return;

      this.loadingDiffs = true;
      this._showDiffSkeleton(container);
      const tempDiffContainer = document.createElement('div');
      try {
        const instances = await this._fetchDiffsRaw(this.pullRequestNumber, undefined, tempDiffContainer);
        if (!this.open || !container.isConnected) {
          cleanupDiffs(instances);
          return;
        }
        container.querySelectorAll('.diff-skeleton').forEach(el => el.remove());
        Array.from(tempDiffContainer.children).forEach(el => container.appendChild(el));
        this.diffInstances = instances;
        this.changesLoaded = true;
      } catch (error) {
        console.warn('[context-pane] diff unavailable', error);
        container.querySelectorAll('.diff-skeleton').forEach(el => el.remove());
        container.innerHTML = '<p class="context-pane-error">Changes could not be loaded.</p>';
      } finally {
        this.loadingDiffs = false;
      }
    },

    _seedDiscussion(pullRequestNumber) {
      const container = document.querySelector('.context-pane-discussion .discussion-comments');
      if (!container) return;
      const cached = this._discussionByPullRequest.get(pullRequestNumber);
      if (cached !== undefined) {
        container.innerHTML = cached;
        container.dataset.loaded = 'true';
        container.dataset.loading = 'false';
      } else {
        this._showDiscussionSkeleton(container);
        container.dataset.loaded = 'false';
        container.dataset.loading = 'false';
      }
    },

    _showDiscussionSkeleton(container) {
      container.innerHTML = discussionSkeletonHtml();
    },

    _resetDiscussion() {
      const container = document.querySelector('.context-pane-discussion .discussion-comments');
      if (!container) return;
      if (container.querySelector('.discussion-comment-skeleton')) {
        this._discussionSkeleton = container.outerHTML;
        container.dataset.loading = 'false';
        return;
      }
      if (this._discussionSkeleton) {
        container.outerHTML = this._discussionSkeleton;
      }
    },

    loadDiscussion() {
      if (!this.open || !this.pullRequestNumber || !this._owner || !this._repository) return;
      this._loadDiscussion();
    },

    _loadDiscussion() {
      if (!this.open || !this.pullRequestNumber || !this._owner || !this._repository) return;
      const container = document.querySelector('.context-pane-discussion .discussion-comments');
      if (!container || container.dataset.loading === 'true' || container.dataset.loaded === 'true') return;

      if (this._discussionController) this._discussionController.abort();
      const controller = new AbortController();
      this._discussionController = controller;
      this._showDiscussionSkeleton(container);
      container.dataset.loading = 'true';
      loadPullRequestDiscussion(container, this._owner, this._repository, this.pullRequestNumber, controller.signal)
        .then(() => {
          this._discussionByPullRequest.set(this.pullRequestNumber, document.querySelector('.context-pane-discussion .discussion-comments')?.innerHTML || '');
        })
        .catch(error => {
          if (controller.signal.aborted || error.name === 'AbortError') return;
          console.warn('[context-pane] could not load discussion fragment', error);
          container.dataset.loading = 'false';
          container.innerHTML = discussionErrorHtml();
        })
        .finally(() => {
          if (this._discussionController === controller) this._discussionController = null;
        });
    },

    async _fetchDiffsRaw(pullRequestNumber, signal, target) {
      if (!this._owner || !this._repository) return [];
      const instances = await loadPullRequestDiffs(target, this._owner, this._repository, pullRequestNumber, signal);
      return instances;
    },

    _openWithDeferredDiffs() {
      const container = document.getElementById('context-pane-content');
      const pane = document.getElementById('context-pane');

      this.open = true;

      const replaceSkeleton = () => {
        setTimeout(enhanceContextPaneImages, 0);
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

    _showDiffSkeleton(container) {
      if (!container) return;
      const skeleton = document.createElement('div');
      skeleton.className = 'diff-skeleton skeleton';
      skeleton.style.cssText = 'display: flex; flex-direction: column; gap: 0.75rem;';

      const heights = [100, 140, 80, 120, 160];
      for (const h of heights) {
        const block = document.createElement('div');
        block.className = 'skeleton-line is-skeleton';
        block.style.cssText = `
          width: 100%;
          height: ${h}px;
        `;
        skeleton.appendChild(block);
      }
      container.appendChild(skeleton);
    },

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

      if (owner === this.origOwner && repository === this.origRepository) return;

      if (repository) {
        let url = `/${owner}/${repository}`;
        htmx.ajax('GET', url, {
          target: 'main',
          swap: 'innerHTML'
        });
      }
    }
  }));
});
