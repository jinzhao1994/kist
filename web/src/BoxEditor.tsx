import { useId, useState } from "react";
import { createWrite, type Box } from "./api";
import { Dialog } from "./Dialog";
import { useI18n } from "./i18n";
import { useWrites, WriteFeedback } from "./mutations";
export function BoxEditor({ box, close }: { box?: Box; close: () => void }) {
  const { t } = useI18n();
  const id = useId();
  const [name, setName] = useState(box?.name || "");
  const [description, setDescription] = useState(box?.description || "");
  const [start, setStart] = useState(box?.start_at || "");
  const [end, setEnd] = useState(box?.end_at || "");
  const state = useWrites(close);
  return (
    <Dialog
      open
      onOpenChange={(open) => {
        if (!open && !state.busy) close();
      }}
      title={t(box ? "editBox" : "createBox")}
    >
      <form
        onSubmit={(event) => {
          event.preventDefault();
          if (state.locked) return;
          state.run([
            createWrite(
              box ? "PUT" : "POST",
              box ? `/boxes/${box.id}` : "/boxes",
              { name, description, start_at: start, end_at: end },
            ),
          ]);
        }}
      >
        <fieldset disabled={state.busy || state.locked}>
          <label htmlFor={`${id}-name`}>{t("name")}</label>
          <input
            id={`${id}-name`}
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            maxLength={255}
            autoFocus
          />
          <label htmlFor={`${id}-description`}>{t("description")}</label>
          <textarea
            id={`${id}-description`}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            rows={3}
          />
          <div className="field-pair">
            <div>
              <label htmlFor={`${id}-start`}>{t("startAt")}</label>
              <input
                id={`${id}-start`}
                value={start}
                onChange={(e) => setStart(e.target.value)}
                aria-describedby={`${id}-hint`}
              />
            </div>
            <div>
              <label htmlFor={`${id}-end`}>{t("endAt")}</label>
              <input
                id={`${id}-end`}
                value={end}
                onChange={(e) => setEnd(e.target.value)}
                aria-describedby={`${id}-hint`}
              />
            </div>
          </div>
          <p className="hint" id={`${id}-hint`}>
            {t("dateHint")}
          </p>
        </fieldset>
        <WriteFeedback state={state} />
        <div className="actions">
          <button type="button" disabled={state.busy} onClick={close}>
            {t("cancel")}
          </button>
          <button className="primary" disabled={state.busy || state.locked}>
            {t(state.busy ? "saving" : "save")}
          </button>
        </div>
      </form>
    </Dialog>
  );
}
