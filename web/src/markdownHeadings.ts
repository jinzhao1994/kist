// Assign stable, document-local heading IDs without rendering raw HTML.
type Node = {
  type: string;
  tagName?: string;
  value?: string;
  properties?: Record<string, unknown>;
  children?: Node[];
};
export function markdownHeadings() {
  return (tree: Node) => {
    const used = new Set<string>();
    const text = (node: Node): string =>
      node.value || node.children?.map(text).join("") || "";
    const visit = (node: Node) => {
      if (node.tagName && /^h[1-6]$/.test(node.tagName)) {
        const base =
          text(node)
            .toLowerCase()
            .replace(/[^\p{L}\p{N}_\s-]/gu, "")
            .trim()
            .replace(/\s/g, "-") || "section";
        let id = base,
          suffix = 0;
        while (used.has(id)) id = `${base}-${++suffix}`;
        used.add(id);
        node.properties = { ...node.properties, id: `markdown-${id}` };
      }
      node.children?.forEach(visit);
    };
    visit(tree);
  };
}
