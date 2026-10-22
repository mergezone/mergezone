let diffsLib = null;
let diffsLibPromise = null;

function formatServerDuration(milliseconds) {
  if (!Number.isFinite(milliseconds)) return "—";
  if (milliseconds < 1000) return `${Math.round(milliseconds)}ms`;
  return `${(milliseconds / 1000).toFixed(3).replace(/0+$/, "").replace(/\.$/, "")}s`;
}

function applyStatusTimings(root) {
  if (!root) return;
  const payloads = root.querySelectorAll("[data-status-timings]");
  payloads.forEach((payload) => {
    const metrics = {
      gather: Number(payload.dataset.gatherMs),
      logic: Number(payload.dataset.logicMs),
      template: Number(payload.dataset.templateMs),
    };
    for (const [name, duration] of Object.entries(metrics)) {
      if (!Number.isFinite(duration)) continue;
      // The footer sits outside fragment targets (sidebar, load more, detail),
      // so look it up document-wide instead of within the swapped subtree.
      const target = document.querySelector(`[data-status-${name}]`);
      if (target) target.textContent = formatServerDuration(duration);
    }
    payload.remove();
  });
}

if (document.readyState === "loading") {
  document.addEventListener(
    "DOMContentLoaded",
    () => applyStatusTimings(document),
    {
      once: true,
    },
  );
} else {
  applyStatusTimings(document);
}

document.addEventListener("htmx:afterSwap", (event) => {
  applyStatusTimings(event.detail?.target || document);
});

// Repository navigation replaces the whole view. Close the context pane and
// forget its pull request so the next card click opens fresh instead of
// toggling stale state (which looked like a double click).
function resetContextPaneForNavigation() {
  const contextPane = window.Alpine?.store("contextPane");
  if (!contextPane) return;
  if (contextPane.open) contextPane.closePane();
  contextPane.pullRequestNumber = null;
}

document.addEventListener("htmx:beforeSwap", (event) => {
  if (event.detail?.target?.id === "page-shell") {
    resetContextPaneForNavigation();
  }
});

document.addEventListener("htmx:historyRestore", () => {
  resetContextPaneForNavigation();
});

// A sidebar sort only re-renders the sidebar, so links rendered before the
// sort keep the old query string. When one of those links navigates, copy the
// current sort onto the same-page request. Sort requests are skipped because
// they clear a sort by omitting it.
const SORT_QUERY_PARAMETERS = ["scope_sort", "contributor_sort"];
const SORT_FRAGMENTS = new Set(["scopes", "contributors"]);

function withoutTrailingSlash(pathname) {
  return pathname.replace(/\/+$/, "") || "/";
}

function withCurrentSort(rawURL) {
  const url = new URL(rawURL, window.location.href);
  if (url.origin !== window.location.origin) return null;
  if (
    withoutTrailingSlash(url.pathname) !==
    withoutTrailingSlash(window.location.pathname)
  ) {
    return null;
  }
  if (SORT_FRAGMENTS.has(url.searchParams.get("fragment"))) return null;

  const current = new URLSearchParams(window.location.search);
  let search = url.search;
  for (const name of SORT_QUERY_PARAMETERS) {
    if (url.searchParams.has(name)) continue;
    const value = current.get(name);
    if (!value) continue;
    search += `${search ? "&" : "?"}${encodeURIComponent(name)}=${encodeURIComponent(value)}`;
  }
  return search === url.search ? null : url.pathname + search + url.hash;
}

document.addEventListener("htmx:configRequest", (event) => {
  if (!event.detail?.path) return;
  const url = withCurrentSort(event.detail.path);
  if (url) event.detail.path = url;
});

document.addEventListener("click", (event) => {
  if (event.defaultPrevented || event.button !== 0) return;
  const anchor = event.target.closest?.("a[href]");
  if (!anchor) return;
  const href = anchor.getAttribute("href");
  if (!href || href.startsWith("#")) return;
  const url = withCurrentSort(anchor.href);
  if (url) anchor.setAttribute("href", url);
});

async function ensureDiffsLib() {
  if (diffsLib) return diffsLib;
  if (diffsLibPromise) return diffsLibPromise;
  diffsLibPromise = import("https://esm.sh/@pierre/diffs@1.2.12").then(
    (lib) => {
      diffsLib = lib;
      return lib;
    },
  );
  return diffsLibPromise;
}

let zoomInstance = null;
let zoomFactory = null;
let zoomLibPromise = null;

