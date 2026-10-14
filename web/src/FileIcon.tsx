import type { ArchiveFile } from "./api";

export function fileType(file: ArchiveFile): string | undefined {
  const extension = file.name.match(/\.([a-z0-9]{1,12})$/i)?.[1];
  const known: Record<string, string> = {
    "text/plain": "TXT",
    "application/pdf": "PDF",
    "image/jpeg": "JPG",
    "image/png": "PNG",
    "video/mp4": "MP4",
    "audio/mpeg": "MP3",
    "application/zip": "ZIP",
  };
  return (
    extension?.toUpperCase() ||
    known[(file.media_type || "").split(";")[0].trim().toLowerCase()]
  );
}

export function FileIcon({ folder = false }: { folder?: boolean }) {
  return (
    <svg
      className="browser-item-icon"
      width="24"
      height="24"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {folder ? (
        <path d="M3 7V5a1 1 0 0 1 1-1h5l2 3h9a1 1 0 0 1 1 1v11a1 1 0 0 1-1 1H4a1 1 0 0 1-1-1V7Z" />
      ) : (
        <>
          <path d="M14 3H5v18h14V8Z M14 3v5h5" />
          <path d="M8 12h8M8 16h6" />
        </>
      )}
    </svg>
  );
}
