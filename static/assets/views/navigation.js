import { $ } from "./shared.js?v=14";
export function initNavigation() {
  const sidebar = $("#app-sidebar"), sidebarToggle = $("#sidebar-toggle");
  function closeSidebar(returnFocus = true) {
    if (!sidebar) return;
    sidebar.classList.remove("is-open");
    sidebarToggle.setAttribute("aria-expanded", "false");
    $(".sidebar-backdrop").hidden = true;
    document.body.classList.remove("navigation-open");
    $(".app-main").inert = false;
    sidebar.removeAttribute("role");
    sidebar.removeAttribute("aria-modal");
    if (returnFocus) sidebarToggle.focus();
  }
  if (sidebar) {
    sidebarToggle.addEventListener("click", () => {
      sidebar.classList.add("is-open");
      sidebar.setAttribute("role", "dialog");
      sidebar.setAttribute("aria-modal", "true");
      sidebarToggle.setAttribute("aria-expanded", "true");
      $(".sidebar-backdrop").hidden = false;
      document.body.classList.add("navigation-open");
      $(".app-main").inert = true;
      $(".mobile-close", sidebar).focus();
    });
    document.querySelectorAll("[data-close-sidebar]").forEach(button => button.addEventListener("click", () => closeSidebar()));
    sidebar.addEventListener("keydown", event => {
      if (!sidebar.classList.contains("is-open")) return;
      if (event.key === "Escape") { event.preventDefault(); closeSidebar(); }
      if (event.key === "Tab") {
        const items = [...sidebar.querySelectorAll('a, button:not(:disabled)')].filter(node => node.getClientRects().length);
        const first = items[0], last = items.at(-1);
        if (event.shiftKey && document.activeElement === first) { event.preventDefault(); last.focus(); }
        else if (!event.shiftKey && document.activeElement === last) { event.preventDefault(); first.focus(); }
      }
    });
    matchMedia("(max-width: 760px)").addEventListener("change", () => closeSidebar(false));
  }


  document.addEventListener("click", event => {
    const menu = event.target.closest(".file-menu");
    closeFileMenus(menu);
  });
}

export function setLandingNavigation(signedIn) {
  document.querySelectorAll("[data-guest-only]").forEach(node => { node.hidden = signedIn; });
  document.querySelectorAll("[data-user-only]").forEach(node => { node.hidden = !signedIn; });
  $("[data-workspace-link]").href = signedIn ? "/file/home" : "/file/signup";
  $("[data-workspace-label]").textContent = signedIn ? "Open my files" : "Create your workspace";
}

export function closeFileMenus(except) {
  document.querySelectorAll(".file-menu[open]").forEach(menu => {
    if (menu !== except) { menu.open = false; $("summary", menu).setAttribute("aria-expanded", "false"); }
  });
}
