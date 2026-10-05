import { $, bytes, makeIcon, metadata, toast, status, sizeLabel, dateLabel } from "./shared.js?v=14";
import { closeFileMenus } from "./navigation.js?v=14";
import { downloadURL } from "../config.js?v=14";
export function createFilesView({ onRename, onDelete, onRefresh }) {
  let allFiles = null, page = 1, selectedId = null, editingId = null;
  let pendingDelete = null, detailTrigger = null, deleteTrigger = null;
  const pageSize = 20, rows = $("#file-rows"), detailPanel = $("#detail-panel");
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
    $("#detail-download").href = downloadURL(file.id);
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
        await onRename(file.id, name);
        file.fileName = name;
        editingId = null; renderFiles();
        if (selectedId === file.id) showDetails(file, null, false);
        [...rows.children].find(node => node.dataset.id === String(file.id))?.querySelector(".stored-name")?.focus();
        toast(`Renamed to “${name}”.`);
        await onRefresh();
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
        await onDelete(file.id);
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
    download.href = downloadURL(file.id); download.download = file.fileName;
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

  if (rows) {
    $("#previous-page").addEventListener("click", () => { page--; renderFiles(); });
    $("#next-page").addEventListener("click", () => { page++; renderFiles(); });
    $("#refresh-files").addEventListener("click", () => onRefresh());
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
  return {
    get files() { return allFiles; },
    setFiles(files, resetPage, arrivingIds) {
      allFiles = files; if (resetPage) page = 1;
      paintTotals();
      if (!rows) return;
      const selected = allFiles.find(file => file.id === selectedId);
      if (selected) showDetails(selected, null, false); else closeDetails(false);
      renderFiles(arrivingIds);
      status($("#files-status"));
    },
    setLoading(busy) {
      if (!rows) return;
      $("#refresh-files").disabled = busy;
      $(".file-browser").setAttribute("aria-busy", String(busy));
      if (busy) status($("#files-status"), allFiles ? "Refreshing files…" : "Loading files…");
      $("#file-skeleton").hidden = !busy || Boolean(allFiles);
    },
    showError(error) {
      if (rows) status($("#files-status"), error.message + " Select Refresh to retry.", true);
      else status($("#page-status"), "Storage information could not be loaded. Refresh this page to retry.", true);
      if (!allFiles) {
        document.querySelectorAll("[data-storage-total]").forEach(node => { node.textContent = "Unavailable"; });
        document.querySelectorAll("[data-storage-caption]").forEach(node => { node.textContent = "Refresh to try again"; });
        if ($("#workspace-summary")) $("#workspace-summary").textContent = "Your files couldn’t be loaded";
      }
    }
  };
}
