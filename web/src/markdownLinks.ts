import { api, type ArchiveFile, type Folder } from "./api";

export function markdownURL(
  raw: string,
): "external" | "anchor" | "relative" | "blocked" {
  if (!raw || /[\u0000-\u0020\\]/.test(raw)) return "blocked";
  if (raw.startsWith("#")) return "anchor";
  if (/^(https?:)?\/\//i.test(raw)) return "external";
  if (/^mailto:/i.test(raw)) return "external";
  return /^[a-z][a-z\d+.-]*:/i.test(raw) ? "blocked" : "relative";
}

export function relativeTarget(
  raw: string,
  folderID: string,
  folders: Folder[],
) {
  if (markdownURL(raw) !== "relative") return null;
  let parts: string[];
  try {
    parts = raw.split(/[?#]/, 1)[0].split("/").map(decodeURIComponent);
  } catch {
    return null;
  }
  if (parts.some((part) => /[\\/\u0000]/.test(part))) return null;
  let folder = folders.find((f) =>
    raw.startsWith("/") ? !f.parent_id : f.id === folderID,
  );
  const name = parts.pop();
  if (!folder || !name || name === "." || name === "..") return null;
  for (const part of parts) {
    if (!part || part === ".") continue;
    folder =
      part === ".."
        ? folders.find((f) => f.id === folder!.parent_id)
        : folders.find((f) => f.parent_id === folder!.id && f.name === part);
    if (!folder) return null;
  }
  return { folder: folder.id, name };
}

export async function resolveMarkdownFile(
  file: ArchiveFile,
  raw: string,
  signal?: AbortSignal,
) {
  if (!file.box_id) throw new Error("Missing archive box");
  const { folders } = await api.folders(file.box_id, signal);
  const target = relativeTarget(raw, file.folder_id, folders || []);
  if (!target) throw new Error("Invalid relative path");
  const result = await api.files(
    {
      box: [file.box_id],
      folder: target.folder,
      name_equals: target.name,
      limit: 2,
    },
    signal,
  );
  const matches =
    result.files?.filter(
      (f) => f.folder_id === target.folder && f.name === target.name,
    ) || [];
  if (matches.length !== 1) throw new Error("File not found");
  return matches[0];
}