async function ensureZoom() {
  if (zoomFactory) return zoomFactory;
  if (!zoomLibPromise) {
    zoomLibPromise = import("https://esm.sh/medium-zoom@1.1.0")
      .then((lib) => {
        const factory = lib.default ?? lib.mediumZoom;
        if (typeof factory !== "function") {
          throw new TypeError("medium-zoom did not export a factory function");
        }
        zoomFactory = factory;
        return zoomFactory;
      })
      .catch((error) => {
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
      try {
        zoomInstance.detach();
      } catch (e) {}
      zoomInstance = null;
    }
    zoomInstance = mediumZoom(selector, {
      background: "rgba(0,0,0,0.8)",
      margin: 48,
      zIndex: 99999,
    });
    return zoomInstance;
  } catch (error) {
    zoomInstance = null;
    console.warn("[context-pane] image zoom unavailable", error);
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

async function loadPullRequestDiscussion(
  container,
  owner,
  repository,
  number,
  signal,
) {
  const response = await fetch(
    `/${owner}/${repository}?fragment=pr-discussion&number=${number}`,
    {
      headers: { "HX-Request": "true" },
      signal,
    },
  );
  if (!response.ok)
    throw new Error(`Failed to load discussion (${response.status})`);

  const html = await response.text();
  const parsed = new DOMParser().parseFromString(html, "text/html");
  const comments = parsed.querySelector(".discussion-comments");
  if (!comments)
    throw new Error("Discussion fragment is missing its container");
  comments.dataset.loaded = "true";
  container.replaceWith(comments);
}

function enhanceContextPaneImages() {
  try {
    Alpine.store("app")._loadContextPaneImages();
  } catch (error) {
    console.warn("[context-pane] image loading unavailable", error);
  }
  const body = document.querySelector(".context-pane-body");
  if (body) attachZoom(body.querySelectorAll("img"));
}

function getDiffTheme() {
  return document.documentElement.classList.contains("dark")
    ? "pierre-dark"
    : "pierre-light";
}

async function loadPullRequestDiffs(
  container,
  owner,
  repository,
  pullRequestNumber,
  signal,
) {
  const lib = await ensureDiffsLib();

  const resp = await fetch(
    `/${owner}/${repository}/pull/${pullRequestNumber}/diff`,
    { signal },
  );
  if (!resp.ok) throw new Error("Failed to fetch diff: " + resp.status);

  const diffText = await resp.text();
  const patches = lib.parsePatchFiles(diffText);

  const instances = [];

  for (const patch of patches) {
    for (const fileDiffMeta of patch.files) {
      const fileDiff = new lib.FileDiff({
        theme: getDiffTheme(),
        diffStyle: "unified",
        overflow: "scroll",
        disableLineNumbers: false,
        diffIndicators: "bars",
      });

      fileDiff.render({ fileDiff: fileDiffMeta, containerWrapper: container });
      instances.push(fileDiff);
    }
  }

  return instances;
}

function cleanupDiffs(instances) {
  for (const inst of instances) {
    try {
      inst.cleanUp();
    } catch (e) {}
  }
}

// Relocate the repository search row between the top nav and the mobile bottom
// dock. The nav is re-rendered on every htmx swap, so this runs from the nav
// component and from a single resize listener registered below. The dock is
// present on both home and repository pages, so search placement is consistent.
function syncSearchPlacement() {
  // Target the interactive nav search specifically: the home page also renders
  // a non-interactive demo row, which must stay put.
  const row = document.querySelector(
    "#nav-search-slot .search-input-row, #mobile-dock-search .search-input-row",
  );
  if (!row) return;

  const isMobile = window.matchMedia("(max-width: 1024px)").matches;
  const dock = document.getElementById("mobile-dock-search");
  const target =
    isMobile && dock ? dock : document.getElementById("nav-search-slot");

  if (target && row.parentElement !== target) {
    target.appendChild(row);
  }
}

document.addEventListener("alpine:init", () => {
  window.addEventListener("resize", () => {
    window.clearTimeout(window._searchPlacementTimer);
    window._searchPlacementTimer = window.setTimeout(syncSearchPlacement, 150);
  });

  Alpine.store("app", {
    themes: ["light", "dark"],
    theme: "dark",
    filterSidebarWidth: 384,
    contextPaneWidth: 384,
    filterSidebarMinWidth: 256,
    filterSidebarMaxWidth: 640,
    contextPaneMinWidth: 320,
    contextPaneMaxWidth: 640,
    _paneResize: null,

    init() {
      const saved = localStorage.getItem("theme");
      if (saved === "light" || saved === "white" || saved === "parchment") {
        this.theme = "light";
      } else if (
        saved === "dark" ||
        saved === "purple" ||
        saved === "midnight"
      ) {
        this.theme = "dark";
      } else {
        this.theme = window.matchMedia("(prefers-color-scheme: dark)").matches
          ? "dark"
          : "light";
      }
      document.documentElement.className = this.theme;
      localStorage.setItem("theme", this.theme);

      const rootStyle = getComputedStyle(document.documentElement);
      const remSize = parseFloat(rootStyle.fontSize) || 16;
      const cssLength = (name, fallback) => {
        const value = rootStyle.getPropertyValue(name).trim();
        const amount = parseFloat(value);
        if (!Number.isFinite(amount)) return fallback;
        return value.endsWith("rem") ? amount * remSize : amount;
      };
      this.filterSidebarMinWidth = cssLength("--filter-sidebar-min-width", 256);
      this.filterSidebarMaxWidth = cssLength("--filter-sidebar-max-width", 640);
      this.contextPaneMinWidth = cssLength("--context-pane-min-width", 320);
      this.contextPaneMaxWidth = cssLength("--context-pane-max-width", 640);
      const savedFilterWidth = parseFloat(
        localStorage.getItem("filter-sidebar-width"),
      );
      const savedContextWidth = parseFloat(
        localStorage.getItem("context-pane-width"),
      );
      this.setPaneWidth(
        "filter",
        Number.isFinite(savedFilterWidth)
          ? savedFilterWidth
          : cssLength("--filter-sidebar-width", 384),
      );
      this.setPaneWidth(
        "context",
        Number.isFinite(savedContextWidth)
          ? savedContextWidth
          : cssLength("--context-pane-width", 384),
      );
    },

    cycleTheme() {
      const wasDark = this.theme === "dark";
      this.theme = wasDark ? "light" : "dark";
      const isDark = this.theme === "dark";
      document.documentElement.className = this.theme;
      localStorage.setItem("theme", this.theme);

      if (wasDark !== isDark) {
        Alpine.store("contextPane").rerenderDiffs();
      }
    },

    setPaneWidth(kind, requestedWidth, persist = false) {
      const minimum =
        kind === "filter"
          ? this.filterSidebarMinWidth
          : this.contextPaneMinWidth;
      const maximum = Math.max(
        minimum,
        kind === "filter"
          ? this.filterSidebarMaxWidth
          : this.contextPaneMaxWidth,
      );
      const width = Math.round(
        Math.min(maximum, Math.max(minimum, requestedWidth)),
      );
      const property =
        kind === "filter" ? "--filter-sidebar-width" : "--context-pane-width";
      const storageKey =
        kind === "filter" ? "filter-sidebar-width" : "context-pane-width";

      if (kind === "filter") this.filterSidebarWidth = width;
      else this.contextPaneWidth = width;

      document.documentElement.style.setProperty(property, `${width}px`);
      if (persist) localStorage.setItem(storageKey, String(width));
      return width;
    },

    startPaneResize(kind, event) {
      if (event.button !== 0 || window.innerWidth < 1024) return;
      event.preventDefault();
      this.endPaneResize();

      const selector = kind === "filter" ? ".aside-column" : "#context-pane";
      const element = document.querySelector(selector);
      if (!element || (kind === "context" && !Alpine.store("contextPane").open))
        return;

      const state = {
        kind,
        pointerId: event.pointerId,
        startX: event.clientX,
        startWidth: element.getBoundingClientRect().width,
      };
      this._paneResize = state;
      document.body.classList.add("pane-resizing");

      try {
        event.currentTarget.setPointerCapture(event.pointerId);
      } catch (e) {}

      state.move = (moveEvent) => {
        if (moveEvent.pointerId !== state.pointerId) return;
        const delta = moveEvent.clientX - state.startX;
        const width =
          kind === "filter"
            ? state.startWidth + delta
            : state.startWidth - delta;
        this.setPaneWidth(kind, width);
      };
      state.end = (endEvent) => {
        if (endEvent.pointerId !== state.pointerId) return;
        this.endPaneResize(true);
      };

      window.addEventListener("pointermove", state.move);
      window.addEventListener("pointerup", state.end);
      window.addEventListener("pointercancel", state.end);
    },

    adjustPaneWidth(kind, delta) {
      const current =
        kind === "filter" ? this.filterSidebarWidth : this.contextPaneWidth;
      this.setPaneWidth(kind, current + delta, true);
    },

    endPaneResize(persist = false) {
      const state = this._paneResize;
      if (!state) return;
      window.removeEventListener("pointermove", state.move);
      window.removeEventListener("pointerup", state.end);
      window.removeEventListener("pointercancel", state.end);
      document.body.classList.remove("pane-resizing");
      if (persist) {
        const width =
          state.kind === "filter"
            ? this.filterSidebarWidth
            : this.contextPaneWidth;
        const key =
          state.kind === "filter"
            ? "filter-sidebar-width"
            : "context-pane-width";
        localStorage.setItem(key, String(width));
      }
      this._paneResize = null;
    },

    _loadAvatars() {
      const sidebar = document.querySelector(".aside-column");
      if (!sidebar) return;
      sidebar.querySelectorAll("img[data-src]").forEach((img) => {
        const src = img.dataset.src;
        const preloadImg = new Image();
        preloadImg.onload = () => {
          img.src = src;
          img.removeAttribute("data-src");
          img.classList.add("loaded");
        };
        preloadImg.src = src;
      });
    },

    _loadContextPaneImages() {
      const container = document.getElementById("context-pane-content");
      if (!container) return;
      container.querySelectorAll("img[data-src]").forEach((img) => {
        const src = img.dataset.src;
        const preloadImg = new Image();
        preloadImg.onload = () => {
          img.src = src;
          img.removeAttribute("data-src");
          img.classList.add("loaded");
        };
        preloadImg.src = src;
      });
    },
  });

  Alpine.store("contextPane", {
    open: false,
    pullRequestNumber: null,
    pullRequestTitle: "",
    pullRequestTitleHtml: "",
    pullRequestStatusClass: "",
    pullRequestURL: "",
    pullRequestState: "",
    pullRequestExpiryStatus: "",
    pullRequestExpiryIconClass: "",
    pullRequestExpiryReason: "",
    pullRequestSupersededBy: null,
    pullRequestComments: "0",
    pullRequestAge: "",
    pullRequestAuthorName: "",
    pullRequestAuthorURL: "",
    pullRequestScope: "",
    pullRequestScopeURL: "",
    bodyHtml: "",
    diffInstances: [],
    loadingDiffs: false,
    changesLoaded: false,
    _discussionController: null,
    _discussionSkeleton: "",
    _owner: "",
    _repository: "",
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
        const currentDiscussion = document.querySelector(
          ".context-pane-discussion .discussion-comments",
        );
        if (currentDiscussion?.dataset.loaded === "true") {
          this._discussionByPullRequest.set(
            this.pullRequestNumber,
            currentDiscussion.innerHTML,
          );
        }
      }
      this._cleanup();
      this._resetDiscussion();

      let seed = {};
      try {
        const card = document.querySelector(
          `.pull-request[data-pull-request-number="${pullRequestNumber}"]`,
        );
        const seedNode = card?.querySelector(
          '.pull-request-seed script[type="application/json"]',
        );
        seed = JSON.parse(
          seedNode?.textContent || dataset.pullRequestSeed || "{}",
        );
        if (Object.keys(seed).length)
          this._seedByPullRequest.set(pullRequestNumber, seed);
      } catch (error) {
        console.warn("[context-pane] invalid pull request seed", error);
      }

      this.pullRequestNumber = seed.number || pullRequestNumber;
      this.pullRequestTitle =
        seed.title || dataset.pullRequestTitle || "Pull Request";
      this.pullRequestTitleHtml = seed.titleHtml || this.pullRequestTitle;
      this.pullRequestStatusClass =
        seed.statusClass || dataset.pullRequestStatusClass || "";
      this.pullRequestURL = seed.url || dataset.pullRequestURL || "#";
      this.pullRequestState = seed.state || dataset.pullRequestState || "";
      this.pullRequestExpiryStatus =
        seed.expiryStatus || dataset.pullRequestExpiryStatus || "";
      this.pullRequestExpiryIconClass =
        seed.expiryIconClass || dataset.pullRequestExpiryIconClass || "";
      this.pullRequestExpiryReason =
        seed.expiryReason || dataset.pullRequestExpiryReason || "";
      this.pullRequestSupersededBy = this._supersession(
        seed.supersededBy,
        dataset,
      );
      this.pullRequestComments = String(
        seed.comments ?? dataset.pullRequestComments ?? "0",
      );
      this.pullRequestAge = seed.age || dataset.pullRequestAge || "";
      this.pullRequestAuthorName =
        seed.authorName || dataset.pullRequestAuthorName || "";
      this.pullRequestAuthorURL =
        seed.authorURL || dataset.pullRequestAuthorURL || "#";
      this.pullRequestScope = seed.scope || dataset.pullRequestScope || "";
      this.pullRequestScopeURL =
        seed.scopeURL || dataset.pullRequestScopeURL || "#";
      this._seedByPullRequest.set(pullRequestNumber, seed);

      this._owner = "";
      this._repository = "";
      const card = document.querySelector(
        `.pull-request[data-pull-request-number="${pullRequestNumber}"]`,
      );
      const owner = card?.closest("[data-owner]")?.dataset.owner;
      const repository = card?.closest("[data-repository]")?.dataset.repository;

      if (!owner || !repository) {
        const pathParts = window.location.pathname.split("/").filter(Boolean);
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

      this.bodyHtml = seed.bodyHtml || "";
      this.loadingBody = false;
      this._seedDiscussion(this.pullRequestNumber);
      this.loadingDiffs = false;
      this.changesLoaded = false;

      this.open = true;

      const selected = document.querySelector(".pull-request.selected");
      if (selected) selected.classList.remove("selected");
      card?.classList.add("selected");

      setTimeout(enhanceContextPaneImages, 0);
    },

    closePane() {
      this._stashDiffs();
      this._cleanup();
      this.open = false;
    },

    openPrRef(event) {
      const retry = event.target.closest("[data-discussion-retry]");
      if (retry) {
        event.preventDefault();
        const container = retry.closest(".discussion-comments");
        if (container) {
          container.dataset.loading = "false";
          container.dataset.loaded = "false";
        }
        this.loadDiscussion();
        return;
      }

      const link = event.target.closest(".pull-request-reference");
      if (!link) return;
      event.preventDefault();

      const pullRequestNumber = parseInt(link.dataset.pullRequestNumber, 10);
      if (isNaN(pullRequestNumber)) return;

      const card = document.querySelector(
        `.pull-request[data-pull-request-number="${pullRequestNumber}"]`,
      );
      if (card) {
        this.openPane(pullRequestNumber, card.dataset);
        return;
      }

      const seeded = this._seedByPullRequest.get(pullRequestNumber);
      if (seeded) {
        this.openPane(pullRequestNumber, {
          pullRequestSeed: JSON.stringify(seeded),
        });
        return;
      }

      const pathParts = window.location.pathname.split("/").filter(Boolean);
      const owner = pathParts[0] || this._owner;
      const repository = pathParts[1] || this._repository;
      if (!owner || !repository) return;

      if (this.pullRequestNumber) {
        const currentDiscussion = document.querySelector(
          ".context-pane-discussion .discussion-comments",
        );
        if (currentDiscussion?.dataset.loaded === "true") {
          this._discussionByPullRequest.set(
            this.pullRequestNumber,
            currentDiscussion.innerHTML,
          );
        }
      }
      this._cleanup();
      this._resetDiscussion();
      this.pullRequestNumber = pullRequestNumber;
      this.pullRequestTitle = "Pull Request #" + pullRequestNumber;
      this.pullRequestTitleHtml = this.pullRequestTitle;
      this.pullRequestStatusClass = "";
      this.pullRequestURL = `https://github.com/${owner}/${repository}/pull/${pullRequestNumber}`;
      this.pullRequestState = "";
      this.pullRequestExpiryStatus = "";
      this.pullRequestExpiryIconClass = "";
      this.pullRequestExpiryReason = "";
      this.pullRequestSupersededBy = null;
      this.pullRequestAge = "";
      this.pullRequestAuthorName = "";
      this.pullRequestAuthorURL = "#";
      this.pullRequestScope = "";
      this.pullRequestScopeURL = "#";
      this._owner = owner;
      this._repository = repository;
      const seed = this._seedByPullRequest.get(pullRequestNumber);
      if (seed) {
        this.pullRequestTitle = seed.title || this.pullRequestTitle;
        this.pullRequestTitleHtml = seed.titleHtml || this.pullRequestTitle;
        this.pullRequestStatusClass = seed.statusClass || "";
        this.pullRequestURL = seed.url || this.pullRequestURL;
        this.pullRequestState = seed.state || "";
        this.pullRequestExpiryStatus = seed.expiryStatus || "";
        this.pullRequestExpiryIconClass = seed.expiryIconClass || "";
        this.pullRequestExpiryReason = seed.expiryReason || "";
        this.pullRequestSupersededBy = this._supersession(seed.supersededBy);
        this.pullRequestComments = String(seed.comments ?? 0);
        this.pullRequestAge = seed.age || "";
        this.pullRequestAuthorName = seed.authorName || "";
        this.pullRequestAuthorURL = seed.authorURL || "#";
        this.pullRequestScope = seed.scope || "";
        this.pullRequestScopeURL = seed.scopeURL || "#";
        this.bodyHtml = seed.bodyHtml || "";
        this.loadingBody = false;
        this._seedDiscussion(pullRequestNumber);
        this.open = true;
        setTimeout(enhanceContextPaneImages, 0);
        return;
      }

      this.bodyHtml = "";
      this.loadingBody = false;
      this.loadingDiffs = false;
      this.changesLoaded = false;
      this.open = true;
      setTimeout(enhanceContextPaneImages, 0);
    },

    _supersession(seedSupersededBy, dataset = {}) {
      const number =
        seedSupersededBy?.number ?? dataset.pullRequestSupersededByNumber;
      if (number === undefined || number === null || number === "") return null;
      return {
        number: String(number),
        url: seedSupersededBy?.url || dataset.pullRequestSupersededByUrl || "#",
        reason:
          seedSupersededBy?.reason ||
          dataset.pullRequestSupersededByReason ||
          "is newer",
      };
    },

    _stashDiffs() {
      const container = document.getElementById("context-pane-content");
      const diffs = container?.querySelector(".context-pane-diffs");
      if (!diffs) return;

      diffs.querySelectorAll("diffs-container").forEach((el) => {
        el.remove();
        this._stashedDiffs.push(el);
      });
    },

    _unstashDiffs() {
      if (this._stashedDiffs.length === 0) return false;

      const container = document.getElementById("context-pane-content");
      const diffs = container?.querySelector(".context-pane-diffs");
      if (!diffs) return false;

      this._stashedDiffs.forEach((el) => diffs.appendChild(el));
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
        try {
          zoomInstance.detach();
        } catch (error) {
          console.warn("[context-pane] image zoom cleanup failed", error);
        }
        zoomInstance = null;
      }

      this._stashedDiffs = [];

      const container = document.getElementById("context-pane-content");
      if (container) {
        container
          .querySelectorAll("diffs-container")
          .forEach((el) => el.remove());
      }

      const selected = document.querySelector(".pull-request.selected");
      if (selected) selected.classList.remove("selected");
    },

    async rerenderDiffs() {
      if (
        !this.open ||
        !this.pullRequestNumber ||
        !this._owner ||
        !this._repository
      )
        return;
      if (!this.changesLoaded || !this.diffInstances.length) return;

      const container = document.getElementById("context-pane-content");
      const diffs = container?.querySelector(".context-pane-diffs");
      if (!diffs) return;

      cleanupDiffs(this.diffInstances);
      this.diffInstances = [];
      diffs.querySelectorAll("diffs-container").forEach((el) => el.remove());

      this._showDiffSkeleton(diffs);

      const tempDiffContainer = document.createElement("div");
      try {
        const instances = await loadPullRequestDiffs(
          tempDiffContainer,
          this._owner,
          this._repository,
          this.pullRequestNumber,
        );

        diffs.querySelectorAll(".diff-skeleton").forEach((el) => el.remove());
        Array.from(tempDiffContainer.children).forEach((el) =>
          diffs.appendChild(el),
        );
        this.diffInstances = instances;
        this.loadingDiffs = false;
      } catch (error) {
        console.warn("[context-pane] could not refresh diff", error);
        diffs.querySelectorAll(".diff-skeleton").forEach((el) => el.remove());
        this.loadingDiffs = false;
      }
    },

    async loadChanges() {
      if (
        !this.open ||
        this.changesLoaded ||
        this.loadingDiffs ||
        !this.pullRequestNumber ||
        !this._owner ||
        !this._repository
      )
        return;
      const container = document.querySelector(".context-pane-diffs");
      if (!container) return;

      this.loadingDiffs = true;
      this._showDiffSkeleton(container);
      const tempDiffContainer = document.createElement("div");
      try {
        const instances = await this._fetchDiffsRaw(
          this.pullRequestNumber,
          undefined,
          tempDiffContainer,
        );
        if (!this.open || !container.isConnected) {
          cleanupDiffs(instances);
          return;
        }
        container
          .querySelectorAll(".diff-skeleton")
          .forEach((el) => el.remove());
        Array.from(tempDiffContainer.children).forEach((el) =>
          container.appendChild(el),
        );
        this.diffInstances = instances;
        this.changesLoaded = true;
      } catch (error) {
        console.warn("[context-pane] diff unavailable", error);
        container
          .querySelectorAll(".diff-skeleton")
          .forEach((el) => el.remove());
        container.innerHTML =
          '<p class="context-pane-error">Changes could not be loaded.</p>';
      } finally {
        this.loadingDiffs = false;
      }
    },

    _seedDiscussion(pullRequestNumber) {
      const container = document.querySelector(
        ".context-pane-discussion .discussion-comments",
      );
      if (!container) return;
      const cached = this._discussionByPullRequest.get(pullRequestNumber);
      if (cached !== undefined) {
        container.innerHTML = cached;
        container.dataset.loaded = "true";
        container.dataset.loading = "false";
      } else {
        this._showDiscussionSkeleton(container);
        container.dataset.loaded = "false";
        container.dataset.loading = "false";
      }
    },

    _showDiscussionSkeleton(container) {
      container.innerHTML = discussionSkeletonHtml();
    },

    _resetDiscussion() {
      const container = document.querySelector(
        ".context-pane-discussion .discussion-comments",
      );
      if (!container) return;
      if (container.querySelector(".discussion-comment-skeleton")) {
        this._discussionSkeleton = container.outerHTML;
        container.dataset.loading = "false";
        return;
      }
      if (this._discussionSkeleton) {
        container.outerHTML = this._discussionSkeleton;
      }
    },

    loadDiscussion() {
      if (
        !this.open ||
        !this.pullRequestNumber ||
        !this._owner ||
        !this._repository
      )
        return;
      this._loadDiscussion();
    },

    _loadDiscussion() {
      if (
        !this.open ||
        !this.pullRequestNumber ||
        !this._owner ||
        !this._repository
      )
        return;
      const container = document.querySelector(
        ".context-pane-discussion .discussion-comments",
      );
      if (
        !container ||
        container.dataset.loading === "true" ||
        container.dataset.loaded === "true"
      )
        return;

      if (this._discussionController) this._discussionController.abort();
      const controller = new AbortController();
      this._discussionController = controller;
      this._showDiscussionSkeleton(container);
      container.dataset.loading = "true";
      loadPullRequestDiscussion(
        container,
        this._owner,
        this._repository,
        this.pullRequestNumber,
        controller.signal,
      )
        .then(() => {
          this._discussionByPullRequest.set(
            this.pullRequestNumber,
            document.querySelector(
              ".context-pane-discussion .discussion-comments",
            )?.innerHTML || "",
          );
        })
        .catch((error) => {
          if (controller.signal.aborted || error.name === "AbortError") return;
          console.warn(
            "[context-pane] could not load discussion fragment",
            error,
          );
          container.dataset.loading = "false";
          container.innerHTML = discussionErrorHtml();
        })
        .finally(() => {
          if (this._discussionController === controller)
            this._discussionController = null;
        });
    },

    async _fetchDiffsRaw(pullRequestNumber, signal, target) {
      if (!this._owner || !this._repository) return [];
      const instances = await loadPullRequestDiffs(
        target,
        this._owner,
        this._repository,
        pullRequestNumber,
        signal,
      );
      return instances;
    },

    _openWithDeferredDiffs() {
      const container = document.getElementById("context-pane-content");
      const pane = document.getElementById("context-pane");

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
        if (pane) pane.removeEventListener("transitionend", onTransitionEnd);
        callback();
      };

      if (pane) {
        pane.addEventListener("transitionend", onTransitionEnd);
      }

      setTimeout(() => {
        if (!handled) {
          handled = true;
          if (pane) pane.removeEventListener("transitionend", onTransitionEnd);
          callback();
        }
      }, 100);
    },

    _showDiffSkeleton(container) {
      if (!container) return;
      const skeleton = document.createElement("div");
      skeleton.className = "diff-skeleton skeleton";
      skeleton.style.cssText =
        "display: flex; flex-direction: column; gap: 0.75rem;";

      const heights = [100, 140, 80, 120, 160];
      for (const h of heights) {
        const block = document.createElement("div");
        block.className = "skeleton-line is-skeleton";
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
  Alpine.store("filterPanel", {
    open: false,

    toggle() {
      if (this.open) this.close();
      else this.openPanel();
    },

    openPanel() {
      this._moveFiltersToPanel();
      this.open = true;
      document.body.classList.add("filter-pane-open");
    },

    close() {
      this.open = false;
      document.body.classList.remove("filter-pane-open");
    },

    _moveFiltersToPanel() {
      const panel = document.querySelector(".filter-panel-body");
      if (!panel) return;

      const scopesSection = document.getElementById("scopes-section");
      const contribsSection = document.getElementById("contributors-section");

      if (scopesSection && scopesSection.parentElement !== panel) {
        panel.appendChild(scopesSection);
      }
      if (contribsSection && contribsSection.parentElement !== panel) {
        panel.appendChild(contribsSection);
      }
    },
  });

  Alpine.data("nav", () => ({
    origOwner: "",
    origRepository: "",
    ownerEditing: false,
    repositoryEditing: false,
    _fontsReady: false,

    startOwnerEdit() {
      this.repositoryEditing = false;
      this.ownerEditing = true;
      this._focusEditor("ownerInput");
    },

    startRepositoryEdit() {
      this.repositoryEditing = true;
      this._focusEditor("repositoryInput");
    },

    _focusEditor(refName) {
      this.$nextTick(() => {
        requestAnimationFrame(() => {
          const input = this.$refs[refName];
          if (!input) return;
          input.focus();
          input.select();
          this._resize(input);
        });
      });
    },

    init() {
      document.fonts.ready.then(() => {
        this._fontsReady = true;
        this._resizeAllInputs();
      });

      // Place the search row before measuring it: on mobile the nav slot is
      // hidden, so inputs must already be in the visible dock to size correctly.
      this.$nextTick(() => {
        this._captureOriginals();
        syncSearchPlacement();
        this._setupInputs();
      });

      // Load sidebar avatars on initial render
      this.$nextTick(() => {
        Alpine.store("app")._loadAvatars();
      });

      document.addEventListener("htmx:afterSwap", () => {
        this.$nextTick(() => {
          Alpine.store("app")._loadAvatars();
          this._captureOriginals();
          syncSearchPlacement();
          this._setupInputs();
          if (this._fontsReady) {
            this._resizeAllInputs();
          } else {
            document.fonts.ready.then(() => this._resizeAllInputs());
          }
        });
      });
    },

    _captureOriginals() {
      this.origOwner =
        document
          .querySelector('.title-input[data-part="owner"]')
          ?.value.trim() || "";
      this.origRepository =
        document
          .querySelector('.title-input[data-part="repository"]')
          ?.value.trim() || "";
    },

    _setupInputs() {
      const inputs = document.querySelectorAll(".title-input");
      inputs.forEach((input) => {
        this._resize(input);
        if (input.dataset.navSetup === "true") return;
        input.dataset.navSetup = "true";

        input.addEventListener("input", () => this._resize(input));
        input.addEventListener("change", () => this._resize(input));
        input.addEventListener("focus", () => {
          this._resize(input);
          input.select();
          if (input.dataset.part === "owner") this.ownerEditing = true;
          if (input.dataset.part === "repository")
            this.repositoryEditing = true;
        });
        input.addEventListener("keydown", (ev) => {
          if (ev.key === "Enter" && input.hasAttribute("data-part")) {
            ev.preventDefault();
            this.ownerEditing = false;
            this.repositoryEditing = false;
            this._navigate(input);
          }
        });
        input.addEventListener("blur", (event) => {
          const relatedTarget = event.relatedTarget;
          setTimeout(() => {
            const searchBar = input.closest(".search-input-row");
            if (
              searchBar?.contains(relatedTarget) ||
              searchBar?.contains(document.activeElement)
            )
              return;
            this.ownerEditing = false;
            this.repositoryEditing = false;
            this._navigate(input);
          }, 0);
        });
      });
    },

    _resize(input) {
      const styles = getComputedStyle(input);
      const canvas = document.createElement("canvas");
      const context = canvas.getContext("2d");
      if (!context) return;

      context.font = styles.font;
      const text = input.value || input.placeholder || "";
      const letterSpacing = parseFloat(styles.letterSpacing) || 0;
      const textWidth =
        context.measureText(text).width +
        Math.max(0, text.length - 1) * letterSpacing;
      const horizontalSpace =
        parseFloat(styles.paddingLeft) +
        parseFloat(styles.paddingRight) +
        parseFloat(styles.borderLeftWidth) +
        parseFloat(styles.borderRightWidth);
      input.style.width = `${Math.ceil(textWidth + horizontalSpace + 2)}px`;
    },

    _resizeAllInputs() {
      document
        .querySelectorAll(".title-input")
        .forEach((input) => this._resize(input));
    },

    _navigate() {
      const owner = (
        document.querySelector('.title-input[data-part="owner"]')?.value || ""
      ).trim();
      const repository = (
        document.querySelector('.title-input[data-part="repository"]')?.value ||
        ""
      ).trim();

      if (owner === this.origOwner && repository === this.origRepository)
        return;

      if (repository) {
        htmx.ajax("GET", `/${owner}/${repository}`, {
          source: document.querySelector(".search-input-row"),
          target: "#page-shell",
          swap: "innerHTML",
        });
      }
    },
  }));
});
