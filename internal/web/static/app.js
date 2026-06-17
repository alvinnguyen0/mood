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
  window.addEventListener("scroll", hide, { passive: true });
})();
