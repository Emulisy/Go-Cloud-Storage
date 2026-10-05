import { $, makeIcon, metadata, toast, status, sizeLabel } from "./shared.js?v=14";
import { ordinaryLimit } from "../config.js?v=14";
export function createUploadView(pageName, state, onSelect) {
  const uploadPanel = $("#upload-panel"), uploadForm = $("#upload-form");
  const submitUpload = uploadForm && $('button[type="submit"]', uploadForm);
  const fileInput = $("#upload-file"), progress = $("#upload-progress");
  const dialog = $("#file-dialog");
  function openUpload(focus = true) {
    uploadPanel.hidden = false;
    if (focus) { uploadPanel.scrollIntoView({ behavior: matchMedia("(prefers-reduced-motion: reduce)").matches ? "auto" : "smooth", block: "nearest" }); fileInput.focus({ preventScroll: true }); }
  }
  document.querySelectorAll("[data-open-upload]").forEach(button => button.addEventListener("click", () => openUpload()));
  $("#close-upload").addEventListener("click", () => {
    uploadPanel.hidden = true;
    if (state.uploading) toast("Your transfer is still running. Open Upload file to see its progress.");
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
    if (state.uploading) return;
    onSelect(file || null);
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
  fileInput.addEventListener("change", () => selectFile(fileInput.files[0]));

  // File drops work throughout the files workspace and in the upload panel.
  let dragDepth = 0;
  const dropOverlay = $("#workspace-drop-target"), zone = $("#drop-zone");
  const fileDrag = event => [...(event.dataTransfer?.types || [])].includes("Files");
  document.addEventListener("dragenter", event => {
    if (!fileDrag(event)) return;
    event.preventDefault(); dragDepth++;
    if (dropOverlay && !state.uploading && !dialog?.open) dropOverlay.hidden = false;
    zone.classList.toggle("drag-active", !state.uploading);
  });
  document.addEventListener("dragover", event => {
    if (!fileDrag(event)) return;
    event.preventDefault(); event.dataTransfer.dropEffect = state.uploading ? "none" : "copy";
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
    if (state.uploading || dialog?.open) return;
    openUpload();
    if (event.dataTransfer.files.length !== 1) { status($("#upload-status"), "Choose one file at a time.", true); return; }
    fileInput.files = event.dataTransfer.files; selectFile(fileInput.files[0]);
  });
  window.addEventListener("blur", () => { dragDepth = 0; if (dropOverlay) dropOverlay.hidden = true; zone.classList.remove("drag-active"); });
  return { uploadForm, submitUpload, fileInput, progress, setStage, transferProgress };
}
