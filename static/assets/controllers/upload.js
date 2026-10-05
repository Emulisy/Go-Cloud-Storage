import { $, bytes, status, sizeLabel, toast } from "../views/shared.js?v=14";
import { createUploadView } from "../views/upload.js?v=14";
import { ordinaryLimit } from "../config.js?v=14";
import { hashFile, checkStoredFile, ordinaryUpload, multipartUpload } from "../requests/uploads.js?v=14";
export function initUpload(pageName, loadFiles) {
  if (!$("#upload-form")) return;

  const transferEvents = {
    onTransfer(transfer) {
      activeTransfer = transfer;
      $("#cancel-upload").hidden = false;
      $("#pause-upload").hidden = transfer.mode !== "multipart";
    },
    onProgress(uploaded, total, saving = false) {
      transferProgress(uploaded, total);
      if (activeTransfer.mode === "ordinary") status($("#upload-status"), saving ? "Saving your file…" : "Your file is on its way.");
      else setStage("uploading", "Uploading");
    },
    onSaving() { setStage("verifying", "Saving…"); $("#cancel-upload").hidden = true; },
    onResume(saved) {
      setStage("uploading", "Resumed");
      status($("#upload-status"), `Upload resumed. ${sizeLabel(saved)} was already saved.`);
      toast("Picking up from your saved progress.");
    },
    onVerifying() {
      $("#pause-upload").hidden = $("#cancel-upload").hidden = true;
      setStage("verifying", "Verifying"); status($("#upload-status"), "Checking your file before it joins your workspace…");
    }
  };
  let selectedFile = null, uploading = false, activeTransfer = null, hashForFile = null;
  const state = { get uploading() { return uploading; } };
  const { uploadForm, submitUpload, fileInput, progress, setStage, transferProgress } = createUploadView(pageName, state, file => { selectedFile = file; hashForFile = null; });
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
          hashForFile = await hashFile(file);
        }
        status($("#upload-status"), "Checking file…"); setStage("preparing", "Checking file…");
        reused = await checkStoredFile(file, hashForFile);
      } else if (file.size > ordinaryLimit) throw new Error("Large uploads need a secure connection. Open GoCloud using HTTPS.");
      let result = {};
      if (!reused) {
        setStage("uploading", "Uploading"); status($("#upload-status"), "Your file is on its way.");
        progress.value = 0; transferProgress(0, file.size); submitUpload.textContent = "Uploading…";
        result = file.size > ordinaryLimit ? await multipartUpload(file, hashForFile, transferEvents) : await ordinaryUpload(file, transferEvents);
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
}
