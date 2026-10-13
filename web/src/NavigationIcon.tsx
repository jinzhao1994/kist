export function NavigationIcon({
  kind,
}: {
  kind: "files" | "boxes" | "trash" | "history";
}) {
  return (
    <svg
      className="navigation-icon"
      width="18"
      height="18"
      viewBox="0 0 24 24"
      fill="none"
      stroke="currentColor"
      strokeWidth="1.5"
      strokeLinecap="round"
      strokeLinejoin="round"
      aria-hidden="true"
    >
      {kind === "files" && (
        <>
          <rect x="3" y="3" width="7" height="7" rx="1" />
          <rect x="14" y="3" width="7" height="7" rx="1" />
          <rect x="3" y="14" width="7" height="7" rx="1" />
          <rect x="14" y="14" width="7" height="7" rx="1" />
        </>
      )}
      {kind === "boxes" && (
        <>
          <rect x="3" y="4" width="18" height="5" rx="1" />
          <path d="M5 9v11h14V9M10 13h4" />
        </>
      )}
      {kind === "trash" && (
        <>
          <path d="M3 6h18M9 6V3h6v3M5 6l1 15h12l1-15M10 10v7M14 10v7" />
        </>
      )}
      {kind === "history" && (
        <>
          <path d="M3 10a9 9 0 1 1 2 8M3 4v6h6M12 7v5l3 2" />
        </>
      )}
    </svg>
  );
}
