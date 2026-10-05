import { getFiles, renameFile, deleteFile } from "../requests/files.js?v=14";
import { createFilesView } from "../views/files.js?v=14";

export function initFiles(pageName) {
  let loading = false, refreshQueued = false;
  const view = createFilesView({ onRename: renameFile, onDelete: deleteFile, onRefresh: loadFiles });
  async function loadFiles(resetPage = false, animateNew = false) {
    if (!pageName) return;
    if (loading) { refreshQueued = refreshQueued || animateNew; return; }
    loading = true;
    const knownIds = new Set((view.files || []).map(file => file.id));
    view.setLoading(true);
    try {
      const files = await getFiles();
      view.setFiles(files, resetPage, animateNew ? new Set(files.filter(file => !knownIds.has(file.id)).map(file => file.id)) : new Set());
    } catch (error) { view.showError(error); }
    finally {
      loading = false; view.setLoading(false);
      if (refreshQueued) { refreshQueued = false; loadFiles(true, true); }
    }
  }
  if (pageName) loadFiles();
  return loadFiles;
}
