import { request } from "./http.js?v=14";

// Preserve server order and publish totals only after every page is available.
export async function getFiles() {
  const files = [], seen = new Set();
  for (let currentPage = 1; ; currentPage++) {
    const batch = await (await request(`/api/files?page=${currentPage}&pageSize=100`)).json();
    if (!Array.isArray(batch) || batch.some(file => !file || !Number.isSafeInteger(file.id) || typeof file.fileName !== "string" || !Number.isFinite(file.fileSize))) throw new Error("Unable to read the file list.");
    let newCount = 0;
    for (const file of batch) { if (!seen.has(file.id)) { seen.add(file.id); files.push(file); newCount++; } }
    if (batch.length < 100) break;
    if (!newCount) throw new Error("The file list changed while loading. Please refresh.");
  }
  return files;
}

export function renameFile(id, name) {
  return request(`/api/files/${encodeURIComponent(id)}`, { method: "PATCH", body: new URLSearchParams({ name }) });
}

export function deleteFile(id) {
  return request(`/api/files/${encodeURIComponent(id)}`, { method: "DELETE" });
}
