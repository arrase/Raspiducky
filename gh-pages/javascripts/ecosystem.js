(function () {
  const THEME_KEY = "arrase.theme";

  function applySavedTheme() {
    const saved = localStorage.getItem(THEME_KEY);
    if (saved !== "slate" && saved !== "default") return;

    if (document.body.dataset.mdColorScheme !== saved) {
      document.body.dataset.mdColorScheme = saved;
    }

    const inputs = document.querySelectorAll('input[name="__palette"]');
    inputs.forEach((input) => {
      const scheme = input.dataset.mdColorScheme;
      if (scheme === saved && !input.checked) {
        input.checked = true;
      }
    });
  }

  function bindThemeSync() {
    const paletteForm = document.querySelector('form[data-md-component="palette"]');
    if (paletteForm && !paletteForm.dataset.arraseBound) {
      paletteForm.dataset.arraseBound = "true";
      paletteForm.addEventListener("change", (event) => {
        const scheme = event.target?.dataset?.mdColorScheme;
        if (scheme === "slate" || scheme === "default") {
          localStorage.setItem(THEME_KEY, scheme);
        }
      });
    }
  }

  function setupEcosystemHeader() {
    const path = window.location.pathname;
    const isRootPortal = path === "/" || path === "/index.html";

    if (isRootPortal) {
      document.body.dataset.arrasePortal = "true";
      return;
    }

    delete document.body.dataset.arrasePortal;

    const headerInner = document.querySelector(".md-header__inner");
    if (!headerInner || headerInner.querySelector(".arrase-portal-link")) {
      return;
    }

    const portalLink = document.createElement("a");
    portalLink.href = "https://arrase.github.io/";
    portalLink.className = "arrase-portal-link";
    portalLink.title = "Back to Arrase Projects";
    portalLink.innerHTML =
      '<i class="fa-solid fa-arrow-left" aria-hidden="true"></i><span>All Projects</span>';

    headerInner.insertBefore(portalLink, headerInner.firstChild);
  }

  function initEcosystem() {
    applySavedTheme();
    bindThemeSync();
    setupEcosystemHeader();
  }

  if (document.readyState === "loading") {
    document.addEventListener("DOMContentLoaded", initEcosystem);
  } else {
    initEcosystem();
  }

  if (typeof document$ !== "undefined" && document$.subscribe) {
    document$.subscribe(initEcosystem);
  }
})();
