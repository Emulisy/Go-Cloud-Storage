import { $, dateLabel } from "./shared.js?v=14";
export function paintUser(user) {
  document.querySelectorAll("[data-username]").forEach(node => { node.textContent = user.username; });
  document.querySelectorAll("[data-avatar]").forEach(node => {
    node.textContent = Array.from(user.username || "").slice(0, 2).join("").toUpperCase();
  });
  if ($("#account-email")) $("#account-email").textContent = user.email;
  document.querySelectorAll(".profile-email").forEach(node => { node.textContent = user.email; });
  if ($("#account-signup")) $("#account-signup").textContent = dateLabel(user.signupAt);
  if ($("#account-active")) $("#account-active").textContent = dateLabel(user.lastActive);
}
export function fillProfile(user) {
  $("#profile-name").value = user.username;
  $("#profile-email").value = user.email;
}

export function enableAccountForms() {
  document.querySelectorAll(".account-form fieldset").forEach(node => { node.disabled = false; });
}
