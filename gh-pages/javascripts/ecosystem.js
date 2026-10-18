(function () {
  const THEME_KEY = "arrase.theme";

  function applySavedTheme() {
    const saved = localStorage.getItem(THEME_KEY);
    if (saved !== "slate" && saved !== "default") return;

    if (document.body.getAttribute("data-md-color-scheme") !== saved) {
      document.body.setAttribute("data-md-color-scheme", saved);
    }

    const inputs = document.querySelectorAll('input[name="__palette"]');
    inputs.forEach((input) => {
      const scheme = input.getAttribute("data-md-color-scheme");
      if (scheme === saved && !input.checked) {
        input.checked = true;
      }
    });
  }

  function bindThemeSync() {
    const paletteForm = document.querySelector('form[data-md-component="palette"]');
    if (paletteForm && !paletteForm.hasAttribute("data-arrase-bound")) {
      paletteForm.setAttribute("data-arrase-bound", "true");
      paletteForm.addEventListener("change", (event) => {
        const target = event.target;
        if (target && target.getAttribute) {
          const scheme = target.getAttribute("data-md-color-scheme");
          if (scheme === "slate" || scheme === "default") {
            localStorage.setItem(THEME_KEY, scheme);
          }
        }
      });
    }
  }

  function setupEcosystemHeader() {
    const path = window.location.pathname;
    const isRootPortal = path === "/" || path === "/index.html";

    if (isRootPortal) {
      document.body.setAttribute("data-arrase-portal", "true");
      return;
    }

    document.body.removeAttribute("data-arrase-portal");

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
