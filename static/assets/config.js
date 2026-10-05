// Shared upload limit and download routes; no requests or DOM access.
export const ordinaryLimit = 100 * 1024 * 1024;

export function downloadURL(id) {
  return `/api/files/${encodeURIComponent(id)}/content`;
}
