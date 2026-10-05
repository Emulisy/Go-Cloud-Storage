import { request } from "./http.js?v=14";

export async function hashFile(file) {
  const digest = await crypto.subtle.digest("SHA-256", await file.arrayBuffer());
  return [...new Uint8Array(digest)].map(byte => byte.toString(16).padStart(2, "0")).join("");
}
export async function checkStoredFile(file, hash) {
  const result = await (await request("/api/files/fast", { method: "POST", body: new URLSearchParams({ filehash: hash, filename: file.name, filesize: String(file.size) }) })).json();
  if (typeof result.reused !== "boolean") throw new Error("Unable to check your file. Please retry.");
  return result.reused;
}
export function ordinaryUpload(file, { onTransfer, onProgress, onSaving }) {
  return new Promise((resolve, reject) => {
    const xhr = new XMLHttpRequest();
    onTransfer({ mode: "ordinary", xhr, cancelRequested: false });
    xhr.open("POST", "/api/files");
    xhr.upload.onprogress = event => {
      if (!event.lengthComputable) return;
      onProgress(Math.floor(file.size * event.loaded / event.total), file.size, event.loaded === event.total);
      if (event.loaded === event.total) {
        // Once the request body arrives, the server may commit the file.
        onSaving();
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

export async function multipartUpload(file, hash, { onTransfer, onProgress, onResume, onVerifying }) {
  const upload = await (await request("/api/uploads", { method: "POST", body: new URLSearchParams({ filehash: hash, filename: file.name, filesize: String(file.size) }) })).json();
  const chunkCount = Number(upload.chunkCount), chunkSize = Number(upload.chunkSize);
  if (!Number.isSafeInteger(chunkCount) || !Number.isSafeInteger(chunkSize) || chunkCount < 1 || chunkCount > 10000 || chunkSize <= 0 || chunkCount !== Math.ceil(file.size / chunkSize)) throw new Error("Unable to start this upload. Please try again.");
  const transfer = { mode: "multipart", hash, controller: new AbortController(), cancelRequested: false, pauseRequested: false, verifying: false };
  onTransfer(transfer);
  try {
    const info = await (await request(`/api/uploads/${hash}`, { signal: transfer.controller.signal })).json();
    if (!Array.isArray(info.uploadedChunks) || info.chunkCount !== chunkCount || info.uploadedChunks.some(index => !Number.isInteger(index) || index < 0 || index >= chunkCount)) throw new Error("Unable to read saved progress. Please retry.");
    const completed = new Set(info.uploadedChunks);
    let saved = [...completed].reduce((total, index) => total + Math.min(chunkSize, file.size - index * chunkSize), 0);
    onProgress(saved, file.size);
    if (completed.size) {
      onResume(saved);
    }
    for (let index = 0; index < chunkCount; index++) {
      if (transfer.pauseRequested || transfer.cancelRequested) throw new DOMException("Transfer stopped", "AbortError");
      if (!completed.has(index)) {
        await request(`/api/uploads/${hash}/parts/${index}`, { method: "PUT", headers: { "Content-Type": "application/octet-stream" }, body: file.slice(index * chunkSize, Math.min((index + 1) * chunkSize, file.size)), signal: transfer.controller.signal });
        completed.add(index); saved += Math.min(chunkSize, file.size - index * chunkSize);
        onProgress(saved, file.size);
      }
    }
    transfer.verifying = true;
    onVerifying();
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
