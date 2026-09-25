(() => {
  "use strict";
  const $ = (selector, root = document) => root.querySelector(selector);
  const bytes = value => new TextEncoder().encode(value).length;
  const pageName = document.body.dataset.page;
  const ordinaryLimit = 100 * 1024 * 1024;

  function status(node, message = "", error = false) {
    if (!node) return;
    node.textContent = message;
    node.classList.toggle("is-error", error);
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

  function fileKind(name) {
    const ext = String(name || "").split(".").pop();
    if (!ext || ext === name) return "FILE";
    return ext.slice(0, 4).toUpperCase();
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

  let page = 1, fileCount = 0, hasNext = false, loading = false, action = null;
  const pageSize = 20;
  const rows = $("#file-rows");

  function renderFiles(files) {
    rows.replaceChildren();
    for (const file of files) {
      const row = document.createElement("tr");
      const nameCell = document.createElement("td");
      const wrap = document.createElement("div");
      wrap.className = "file-cell";
      const kind = document.createElement("span");
      kind.className = "file-type";
      kind.textContent = fileKind(file.fileName);
      const copy = document.createElement("span");
      const name = document.createElement("strong");
      name.className = "stored-name";
      name.textContent = file.fileName;
      const hash = document.createElement("span");
      hash.className = "file-hash";
      hash.textContent = file.fileHash ? `SHA-256 ${file.fileHash.slice(0, 12)}…` : "";
      copy.append(name);
      if (file.fileHash) copy.append(hash);
      wrap.append(kind, copy);
      nameCell.append(wrap);
      row.append(nameCell);
      for (const text of [sizeLabel(file.fileSize), dateLabel(file.uploadAt), dateLabel(file.lastUpdated)]) {
        const cell = document.createElement("td");
        cell.textContent = text;
        row.append(cell);
      }
      const controls = document.createElement("td"), group = document.createElement("div");
      group.className = "table-actions";
      const download = document.createElement("a");
      download.className = "row-action";
      download.href = `/api/files/${encodeURIComponent(file.id)}/content`;
      download.download = file.fileName;
      download.textContent = "Download";
      download.setAttribute("aria-label", `Download ${file.fileName}`);
      group.append(download);
      for (const kindName of ["Rename", "Delete"]) {
        const button = document.createElement("button");
        button.type = "button";
        button.className = "row-action" + (kindName === "Delete" ? " danger" : "");
        button.textContent = kindName;
        button.setAttribute("aria-label", `${kindName} ${file.fileName}`);
        button.addEventListener("click", () => openAction(kindName, file));
        group.append(button);
      }
      controls.append(group);
      row.append(controls);
      rows.append(row);
    }
  }

  async function loadFiles(target = page) {
    if (!rows || loading) return;
    loading = true;
    const previous = $("#previous-page"), next = $("#next-page"), refresh = $("#refresh-files");
    if (previous) previous.disabled = true;
    if (next) next.disabled = true;
    if (refresh) refresh.disabled = true;
    status($("#files-status"), "Loading files…");
    try {
      let files = await (await request(`/api/files?page=${target}&pageSize=${pageSize}`)).json();
      if (!Array.isArray(files)) throw new Error("Unable to read the file list.");
      if (target > 1 && files.length === 0) {
        target--;
        files = await (await request(`/api/files?page=${target}&pageSize=${pageSize}`)).json();
        hasNext = false;
      } else hasNext = files.length === pageSize;
      page = target;
      fileCount = files.length;
      renderFiles(files);
      status($("#files-status"), files.length ? "" : "No files yet. Upload a file to get started.");
      if ($("#pager")) $("#pager").hidden = page === 1 && !hasNext;
      if ($("#page-label")) $("#page-label").textContent = `Page ${page}`;
    } catch (error) {
      status($("#files-status"), error.message + " Select Refresh to retry.", true);
    } finally {
      loading = false;
      if (previous) previous.disabled = page === 1;
      if (next) next.disabled = !hasNext;
      if (refresh) refresh.disabled = false;
    }
  }

  if (pageName === "files") {
    $("#previous-page").addEventListener("click", () => loadFiles(page - 1));
    $("#next-page").addEventListener("click", () => loadFiles(page + 1));
    $("#refresh-files").addEventListener("click", () => loadFiles());
    loadFiles();
  }

  const dialog = $("#file-dialog"), actionForm = $("#file-action-form");
  function openAction(kind, file) {
    action = { kind, file };
    $("#dialog-title").textContent = `${kind} file`;
    $("#dialog-description").textContent = kind === "Delete"
      ? `Delete “${file.fileName}” from your files? This cannot be undone.`
      : `Choose a new name for “${file.fileName}”.`;
    $("#rename-field").hidden = kind !== "Rename";
    $("#rename-input").required = kind === "Rename";
    $("#rename-input").value = file.fileName;
    $("#dialog-submit").textContent = kind === "Delete" ? "Delete file" : "Save name";
    $("#dialog-submit").classList.toggle("button-danger-ghost", kind === "Delete");
    $("#dialog-submit").classList.toggle("button-primary", kind !== "Delete");
    status($("#dialog-status"));
    dialog.showModal();
  }
  if (dialog) {
    $("#dialog-cancel").addEventListener("click", () => dialog.close());
    dialog.addEventListener("cancel", event => { if ($("#dialog-submit").disabled) event.preventDefault(); });
    actionForm.addEventListener("submit", async event => {
      event.preventDefault();
      const { kind, file } = action;
      const name = $("#rename-input").value.trim();
      if (kind === "Rename" && (!name || bytes(name) > 255)) {
        status($("#dialog-status"), "Use a file name of 1–255 bytes.", true);
        return;
      }
      $("#dialog-submit").disabled = $("#dialog-cancel").disabled = true;
      status($("#dialog-status"), kind === "Delete" ? "Deleting…" : "Saving…");
      try {
        await request(
          `/api/files/${encodeURIComponent(file.id)}`,
          kind === "Delete" ? { method: "DELETE" } : { method: "PATCH", body: new URLSearchParams({ name }) }
        );
        dialog.close();
        await loadFiles(kind === "Delete" && fileCount === 1 && page > 1 ? page - 1 : page);
      } catch (error) {
        status($("#dialog-status"), error.message, true);
      } finally {
        $("#dialog-submit").disabled = $("#dialog-cancel").disabled = false;
      }
    });
  }

  const uploadForm = $("#upload-form"), fileInput = $("#upload-file"), progress = $("#upload-progress");
  if (!uploadForm) return;

  let selectedFile = null, uploading = false;

  function selectFile(file) {
    selectedFile = file || null;
    status($("#selected-file"), file ? `${file.name} · ${sizeLabel(file.size)}` : "");
    status($("#upload-status"));
    progress.hidden = true;
    progress.value = 0;
  }

  async function sha256Hex(file) {
    const digest = await crypto.subtle.digest("SHA-256", await file.arrayBuffer());
    return Array.from(new Uint8Array(digest), byte => byte.toString(16).padStart(2, "0")).join("");
  }

  function ordinaryUpload(file) {
    return new Promise((resolve, reject) => {
      const xhr = new XMLHttpRequest();
      xhr.open("POST", "/api/files");
      xhr.upload.onprogress = event => {
        if (event.lengthComputable) {
          progress.value = event.loaded / event.total * 100;
          status($("#upload-status"), event.loaded === event.total ? "Saving your file…" : `Uploading… ${Math.floor(progress.value)}%`);
        }
      };
      xhr.onerror = () => reject(new Error("Connection lost. Please retry your upload."));
      xhr.onload = () => {
        if (xhr.status === 401) location.assign("/file/signin");
        if (xhr.status >= 200 && xhr.status < 300) resolve();
        else reject(new Error(xhr.responseText.trim() || "Upload failed. Please try again."));
      };
      const body = new FormData();
      body.append("file", file);
      xhr.send(body);
    });
  }

  async function multipartUpload(file) {
    if (!globalThis.crypto?.subtle) {
      throw new Error("Large uploads need HTTPS or localhost. Open the app using a secure connection.");
    }
    status($("#upload-status"), "Calculating SHA-256…");
    const hash = await sha256Hex(file);
    const upload = await (await request("/api/uploads", {
      method: "POST",
      body: new URLSearchParams({ filehash: hash, filename: file.name, filesize: String(file.size) })
    })).json();
    const chunkCount = Number(upload.chunkCount);
    const chunkSize = Number(upload.chunkSize);
    if (!Number.isSafeInteger(chunkCount) || !Number.isSafeInteger(chunkSize) ||
        chunkSize <= 0 || chunkCount !== Math.ceil(file.size / chunkSize)) {
      throw new Error("Unable to start the large-file upload.");
    }
    const info = await (await request(`/api/uploads/${hash}`)).json();
    if (!Array.isArray(info.uploadedChunks) || info.chunkCount !== chunkCount ||
        info.uploadedChunks.some(index => !Number.isInteger(index) || index < 0 || index >= chunkCount)) {
      throw new Error("Unable to read upload progress. Please retry.");
    }
    const completed = new Set(info.uploadedChunks);
    if (completed.size) status($("#upload-status"), `Resuming… ${completed.size} of ${chunkCount} parts already stored.`);
    for (let index = 0; index < chunkCount; index++) {
      progress.value = completed.size / chunkCount * 100;
      if (!completed.has(index)) {
        status($("#upload-status"), `Uploading part ${index + 1} of ${chunkCount}…`);
        await request(`/api/uploads/${hash}/parts/${index}`, {
          method: "PUT",
          headers: { "Content-Type": "application/octet-stream" },
          body: file.slice(index * chunkSize, Math.min((index + 1) * chunkSize, file.size))
        });
        completed.add(index);
      }
      progress.value = completed.size / chunkCount * 100;
    }
    status($("#upload-status"), "Verifying and saving your file…");
    await request(`/api/uploads/${hash}/completion`, { method: "POST" });
  }

  fileInput.addEventListener("change", () => selectFile(fileInput.files[0]));
  const zone = $("#drop-zone");
  for (const eventName of ["dragover", "dragleave", "drop"]) {
    zone.addEventListener(eventName, event => {
      event.preventDefault();
      zone.classList.toggle("drag-active", eventName === "dragover" && !uploading);
      if (eventName === "drop" && !uploading) {
        if (event.dataTransfer.files.length !== 1) {
          status($("#upload-status"), "Choose one file at a time.", true);
          return;
        }
        fileInput.files = event.dataTransfer.files;
        selectFile(fileInput.files[0]);
      }
    });
  }

  uploadForm.addEventListener("submit", async event => {
    event.preventDefault();
    if (!selectedFile || uploading) return;
    const file = selectedFile;
    if (!file.name.trim() || bytes(file.name.trim()) > 255) {
      status($("#upload-status"), "Use a file name of 1–255 bytes.", true);
      return;
    }
    uploading = true;
    fileInput.disabled = $('button[type="submit"]', uploadForm).disabled = true;
    progress.hidden = false;
    progress.value = 0;
    status($("#upload-status"), "Preparing upload…");
    try {
      if (file.size <= ordinaryLimit) await ordinaryUpload(file);
      else await multipartUpload(file);
      progress.value = 100;
      status($("#upload-status"), `Uploaded “${file.name}”.`);
      uploadForm.reset();
      selectedFile = null;
      status($("#selected-file"));
      if (pageName === "files") await loadFiles(1);
    } catch (error) {
      status($("#upload-status"), error.message + " Choose Upload file to retry.", true);
    } finally {
      uploading = false;
      fileInput.disabled = $('button[type="submit"]', uploadForm).disabled = false;
    }
  });

  window.addEventListener("beforeunload", event => {
    if (uploading) {
      event.preventDefault();
      event.returnValue = "";
    }
  });
})();
