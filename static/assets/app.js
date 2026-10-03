(() => {
  "use strict";
  const $ = (selector, root = document) => root.querySelector(selector);
  const bytes = value => new TextEncoder().encode(value).length;
  const pageName = document.body.dataset.page;
  const landingPage = document.body.classList.contains("welcome-page");
  const ordinaryLimit = 100 * 1024 * 1024;

  // Small, shared inline icons; user content never enters SVG markup.
  const iconPaths = {
    file: '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8Z"/><path d="M14 2v6h6M8 13h8M8 17h5"/>',
    more: '<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>',
    download: '<path d="M12 3v12m-4-4 4 4 4-4M4 17v3a1 1 0 0 0 1 1h14a1 1 0 0 0 1-1v-3"/>',
    rename: '<path d="m15 4 5 5M4 20l5-1L20 8a2 2 0 0 0-5-5L4 14Z"/>',
    delete: '<path d="M3 6h18M9 6V3h6v3M5 6l1 15h12l1-15M10 10v7M14 10v7"/>'
  };
  function makeIcon(kind) {
    const node = document.createElementNS("http://www.w3.org/2000/svg", "svg");
    for (const [key, value] of Object.entries({ viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", "stroke-width": "2", "stroke-linecap": "round", "stroke-linejoin": "round", "aria-hidden": "true", class: "icon" })) node.setAttribute(key, value);
    node.innerHTML = iconPaths[kind];
    return node;
  }

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

  function closeFileMenus(except) {
    document.querySelectorAll(".file-menu[open]").forEach(menu => {
      if (menu !== except) { menu.open = false; $("summary", menu).setAttribute("aria-expanded", "false"); }
    });
  }
  document.addEventListener("click", event => {
    const menu = event.target.closest(".file-menu");
    closeFileMenus(menu);
  });

  function status(node, message = "", error = false) {
    if (!node) return;
    node.textContent = message;
    node.classList.toggle("is-error", error);
    if (node.id === "upload-status" && $("#upload-form")?.getAttribute("aria-busy") === "true") {
      const button = $('#upload-form button[type="submit"]');
      button.textContent = /Verifying/.test(message) ? "Verifying…" : /Preparing|Checking/.test(message) ? "Preparing…" : "Uploading…";
    }
  }

  function sizeLabel(size) {
    if (!Number.isFinite(size) || size < 0) return "—";
    if (size < 1024) return `${size} B`;
    const units = ["KiB", "MiB", "GiB", "TiB"];
    let value = size / 1024, i = 0;
    while (value >= 1024 && i < units.length - 1) {
      value /= 1024;
      i++;
    }
    return `${value.toLocaleString(undefined, { maximumFractionDigits: 1 })} ${units[i]}`;
  }

  function dateLabel(value) {
    const date = new Date(value);
    return Number.isNaN(date.getTime()) ? "—" : date.toLocaleString(undefined, {
      year: "numeric",
      month: "short",
      day: "numeric",
      hour: "numeric",
      minute: "2-digit"
    });
  }

  async function request(url, options = {}, redirect = true) {
    const response = await fetch(url, { credentials: "same-origin", cache: "no-store", ...options });
    if (response.status === 401 && redirect) {
      location.assign("/file/signin");
      throw new Error("Your session has expired. Please sign in again.");
    }
    if (!response.ok) {
      const message = (await response.text()).trim();
      throw new Error(message || "The request failed. Please try again.");
    }
    return response;
  }

  function setLandingNavigation(signedIn) {
    document.querySelectorAll("[data-guest-only]").forEach(node => { node.hidden = signedIn; });
    document.querySelectorAll("[data-user-only]").forEach(node => { node.hidden = !signedIn; });
    $("[data-workspace-link]").href = signedIn ? "/file/home" : "/file/signup";
    $("[data-workspace-label]").textContent = signedIn ? "Open my files" : "Create your workspace";
  }

  async function refreshLandingNavigation() {
    setLandingNavigation(false);
    try {
      const user = await (await request("/api/users/me", {}, false)).json();
      if (typeof user?.username === "string") setLandingNavigation(true);
    } catch { /* Keep the public landing page available when signed out or offline. */ }
  }
  if (landingPage) refreshLandingNavigation();

  function validate(form) {
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

  document.addEventListener("input", event => {
    if (event.target instanceof HTMLInputElement) event.target.setCustomValidity("");
    const confirm = event.target.form && $('[name="confirmPassword"]', event.target.form);
    if (confirm) confirm.setCustomValidity("");
  });

  document.querySelectorAll(".signout-form").forEach(form => {
    form.addEventListener("submit", async event => {
      event.preventDefault();
      const button = $('button[type="submit"]', form);
      if (button.disabled) return;
      button.disabled = true;
      status($("#page-status"), "Signing out…");
      try {
        await request(form.action, { method: "POST" }, false);
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
        await request(authForm.action, { method: "POST", body }, false);
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

  function paintUser(user) {
    document.querySelectorAll("[data-username]").forEach(node => { node.textContent = user.username; });
    document.querySelectorAll("[data-avatar]").forEach(node => {
      node.textContent = Array.from(user.username || "").slice(0, 2).join("").toUpperCase();
    });
    if ($("#account-email")) $("#account-email").textContent = user.email;
    document.querySelectorAll(".profile-email").forEach(node => { node.textContent = user.email; });
    if ($("#account-signup")) $("#account-signup").textContent = dateLabel(user.signupAt);
    if ($("#account-active")) $("#account-active").textContent = dateLabel(user.lastActive);
  }

  async function loadUser() {
    const user = await (await request("/api/users/me")).json();
    paintUser(user);
    return user;
  }

  if (pageName) {
    loadUser().then(user => {
      if (pageName !== "account") return;
      $("#profile-name").value = user.username;
      $("#profile-email").value = user.email;
      document.querySelectorAll(".account-form fieldset").forEach(node => { node.disabled = false; });
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
        await request(`/api/users/me/${kind}`, { method: "PATCH", body }, kind !== "password");
        if (kind === "password") {
          form.reset();
          status(output, "Password updated.");
        } else {
          const user = await loadUser();
          $("#profile-name").value = user.username;
          $("#profile-email").value = user.email;
          status(output, kind === "email" ? "Email updated. Use this address to sign in." : "Display name updated.");
        }
      } catch (error) {
        status(output, error.message, true);
      } finally {
        fieldset.disabled = false;
      }
    });
  });

  // Preserve the file order returned by the existing API.
  // Totals are committed only after every page loads; partial results are never totals.
  let allFiles = null, page = 1, loading = false, selectedId = null, editingId = null;
  let pendingDelete = null, detailTrigger = null, deleteTrigger = null;
  const pageSize = 20, rows = $("#file-rows"), detailPanel = $("#detail-panel");
  const uploadPanel = $("#upload-panel"), uploadForm = $("#upload-form");
  const submitUpload = uploadForm && $('button[type="submit"]', uploadForm);
  const fileInput = $("#upload-file"), progress = $("#upload-progress");
  let selectedFile = null, uploading = false, activeTransfer = null, hashForFile = null;
  let refreshQueued = false;

  Object.assign(iconPaths, {
    image: '<rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8" cy="8" r="1.5"/><path d="m21 15-5-5L5 21"/>',
    video: '<rect x="3" y="4" width="18" height="16" rx="2"/><path d="m10 8 6 4-6 4ZM7 4v16M17 4v16"/>',
    audio: '<path d="M9 18V5l11-2v13M9 8l11-2"/><ellipse cx="6" cy="18" rx="3" ry="3"/><ellipse cx="17" cy="16" rx="3" ry="3"/>',
    archive: '<path d="M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9Z"/><path d="M14 3v6h6M10 3v2h2v2h-2v2h2v2h-2v2h2v4h-3v-4"/>',
    code: '<path d="m8 7-5 5 5 5m8-10 5 5-5 5m-3-13-2 20"/>',
    check: '<path d="m5 12 4 4L19 6"/>'
  });

  function metadata(name) {
    const ext = String(name || "").includes(".") ? name.split(".").pop().toLowerCase() : "";
    const groups = {
      image: ["jpg", "jpeg", "png", "gif", "webp", "svg", "heic", "avif"],
      video: ["mp4", "mov", "mkv", "webm", "avi"], audio: ["mp3", "wav", "flac", "m4a", "ogg"],
      archive: ["zip", "rar", "7z", "gz", "tar"], code: ["go", "js", "ts", "py", "json", "html", "css", "rs", "java", "sql"],
      document: ["pdf", "doc", "docx", "txt", "md", "csv", "xlsx", "pptx", "rtf"]
    };
    const type = Object.keys(groups).find(key => groups[key].includes(ext)) || "file";
    const label = type === "file" ? "File" : type[0].toUpperCase() + type.slice(1);
    return { type, label, icon: type === "document" ? "file" : type, description: ext ? `${ext.toUpperCase()} · ${label}` : label };
  }

  function toast(message) {
    const stack = $("#toast-stack");
    if (!stack) return;
    const node = document.createElement("div");
    node.className = "toast";
    node.append(makeIcon("check"), document.createTextNode(message));
    stack.replaceChildren(node);
    setTimeout(() => node.remove(), 4500);
  }

  function paintTotals() {
    if (!allFiles) return;
    const size = allFiles.reduce((sum, file) => sum + Math.max(0, Number(file.fileSize) || 0), 0);
    const count = `${allFiles.length.toLocaleString()} ${allFiles.length === 1 ? "file" : "files"}`;
    document.querySelectorAll("[data-storage-total]").forEach(node => { node.textContent = sizeLabel(size); });
    document.querySelectorAll("[data-storage-caption]").forEach(node => { node.textContent = `${count} · total file size`; });
    document.querySelectorAll("[data-file-count]").forEach(node => { node.hidden = false; node.textContent = allFiles.length.toLocaleString(); });
    if ($("#workspace-summary")) $("#workspace-summary").textContent = `${count} · ${sizeLabel(size)} in your workspace`;
    if ($("#file-count")) { $("#file-count").hidden = false; $("#file-count").textContent = allFiles.length.toLocaleString(); }
  }

  function closeDetails(returnFocus = true) {
    if (!detailPanel) return;
    detailPanel.hidden = true;
    selectedId = null;
    rows.querySelectorAll(".is-selected").forEach(node => node.classList.remove("is-selected"));
    rows.querySelectorAll(".stored-name").forEach(node => node.setAttribute("aria-expanded", "false"));
    if (returnFocus && detailTrigger?.isConnected) detailTrigger.focus();
  }

  function showDetails(file, trigger, focus = true) {
    selectedId = file.id;
    if (trigger) detailTrigger = trigger;
    const meta = metadata(file.fileName);
    $("#detail-icon").replaceChildren(makeIcon(meta.icon));
    $("#detail-name").textContent = file.fileName;
    $("#detail-type").textContent = meta.description;
    $("#detail-size").textContent = sizeLabel(file.fileSize);
    $("#detail-uploaded").textContent = dateLabel(file.uploadAt);
    $("#detail-modified").textContent = dateLabel(file.lastUpdated);
    $("#detail-hash").textContent = file.fileHash || "Unavailable";
    $("#detail-download").href = `/api/files/${encodeURIComponent(file.id)}/content`;
    $("#detail-download").download = file.fileName;
    detailPanel.hidden = false;
    rows.querySelectorAll("tr").forEach(row => {
      const selected = row.dataset.id === String(file.id);
      row.classList.toggle("is-selected", selected);
      $(".stored-name", row)?.setAttribute("aria-expanded", String(selected));
    });
    if (focus) $("#close-details").focus();
  }

  function beginRename(file) {
    const row = [...rows.children].find(node => node.dataset.id === String(file.id));
    if (!row) return;
    if (editingId !== null) renderFiles();
    editingId = file.id;
    const currentRow = [...rows.children].find(node => node.dataset.id === String(file.id));
    const copy = $(".file-copy", currentRow), trigger = $(".stored-name", copy);
    const form = document.createElement("form");
    form.className = "rename-inline";
    const input = document.createElement("input");
    input.name = "name"; input.value = file.fileName; input.required = true;
    input.setAttribute("aria-label", `New name for ${file.fileName}`);
    const save = document.createElement("button");
    save.type = "submit"; save.className = "button button-primary"; save.textContent = "Save";
    const cancel = document.createElement("button");
    cancel.type = "button"; cancel.className = "button button-ghost"; cancel.textContent = "Cancel";
    const feedback = document.createElement("p");
    feedback.className = "status"; feedback.setAttribute("role", "status");
    let saving = false;
    const dismiss = () => {
      if (saving) return;
      editingId = null; renderFiles();
      [...rows.children].find(node => node.dataset.id === String(file.id))?.querySelector(".stored-name")?.focus();
    };
    cancel.addEventListener("click", dismiss);
    form.addEventListener("keydown", event => { if (event.key === "Escape") { event.stopPropagation(); event.preventDefault(); dismiss(); } });
    form.addEventListener("submit", async event => {
      event.preventDefault();
      if (saving) return;
      const name = input.value.trim();
      if (!name || bytes(name) > 255) { status(feedback, "Use a file name of 1–255 bytes.", true); return; }
      saving = true; input.disabled = save.disabled = cancel.disabled = true; save.textContent = "Saving…";
      try {
        await request(`/api/files/${encodeURIComponent(file.id)}`, { method: "PATCH", body: new URLSearchParams({ name }) });
        file.fileName = name;
        editingId = null; renderFiles();
        if (selectedId === file.id) showDetails(file, null, false);
        [...rows.children].find(node => node.dataset.id === String(file.id))?.querySelector(".stored-name")?.focus();
        toast(`Renamed to “${name}”.`);
        await loadFiles();
      } catch (error) { status(feedback, error.message, true); }
      finally { saving = false; input.disabled = save.disabled = cancel.disabled = false; save.textContent = "Save"; }
    });
    trigger.hidden = true;
    form.append(input, save, cancel);
    copy.prepend(form);
    copy.append(feedback);
    input.focus();
    const dot = file.fileName.lastIndexOf(".");
    input.setSelectionRange(0, dot > 0 ? dot : file.fileName.length);
  }

  const dialog = $("#file-dialog");
  function confirmDelete(file, trigger) {
    pendingDelete = file; deleteTrigger = trigger;
    $("#dialog-title").textContent = `Delete “${file.fileName}”?`;
    status($("#dialog-status"));
    dialog.showModal();
    $("#dialog-cancel").focus();
  }
  if (dialog) {
    $("#dialog-cancel").addEventListener("click", () => dialog.close());
    dialog.addEventListener("close", () => { if (deleteTrigger?.isConnected) deleteTrigger.focus(); });
    dialog.addEventListener("cancel", event => { if ($("#dialog-submit").disabled) event.preventDefault(); });
    $("#file-action-form").addEventListener("submit", async event => {
      event.preventDefault();
      if (!pendingDelete || $("#dialog-submit").disabled) return;
      const file = pendingDelete;
      $("#dialog-submit").disabled = $("#dialog-cancel").disabled = true;
      $("#dialog-submit").textContent = "Deleting…";
      try {
        await request(`/api/files/${encodeURIComponent(file.id)}`, { method: "DELETE" });
        allFiles = allFiles.filter(item => item.id !== file.id);
        if (selectedId === file.id) closeDetails(false);
        pendingDelete = null; dialog.close(); paintTotals(); renderFiles();
        $("#refresh-files").focus(); toast(`Deleted “${file.fileName}”.`);
      } catch (error) { status($("#dialog-status"), error.message, true); }
      finally {
        $("#dialog-submit").disabled = $("#dialog-cancel").disabled = false;
        $("#dialog-submit").textContent = "Delete file";
      }
    });
  }

  function buildMenu(file) {
    const menu = document.createElement("details"); menu.className = "file-menu";
    const trigger = document.createElement("summary");
    for (const [key, value] of Object.entries({ role: "button", "aria-label": `Actions for ${file.fileName}`, "aria-haspopup": "menu", "aria-expanded": "false", title: `Actions for ${file.fileName}` })) trigger.setAttribute(key, value);
    trigger.append(makeIcon("more"));
    const panel = document.createElement("div"); panel.className = "file-menu-panel";
    panel.setAttribute("role", "menu"); panel.setAttribute("aria-label", `Actions for ${file.fileName}`);
    const download = document.createElement("a");
    download.href = `/api/files/${encodeURIComponent(file.id)}/content`; download.download = file.fileName;
    download.className = "row-action"; download.setAttribute("role", "menuitem");
    download.setAttribute("aria-label", `Download ${file.fileName}`);
    download.append(makeIcon("download"), document.createTextNode("Download"));
    download.addEventListener("click", () => { menu.open = false; trigger.focus(); });
    panel.append(download);
    for (const kind of ["Rename", "Delete"]) {
      const button = document.createElement("button"); button.type = "button";
      button.className = "row-action" + (kind === "Delete" ? " danger" : "");
      button.setAttribute("role", "menuitem"); button.setAttribute("aria-label", `${kind} ${file.fileName}`);
      button.append(makeIcon(kind.toLowerCase()), document.createTextNode(kind));
      button.addEventListener("click", () => { menu.open = false; kind === "Rename" ? beginRename(file) : confirmDelete(file, trigger); });
      panel.append(button);
    }
    menu.append(trigger, panel);
    menu.addEventListener("toggle", () => { trigger.setAttribute("aria-expanded", String(menu.open)); if (menu.open) closeFileMenus(menu); });
    menu.addEventListener("keydown", event => {
      const items = [...panel.querySelectorAll('[role="menuitem"]')];
      if (event.key === "Escape") { event.preventDefault(); event.stopPropagation(); menu.open = false; trigger.setAttribute("aria-expanded", "false"); trigger.focus(); }
      if (["ArrowDown", "ArrowUp", "Home", "End"].includes(event.key)) {
        event.preventDefault(); menu.open = true; trigger.setAttribute("aria-expanded", "true"); closeFileMenus(menu);
        let index = items.indexOf(document.activeElement);
        if (event.key === "Home") index = 0;
        else if (event.key === "End") index = items.length - 1;
        else index = (index + (event.key === "ArrowDown" ? 1 : -1) + items.length) % items.length;
        items[index].focus();
      }
    });
    menu.addEventListener("focusout", event => { if (!menu.contains(event.relatedTarget)) menu.open = false; });
    return menu;
  }

  function renderFiles(arrivingIds = new Set()) {
    if (!rows || !allFiles) return;
    const files = allFiles;
    const pages = Math.max(1, Math.ceil(files.length / pageSize));
    page = Math.max(1, Math.min(page, pages));
    rows.replaceChildren(); editingId = null;
    for (const file of files.slice((page - 1) * pageSize, page * pageSize)) {
      const row = document.createElement("tr"); row.dataset.id = String(file.id);
      row.classList.toggle("is-selected", file.id === selectedId);
      if (arrivingIds.has(file.id)) row.classList.add("file-row-enter");
      const cell = document.createElement("td"), wrap = document.createElement("div"), type = document.createElement("span"), copy = document.createElement("div");
      const meta = metadata(file.fileName);
      wrap.className = "file-cell"; copy.className = "file-copy"; type.className = `file-type type-${meta.type}`;
      type.append(makeIcon(meta.icon));
      const name = document.createElement("button"); name.type = "button"; name.className = "stored-name";
      name.textContent = file.fileName; name.title = file.fileName;
      name.setAttribute("aria-controls", "detail-panel"); name.setAttribute("aria-expanded", String(file.id === selectedId));
      name.addEventListener("click", () => showDetails(file, name));
      const sub = document.createElement("span"); sub.className = "file-subtitle"; sub.textContent = meta.description;
      copy.append(name, sub); wrap.append(type, copy); cell.append(wrap); row.append(cell);
      const time = new Date(file.uploadAt);
      const uploaded = Number.isNaN(time.getTime()) ? "—" : time.toLocaleDateString(undefined, { month: "short", day: "numeric", year: "numeric" });
      for (const [label, value] of [["Size", sizeLabel(file.fileSize)], ["Uploaded", uploaded]]) {
        const data = document.createElement("td"); data.className = "data"; data.dataset.label = label; data.textContent = value;
        if (label === "Uploaded") data.title = dateLabel(file.uploadAt);
        row.append(data);
      }
      const stored = document.createElement("td"); stored.dataset.label = "Status";
      const tag = document.createElement("span"); tag.className = "stored-status"; tag.textContent = "Stored"; stored.append(tag); row.append(stored);
      const control = document.createElement("td"), group = document.createElement("div"); group.className = "table-actions";
      group.append(buildMenu(file)); control.append(group); row.append(control); rows.append(row);
    }
    $("#files-empty").hidden = files.length > 0;
    $(".table-scroll").hidden = files.length === 0;
    const start = (page - 1) * pageSize + 1, end = Math.min(page * pageSize, files.length);
    $("#files-summary").textContent = files.length ? `${start}–${end} of ${files.length.toLocaleString()} ${files.length === 1 ? "file" : "files"}` : "No files stored yet";
    $("#pager").hidden = pages === 1;
    $("#page-label").textContent = `${page} / ${pages}`;
    $("#previous-page").disabled = page === 1; $("#next-page").disabled = page === pages;
  }

  async function loadFiles(resetPage = false, animateNew = false) {
    if (!pageName) return;
    if (loading) { refreshQueued = refreshQueued || animateNew; return; }
    loading = true;
    const knownIds = new Set((allFiles || []).map(file => file.id));
    if (rows) {
      $("#refresh-files").disabled = true;
      $(".file-browser").setAttribute("aria-busy", "true");
      status($("#files-status"), allFiles ? "Refreshing files…" : "Loading files…");
      $("#file-skeleton").hidden = Boolean(allFiles);
    }
    try {
      const files = [], seen = new Set();
      for (let currentPage = 1; ; currentPage++) {
        const batch = await (await request(`/api/files?page=${currentPage}&pageSize=100`)).json();
        if (!Array.isArray(batch) || batch.some(file => !file || !Number.isSafeInteger(file.id) || typeof file.fileName !== "string" || !Number.isFinite(file.fileSize))) throw new Error("Unable to read the file list.");
        let newCount = 0;
        for (const file of batch) { if (!seen.has(file.id)) { seen.add(file.id); files.push(file); newCount++; } }
        if (batch.length < 100) break;
        if (!newCount) throw new Error("The file list changed while loading. Please refresh.");
      }
      allFiles = files; if (resetPage) page = 1;
      paintTotals();
      if (rows) {
        const selected = allFiles.find(file => file.id === selectedId);
        if (selected) showDetails(selected, null, false); else closeDetails(false);
        renderFiles(animateNew ? new Set(files.filter(file => !knownIds.has(file.id)).map(file => file.id)) : new Set());
        status($("#files-status"));
      }
    } catch (error) {
      if (rows) status($("#files-status"), error.message + " Select Refresh to retry.", true);
      else status($("#page-status"), "Storage information could not be loaded. Refresh this page to retry.", true);
      if (!allFiles) {
        document.querySelectorAll("[data-storage-total]").forEach(node => { node.textContent = "Unavailable"; });
        document.querySelectorAll("[data-storage-caption]").forEach(node => { node.textContent = "Refresh to try again"; });
        if ($("#workspace-summary")) $("#workspace-summary").textContent = "Your files couldn’t be loaded";
      }
    } finally {
      loading = false;
      if (rows) {
        $("#refresh-files").disabled = false;
        $(".file-browser").setAttribute("aria-busy", "false");
        $("#file-skeleton").hidden = true;
      }
      if (refreshQueued) { refreshQueued = false; loadFiles(true, true); }
    }
  }

  if (rows) {
    $("#previous-page").addEventListener("click", () => { page--; renderFiles(); });
    $("#next-page").addEventListener("click", () => { page++; renderFiles(); });
    $("#refresh-files").addEventListener("click", () => loadFiles());
    $("#close-details").addEventListener("click", () => closeDetails());
    $("#detail-rename").addEventListener("click", () => {
      const file = allFiles.find(item => item.id === selectedId);
      if (file) { closeDetails(false); beginRename(file); }
    });
    $("#detail-delete").addEventListener("click", event => {
      const file = allFiles.find(item => item.id === selectedId);
      if (file) confirmDelete(file, event.currentTarget);
    });
    document.addEventListener("keydown", event => {
      if (event.key === "Escape" && !detailPanel.hidden && !dialog.open && editingId === null) closeDetails();
    });
  }
  if (pageName) loadFiles();
  if (!uploadForm) return;

  function openUpload(focus = true) {
    uploadPanel.hidden = false;
    if (focus) { uploadPanel.scrollIntoView({ behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth", block: "nearest" }); fileInput.focus({ preventScroll: true }); }
  }
  document.querySelectorAll("[data-open-upload]").forEach(button => button.addEventListener("click", () => openUpload()));
  $("#close-upload").addEventListener("click", () => {
    uploadPanel.hidden = true;
    if (uploading) toast("Your transfer is still running. Open Upload file to see its progress.");
    $('.page-heading [data-open-upload]')?.focus();
  });
  if (pageName === "upload") openUpload(false);
  submitUpload.disabled = true;

  function setStage(stage, label, state = "active") {
    const stages = ["preparing", "uploading", "verifying", "complete"], index = stages.indexOf(stage);
    $("#transfer-card").dataset.state = state;
    $("#transfer-state").textContent = label;
    document.querySelectorAll("[data-stage]").forEach(node => {
      const current = stages.indexOf(node.dataset.stage);
      node.classList.toggle("is-active", current === index);
      node.classList.toggle("is-done", current < index);
      if (current === index) node.setAttribute("aria-current", "step"); else node.removeAttribute("aria-current");
    });
  }
  function transferProgress(uploaded, total) {
    progress.value = total ? Math.min(100, uploaded / total * 100) : 100;
    $("#transfer-readout").hidden = false;
    $("#upload-bytes").textContent = `${sizeLabel(uploaded)} / ${sizeLabel(total)}`;
    $("#upload-percent").textContent = `${Math.floor(progress.value)}%`;
  }
  function selectFile(file) {
    if (uploading) return;
    selectedFile = file || null; hashForFile = null;
    submitUpload.disabled = !file; submitUpload.textContent = "Upload file";
    $("#drop-zone").classList.toggle("has-file", Boolean(file));
    $("#transfer-card").hidden = !file;
    $("#selected-file").textContent = file?.name || "";
    $("#selected-size").textContent = file ? sizeLabel(file.size) : "";
    if (file) {
      const meta = metadata(file.name);
      $("#transfer-icon").className = `file-type type-${meta.type}`;
      $("#transfer-icon").replaceChildren(makeIcon(meta.icon));
      setStage("preparing", "Ready", "ready");
      $('[data-stage="verifying"]').textContent = file.size > ordinaryLimit ? "Verifying" : "Saving";
    }
    $("#resumable-note").hidden = !file || file.size <= ordinaryLimit;
    $('#resumable-note span').textContent = "Resumable upload enabled";
    progress.hidden = true; progress.value = 0;
    $("#transfer-readout").hidden = $("#upload-complete-link").hidden = true;
    status($("#upload-status")); $("#upload-status").classList.remove("is-success");
  }

  function ordinaryUpload(file) {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      activeTransfer = { mode: "ordinary", xhr, cancelRequested: false };
      $("#cancel-upload").hidden = false;
      xhr.open("POST", "/api/files");
      xhr.upload.onprogress = event => {
        if (!event.lengthComputable) return;
        transferProgress(Math.floor(file.size * event.loaded / event.total), file.size);
        status($("#upload-status"), event.loaded === event.total ? "Saving your file…" : "Your file is on its way.");
        if (event.loaded === event.total) {
          setStage("verifying", "Saving…");
          // Once the request body arrives, the server may commit the file.
          $("#cancel-upload").hidden = true;
        }
      };
      xhr.onerror = () => reject(new Error("Connection lost. Select Retry upload to start again."));
      xhr.onabort = () => reject(Object.assign(new Error("Upload cancelled."), { name: "UploadCancelled" }));
      xhr.onload = () => {
        if (xhr.status === 401) location.assign("/file/signin");
        if (xhr.status >= 200 && xhr.status < 300) resolve({});
        else reject(new Error(xhr.responseText.trim() || "Upload failed. Please try again."));
      };
      const body = new FormData(); body.append("file", file); xhr.send(body);
    });
  }

  async function multipartUpload(file, hash) {
    const upload = await (await request("/api/uploads", { method: "POST", body: new URLSearchParams({ filehash: hash, filename: file.name, filesize: String(file.size) }) })).json();
    const chunkCount = Number(upload.chunkCount), chunkSize = Number(upload.chunkSize);
    if (!Number.isSafeInteger(chunkCount) || !Number.isSafeInteger(chunkSize) || chunkCount < 1 || chunkCount > 10000 || chunkSize <= 0 || chunkCount !== Math.ceil(file.size / chunkSize)) throw new Error("Unable to start this upload. Please try again.");
    const transfer = { mode: "multipart", hash, controller: new AbortController(), cancelRequested: false, pauseRequested: false, verifying: false };
    activeTransfer = transfer;
    $("#pause-upload").hidden = $("#cancel-upload").hidden = false;
    try {
      const info = await (await request(`/api/uploads/${hash}`, { signal: transfer.controller.signal })).json();
      if (!Array.isArray(info.uploadedChunks) || info.chunkCount !== chunkCount || info.uploadedChunks.some(index => !Number.isInteger(index) || index < 0 || index >= chunkCount)) throw new Error("Unable to read saved progress. Please retry.");
      const completed = new Set(info.uploadedChunks);
      let saved = [...completed].reduce((total, index) => total + Math.min(chunkSize, file.size - index * chunkSize), 0);
      transferProgress(saved, file.size);
      if (completed.size) {
        setStage("uploading", "Resumed");
        status($("#upload-status"), `Upload resumed. ${sizeLabel(saved)} was already saved.`);
        toast("Picking up from your saved progress.");
      }
      for (let index = 0; index < chunkCount; index++) {
        if (transfer.pauseRequested || transfer.cancelRequested) throw new DOMException("Transfer stopped", "AbortError");
        if (!completed.has(index)) {
          await request(`/api/uploads/${hash}/parts/${index}`, { method: "PUT", headers: { "Content-Type": "application/octet-stream" }, body: file.slice(index * chunkSize, Math.min((index + 1) * chunkSize, file.size)), signal: transfer.controller.signal });
          completed.add(index); saved += Math.min(chunkSize, file.size - index * chunkSize);
          transferProgress(saved, file.size);
          setStage("uploading", "Uploading");
        }
      }
      transfer.verifying = true;
      $("#pause-upload").hidden = $("#cancel-upload").hidden = true;
      setStage("verifying", "Verifying"); status($("#upload-status"), "Checking your file before it joins your workspace…");
      await request(`/api/uploads/${hash}/completion`, { method: "POST" });
      return { verified: true };
    } catch (error) {
      if (transfer.pauseRequested) throw Object.assign(new Error("Upload paused. Your saved progress is kept. Select Resume upload to continue."), { name: "UploadPaused" });
      if (transfer.cancelRequested) {
        try { await request(`/api/uploads/${hash}`, { method: "DELETE" }); }
        catch { throw new Error("The transfer stopped, but saved progress could not be removed. Please retry cancellation."); }
        throw Object.assign(new Error("Upload cancelled."), { name: "UploadCancelled" });
      }
      throw error;
    }
  }

  $("#pause-upload").addEventListener("click", () => {
    if (activeTransfer?.mode !== "multipart" || activeTransfer.verifying || activeTransfer.cancelRequested) return;
    activeTransfer.pauseRequested = true; activeTransfer.controller.abort();
    $("#pause-upload").disabled = true;
  });
  $("#cancel-upload").addEventListener("click", () => {
    if (!activeTransfer || activeTransfer.verifying || activeTransfer.pauseRequested) return;
    activeTransfer.cancelRequested = true;
    $("#cancel-upload").disabled = true; $("#pause-upload").disabled = true;
    if (activeTransfer.mode === "ordinary") activeTransfer.xhr.abort(); else activeTransfer.controller.abort();
    status($("#upload-status"), "Cancelling upload…");
  });
  fileInput.addEventListener("change", () => selectFile(fileInput.files[0]));

  // File drops work throughout the files workspace and in the upload panel.
  let dragDepth = 0;
  const dropOverlay = $("#workspace-drop-target"), zone = $("#drop-zone");
  const fileDrag = event => [...(event.dataTransfer?.types || [])].includes("Files");
  document.addEventListener("dragenter", event => {
    if (!fileDrag(event)) return;
    event.preventDefault(); dragDepth++;
    if (dropOverlay && !uploading && !dialog?.open) dropOverlay.hidden = false;
    zone.classList.toggle("drag-active", !uploading);
  });
  document.addEventListener("dragover", event => {
    if (!fileDrag(event)) return;
    event.preventDefault(); event.dataTransfer.dropEffect = uploading ? "none" : "copy";
  });
  document.addEventListener("dragleave", event => {
    if (!fileDrag(event)) return;
    dragDepth = Math.max(0, dragDepth - 1);
    if (!dragDepth) { if (dropOverlay) dropOverlay.hidden = true; zone.classList.remove("drag-active"); }
  });
  document.addEventListener("drop", event => {
    if (!fileDrag(event)) return;
    event.preventDefault(); dragDepth = 0;
    if (dropOverlay) dropOverlay.hidden = true;
    zone.classList.remove("drag-active");
    if (uploading || dialog?.open) return;
    openUpload();
    if (event.dataTransfer.files.length !== 1) { status($("#upload-status"), "Choose one file at a time.", true); return; }
    fileInput.files = event.dataTransfer.files; selectFile(fileInput.files[0]);
  });
  window.addEventListener("blur", () => { dragDepth = 0; if (dropOverlay) dropOverlay.hidden = true; zone.classList.remove("drag-active"); });

  uploadForm.addEventListener("submit", async event => {
    event.preventDefault();
    if (!selectedFile || uploading) return;
    const file = selectedFile;
    if (!file.name.trim() || bytes(file.name.trim()) > 255) { status($("#upload-status"), "Use a file name of 1–255 bytes.", true); return; }
    uploading = true;
    fileInput.disabled = submitUpload.disabled = true;
    submitUpload.textContent = "Preparing…"; uploadForm.setAttribute("aria-busy", "true");
    progress.hidden = false; progress.removeAttribute("value");
    $("#upload-complete-link").hidden = true; $("#upload-status").classList.remove("is-success");
    setStage("preparing", "Preparing"); status($("#upload-status"), "Preparing your file…");
    try {
      let reused = false;
      if (file.size > 0 && globalThis.crypto?.subtle) {
        if (!hashForFile) {
          const digest = await crypto.subtle.digest("SHA-256", await file.arrayBuffer());
          hashForFile = [...new Uint8Array(digest)].map(byte => byte.toString(16).padStart(2, "0")).join("");
        }
        status($("#upload-status"), "Checking file…"); setStage("preparing", "Checking file…");
        const fast = await (await request("/api/files/fast", { method: "POST", body: new URLSearchParams({ filehash: hashForFile, filename: file.name, filesize: String(file.size) }) })).json();
        if (typeof fast.reused !== "boolean") throw new Error("Unable to check your file. Please retry.");
        reused = fast.reused;
      } else if (file.size > ordinaryLimit) throw new Error("Large uploads need a secure connection. Open GoCloud using HTTPS.");
      let result = {};
      if (!reused) {
        setStage("uploading", "Uploading"); status($("#upload-status"), "Your file is on its way.");
        progress.value = 0; transferProgress(0, file.size); submitUpload.textContent = "Uploading…";
        result = file.size > ordinaryLimit ? await multipartUpload(file, hashForFile) : await ordinaryUpload(file);
      }
      if (reused) {
        progress.value = 100;
        $("#transfer-readout").hidden = false;
        $("#upload-bytes").textContent = "No transfer needed";
        $("#upload-percent").textContent = "Ready";
      } else transferProgress(file.size, file.size);
      setStage("complete", reused ? "Ready instantly" : "Complete", reused ? "instant" : "complete");
      status($("#upload-status"), reused ? "Ready instantly. Your file was already stored, so there was nothing to transfer." : `Upload complete. “${file.name}” is in your workspace.${result.verified ? " Your file has been verified." : ""}`);
      $("#upload-status").classList.add("is-success"); $("#upload-complete-link").hidden = false;
      selectedFile = null; hashForFile = null; uploadForm.reset(); submitUpload.textContent = "Upload file";
      toast(reused ? `“${file.name}” is ready instantly.` : `“${file.name}” has arrived.`);
      // A failed list refresh must never turn a completed transfer into an error.
      await loadFiles(true, true);
    } catch (error) {
      if (error.name === "UploadPaused") {
        setStage("uploading", "Paused", "paused"); status($("#upload-status"), error.message);
        submitUpload.textContent = "Resume upload";
      } else if (error.name === "UploadCancelled") {
        setStage("preparing", "Cancelled", "cancelled"); status($("#upload-status"), error.message);
        progress.hidden = true; submitUpload.textContent = "Upload file";
      } else {
        const integrity = /size or hash does not match/i.test(error.message);
        setStage(integrity ? "verifying" : "uploading", "Interrupted", "error");
        const hint = file.size > ordinaryLimit ? " Saved progress can resume when you retry with this file." : "";
        status($("#upload-status"), (integrity ? "Your file could not be verified. Please retry the upload." : error.message) + hint, true);
        submitUpload.textContent = "Retry upload";
        if (!progress.hasAttribute("value")) progress.hidden = true;
      }
    } finally {
      uploading = false; activeTransfer = null;
      uploadForm.setAttribute("aria-busy", "false"); fileInput.disabled = false; submitUpload.disabled = !selectedFile;
      $("#pause-upload").hidden = $("#cancel-upload").hidden = true;
      $("#pause-upload").disabled = $("#cancel-upload").disabled = false;
    }
  });
  window.addEventListener("beforeunload", event => {
    if (uploading) { event.preventDefault(); event.returnValue = ""; }
  });
})();
