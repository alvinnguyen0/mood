// Lightweight tooltip for activity-grid cells. Works on hover (desktop) and
// tap (touch). Uses event delegation so it also covers htmx-swapped content.
(function () {
  var tip = document.getElementById("tip");
  if (!tip) return;

  function place(el) {
    var r = el.getBoundingClientRect();
    tip.textContent = el.getAttribute("data-tip");
    tip.style.left = (window.scrollX + r.left + r.width / 2) + "px";
    tip.style.top = (window.scrollY + r.top - 6) + "px";
    tip.hidden = false;
  }
  function hide() { tip.hidden = true; }

  document.addEventListener("mouseover", function (e) {
    var el = e.target.closest(".cell[data-tip]");
    if (el) place(el);
  });
  document.addEventListener("mouseout", function (e) {
    if (e.target.closest(".cell[data-tip]")) hide();
  });
  document.addEventListener("click", function (e) {
    var el = e.target.closest(".cell[data-tip]");
    if (el) { place(el); } else { hide(); }
  });

  var touchTimer;
  document.addEventListener("touchstart", function (e) {
    var el = e.target.closest(".cell[data-tip]");
    if (el) {
      e.preventDefault();
      place(el);
    }
  }, { passive: false });
  document.addEventListener("touchend", function (e) {
    if (e.target.closest(".cell[data-tip]")) {
      clearTimeout(touchTimer);
      touchTimer = setTimeout(hide, 1500);
    }
  });
  document.addEventListener("touchcancel", function () {
    clearTimeout(touchTimer);
    hide();
  });

  window.addEventListener("scroll", hide, { passive: true });
})();

// Theme toggle — syncs with the inline script in <head> that sets data-theme
// on load to avoid flash of wrong theme.
(function () {
  var btn = document.getElementById("theme-toggle");
  if (!btn) return;

  function icon(theme) { return theme === "dark" ? "☀︎" : "☾"; }

  function apply(theme) {
    document.documentElement.setAttribute("data-theme", theme);
    btn.textContent = icon(theme);
    btn.setAttribute("aria-label", theme === "dark" ? "switch to light mode" : "switch to dark mode");
  }

  var current = document.documentElement.getAttribute("data-theme") || "light";
  apply(current);

  btn.addEventListener("click", function () {
    var next = document.documentElement.getAttribute("data-theme") === "dark" ? "light" : "dark";
    apply(next);
    try { localStorage.setItem("theme", next); } catch (e) {}
  });
})();
