import { useQuery } from "@tanstack/react-query";
import { createWrite, get, type ArchiveFile, type Box } from "./api";
import { Media, hasPreview } from "./Media";
import { useWrites } from "./mutations";

export function useBoxCover() {
  const writes = useWrites();
  return {
    ...writes,
    setCover: (box: string, file = "") => {
      if (!writes.busy && !writes.locked)
        void writes.run([
          createWrite("PUT", `/boxes/${box}/cover`, { file_id: file }),
        ]);
    },
  };
}

export function BoxCover({ box }: { box: Box }) {
  const query = useQuery({
    queryKey: ["box-cover", box.id, box.cover_file_id],
    queryFn: ({ signal }) =>
      get<{ file?: ArchiveFile }>(`/boxes/${box.id}/cover`, signal),
    enabled: !!box.cover_file_id,
  });
  const file = query.data?.file;
  const icon = (
    <span className="box-icon" aria-hidden="true">
      ▤
    </span>
  );
  if (!box.cover_file_id) return icon;
  return (
    <div className="box-cover">
      {file && hasPreview(file) ? (
        <Media key={`${file.id}:${file.blob_key}`} file={file} />
      ) : (
        icon
      )}
    </div>
  );
}
