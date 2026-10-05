export const $ = (selector, root = document) => root.querySelector(selector);

export const bytes = value => new TextEncoder().encode(value).length;

// Small, shared inline icons; user content never enters SVG markup.
const iconPaths = {
  file: '<path d="M14 2H6a2 2 0 0 0-2 2v16a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V8Z"/><path d="M14 2v6h6M8 13h8M8 17h5"/>',
  more: '<circle cx="5" cy="12" r="1"/><circle cx="12" cy="12" r="1"/><circle cx="19" cy="12" r="1"/>',
  download: '<path d="M12 3v12m-4-4 4 4 4-4M4 17v3a1 1 0 0 0 1 1h14a1 1 0 0 0 1-1v-3"/>',
  rename: '<path d="m15 4 5 5M4 20l5-1L20 8a2 2 0 0 0-5-5L4 14Z"/>',
  delete: '<path d="M3 6h18M9 6V3h6v3M5 6l1 15h12l1-15M10 10v7M14 10v7"/>'
};
export function makeIcon(kind) {
  const node = document.createElementNS("http://www.w3.org/2000/svg", "svg");
  for (const [key, value] of Object.entries({ viewBox: "0 0 24 24", fill: "none", stroke: "currentColor", "stroke-width": "2", "stroke-linecap": "round", "stroke-linejoin": "round", "aria-hidden": "true", class: "icon" })) node.setAttribute(key, value);
  node.innerHTML = iconPaths[kind];
  return node;
}

Object.assign(iconPaths, {
  image: '<rect x="3" y="3" width="18" height="18" rx="2"/><circle cx="8" cy="8" r="1.5"/><path d="m21 15-5-5L5 21"/>',
  video: '<rect x="3" y="4" width="18" height="16" rx="2"/><path d="m10 8 6 4-6 4ZM7 4v16M17 4v16"/>',
  audio: '<path d="M9 18V5l11-2v13M9 8l11-2"/><ellipse cx="6" cy="18" rx="3" ry="3"/><ellipse cx="17" cy="16" rx="3" ry="3"/>',
  archive: '<path d="M14 3H6a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h12a2 2 0 0 0 2-2V9Z"/><path d="M14 3v6h6M10 3v2h2v2h-2v2h2v2h-2v2h2v4h-3v-4"/>',
  code: '<path d="m8 7-5 5 5 5m8-10 5 5-5 5m-3-13-2 20"/>',
  check: '<path d="m5 12 4 4L19 6"/>'
});

export function metadata(name) {
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

export function toast(message) {
  const stack = $("#toast-stack");
  if (!stack) return;
  const node = document.createElement("div");
  node.className = "toast";
  node.append(makeIcon("check"), document.createTextNode(message));
  stack.replaceChildren(node);
  setTimeout(() => node.remove(), 4500);
}

export function status(node, message = "", error = false) {
  if (!node) return;
  node.textContent = message;
  node.classList.toggle("is-error", error);
  if (node.id === "upload-status" && $("#upload-form")?.getAttribute("aria-busy") === "true") {
    const button = $('#upload-form button[type="submit"]');
    button.textContent = /Verifying/.test(message) ? "Verifying…" : /Preparing|Checking/.test(message) ? "Preparing…" : "Uploading…";
  }
}

export function sizeLabel(size) {
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

export function dateLabel(value) {
  const date = new Date(value);
  return Number.isNaN(date.getTime()) ? "—" : date.toLocaleString(undefined, {
    year: "numeric",
    month: "short",
    day: "numeric",
    hour: "numeric",
    minute: "2-digit"
  });
}
