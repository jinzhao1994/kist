import { formatNumber, useI18n } from "./i18n";
import type { ReactNode } from "react";

export function UploadPanel({
  visible,
  title,
  progress,
  progressLabel,
  actions,
  children,
}: {
  visible: boolean;
  title: string;
  progress?: number | null;
  progressLabel?: string;
  actions: ReactNode;
  children: ReactNode;
}) {
  const { prefs } = useI18n();
  return (
    <div hidden={!visible} className="upload-panel">
      <div className="upload-panel-heading">
        <div className="upload-panel-status" role="status">
          <strong>{title}</strong>
          {typeof progress === "number" && (
            <span>{formatNumber(Math.round(progress * 100), prefs)}%</span>
          )}
        </div>
        <div className="upload-actions">{actions}</div>
      </div>
      {progress !== undefined && (
        <progress
          className="upload-progress"
          value={progress ?? undefined}
          max={1}
          aria-label={progressLabel || title}
        />
      )}
      <div className="upload-panel-details">{children}</div>
    </div>
  );
}
