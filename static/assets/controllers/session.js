import { $, status } from "../views/shared.js?v=14";
import { setLandingNavigation } from "../views/navigation.js?v=14";
import { getUser, signOut } from "../requests/users.js?v=14";
export function initSession(pageName, landingPage) {
  async function refreshLandingNavigation() {
    setLandingNavigation(false);
    try {
      const user = await getUser(false);
      if (typeof user?.username === "string") setLandingNavigation(true);
    } catch { /* Keep the public landing page available when signed out or offline. */ }
  }
  if (landingPage) refreshLandingNavigation();
  document.querySelectorAll(".signout-form").forEach(form => {
    form.addEventListener("submit", async event => {
      event.preventDefault();
      const button = $('button[type="submit"]', form);
      if (button.disabled) return;
      button.disabled = true;
      status($("#page-status"), "Signing out…");
      try {
        await signOut(form.action);
        location.replace("/file/signin");
      } catch (error) {
        status($("#page-status"), error.message + " Please try signing out again.", true);
        button.disabled = false;
      }
    });
  });

  // Revalidate pages restored from the browser's back/forward cache after sign-out.
  window.addEventListener("pageshow", event => {
    if (event.persisted && pageName) location.reload();
    else if (event.persisted && landingPage) refreshLandingNavigation();
  });
}
