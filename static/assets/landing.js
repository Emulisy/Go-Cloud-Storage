(function () {
  "use strict";

  const rail = document.querySelector("[data-story-rail]");
  const panels = Array.from(document.querySelectorAll("[data-story-panel]"));
  const progressLinks = Array.from(document.querySelectorAll("[data-panel-link]"));
  const reducedMotion = window.matchMedia("(prefers-reduced-motion: reduce)");

  if (!rail || panels.length === 0) return;

  let frame = 0;

  function updateStory() {
    frame = 0;
    const viewportWidth = Math.max(rail.clientWidth, 1);
    const activeIndex = Math.max(
      0,
      Math.min(panels.length - 1, Math.round(rail.scrollLeft / viewportWidth))
    );

    panels.forEach((panel) => {
      if (reducedMotion.matches) {
        panel.style.removeProperty("--far-x");
        panel.style.removeProperty("--copy-x");
        panel.style.removeProperty("--near-x");
        return;
      }

      const panelOffset = panel.offsetLeft - rail.scrollLeft;
      const progress = Math.max(-1.25, Math.min(1.25, panelOffset / viewportWidth));
      panel.style.setProperty("--far-x", `${progress * -24}px`);
      panel.style.setProperty("--copy-x", `${progress * 18}px`);
      panel.style.setProperty("--near-x", `${progress * 42}px`);
    });

    progressLinks.forEach((link, index) => {
      if (index === activeIndex) {
        link.setAttribute("aria-current", "true");
      } else {
        link.removeAttribute("aria-current");
      }
    });
  }

  function requestUpdate() {
    if (frame) return;
    frame = window.requestAnimationFrame(updateStory);
  }

  progressLinks.forEach((link) => {
    link.addEventListener("click", (event) => {
      const target = document.querySelector(link.getAttribute("href"));
      if (!target) return;
      event.preventDefault();
      target.scrollIntoView({
        behavior: reducedMotion.matches ? "auto" : "smooth",
        block: "nearest",
        inline: "start"
      });
    });
  });

  rail.addEventListener("scroll", requestUpdate, { passive: true });
  window.addEventListener("resize", requestUpdate);
  reducedMotion.addEventListener("change", requestUpdate);
  updateStory();
})();
