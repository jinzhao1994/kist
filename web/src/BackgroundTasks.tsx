import { useQuery } from "@tanstack/react-query";
import { BackgroundTaskQueue } from "./AI";
import { HelpTooltip } from "./HelpTooltip";
import { settingsQueries, type MetadataTask } from "./settingsQueries";
import { useI18n, type MessageKey } from "./i18n";

export function BackgroundTasks({
  disabled = false,
  aiPending = false,
  locationPending = false,
}: {
  disabled?: boolean;
  aiPending?: boolean;
  locationPending?: boolean;
}) {
  const { t } = useI18n();
  const connection = useQuery(settingsQueries.connection);
  const ai = useQuery({
    ...settingsQueries.ai,
    enabled: !!connection.data?.supported,
  });
  const location = useQuery(settingsQueries.location);
  const kinds: MetadataTask[] = connection.data?.supported
    ? ["orientation", "ocr", "desc", "facts", "geo"]
    : ["geo"];
  return (
    <section
      id="settings-background-tasks"
      className="flat-page-section"
      aria-labelledby="settings-background-tasks-title"
    >
      <h2
        id="settings-background-tasks-title"
        className="settings-section-heading"
        aria-label={t("backgroundTasks")}
      >
        {t("backgroundTasks")}
        <HelpTooltip label={t("backgroundTasks")}>
          {t("backgroundTasksHint")}
        </HelpTooltip>
      </h2>
      <div className="settings-blocks">
        {kinds.map((kind) => (
          <BackgroundTaskQueue
            key={kind}
            kind={kind}
            disabled={
              disabled || (kind === "geo" ? locationPending : aiPending)
            }
            connected={
              kind === "geo" ? !!location.data : !!connection.data?.connected
            }
            model={kind === "geo" ? "photon" : ai.data?.model}
            hint={
              kind === "geo"
                ? undefined
                : t(`${kind}BackfillHint` as MessageKey)
            }
          />
        ))}
      </div>
    </section>
  );
}
