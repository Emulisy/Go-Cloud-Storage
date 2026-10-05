import { $, status } from "../views/shared.js?v=14";
import { validate } from "../views/forms.js?v=14";
import { paintUser, fillProfile, enableAccountForms } from "../views/user.js?v=14";
import { getUser, updateUser } from "../requests/users.js?v=14";
export function initAccount(pageName) {
  async function loadUser() {
    const user = await getUser();
    paintUser(user);
    return user;
  }

  if (pageName) {
    loadUser().then(user => {
      if (pageName !== "account") return;
      fillProfile(user);
      enableAccountForms();
    }).catch(error => status($("#page-status"), error.message + " Refresh the page to retry.", true));
  }

  document.querySelectorAll(".account-form").forEach(form => {
    form.addEventListener("submit", async event => {
      event.preventDefault();
      if (!validate(form)) return;
      const body = new URLSearchParams(new FormData(form));
      body.delete("confirmPassword");
      const kind = form.dataset.update;
      if (kind === "name") body.set("userName", body.get("userName").trim());
      if (kind === "email") body.set("email", body.get("email").trim().toLowerCase());
      const fieldset = $("fieldset", form), output = $(".status", form);
      fieldset.disabled = true;
      status(output, "Saving…");
      try {
        await updateUser(kind, body);
        if (kind === "password") {
          form.reset();
          status(output, "Password updated.");
        } else {
          const user = await loadUser();
          fillProfile(user);
          status(output, kind === "email" ? "Email updated. Use this address to sign in." : "Display name updated.");
        }
      } catch (error) {
        status(output, error.message, true);
      } finally {
        fieldset.disabled = false;
      }
    });
  });
}
