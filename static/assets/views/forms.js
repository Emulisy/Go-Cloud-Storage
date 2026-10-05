import { $, bytes } from "./shared.js?v=14";
export function validate(form) {
  for (const input of form.querySelectorAll("input")) input.setCustomValidity("");
  for (const input of form.querySelectorAll('input[name="userName"], input[type="password"]')) {
    if (input.name === "confirmPassword") continue;
    const isName = input.name === "userName";
    const length = bytes(isName ? input.value.trim() : input.value);
    const min = isName ? 3 : 5, max = isName ? 64 : 72;
    if (length < min || length > max) input.setCustomValidity(`Use between ${min} and ${max} bytes.`);
  }
  const confirm = $('[name="confirmPassword"]', form);
  const password = $('[name="newPwd"], [name="userPwd"]', form);
  if (confirm && password && confirm.value !== password.value) confirm.setCustomValidity("The passwords do not match.");
  return form.reportValidity();
}
export function initFormValidation() {
  document.addEventListener("input", event => {
    if (event.target instanceof HTMLInputElement) event.target.setCustomValidity("");
    const confirm = event.target.form && $('[name="confirmPassword"]', event.target.form);
    if (confirm) confirm.setCustomValidity("");
  });
}
