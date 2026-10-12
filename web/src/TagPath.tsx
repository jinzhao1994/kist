import type { Tag } from "./api";

export type FlatTag = Tag & { ancestors: string[] };
export function flattenTags(tags: Tag[], ancestors: string[] = []): FlatTag[] {
  return tags.flatMap((tag) => [
    { ...tag, ancestors },
    ...flattenTags(tag.children || [], [...ancestors, tag.name]),
  ]);
}
export type TagOption = { value: string; label: string; prefix?: string };
export function tagOption(tag: FlatTag): TagOption {
  const prefix = tag.ancestors.length ? `${tag.ancestors.join(" / ")} / ` : "";
  return { value: tag.id, label: prefix + tag.name, prefix };
}
export function TagPath({ option }: { option: TagOption }) {
  return (
    <bdi className="tag-path" title={option.label}>
      {option.prefix && (
        <>
          <span className="tag-path-prefix">
            {option.prefix.trimEnd()}
          </span>{" "}
        </>
      )}
      <span>{option.label.slice(option.prefix?.length || 0)}</span>
    </bdi>
  );
}
