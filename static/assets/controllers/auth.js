import { $, status } from "../views/shared.js?v=14";
import { validate } from "../views/forms.js?v=14";
import { authenticate } from "../requests/users.js?v=14";
export function initAuth() {
  const authForm = $("#auth-form");
  if (authForm) {
    if (authForm.dataset.mode === "login") {
      let savedEmail;
      try { savedEmail = sessionStorage.getItem("signupEmail"); } catch { /* Storage is optional. */ }
      if (savedEmail) {
        $("#auth-email").value = savedEmail;
        try { sessionStorage.removeItem("signupEmail"); } catch { /* Storage is optional. */ }
        status($("#auth-status"), "Account created. Sign in to open your files.");
      }
    }
    authForm.addEventListener("submit", async event => {
      event.preventDefault();
      if (!validate(authForm)) return;
      const body = new URLSearchParams(new FormData(authForm));
      body.delete("confirmPassword");
      body.set("email", body.get("email").trim().toLowerCase());
      const button = $('button[type="submit"]', authForm);
      button.disabled = true;
      status($("#auth-status"), authForm.dataset.mode === "signup" ? "Creating your account…" : "Signing in…");
      try {
        await authenticate(authForm.action, body);
        if (authForm.dataset.mode === "signup") {
          try { sessionStorage.setItem("signupEmail", body.get("email")); } catch { /* Still navigate after account creation. */ }
          location.assign("/file/signin");
        } else location.assign("/file/home");
      } catch (error) {
        status($("#auth-status"), error.message, true);
      } finally {
        button.disabled = false;
      }
    });
  }
}
