import { TextInput, Textarea, Button } from "@mantine/core";
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
        className="box-editor-form"
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
          <TextInput
            label={t("name")}
            id={`${id}-name`}
            value={name}
            onChange={(e) => setName(e.target.value)}
            required
            withAsterisk={false}
            maxLength={255}
            autoFocus
          />
          <Textarea
            label={t("description")}
            id={`${id}-description`}
            value={description}
            onChange={(e) => setDescription(e.target.value)}
            rows={3}
          />
          <div className="box-editor-dates">
            <div className="field-pair">
              <div>
                <TextInput
                  label={t("startAt")}
                  id={`${id}-start`}
                  value={start}
                  onChange={(e) => setStart(e.target.value)}
                  aria-describedby={`${id}-hint`}
                />
              </div>
              <div>
                <TextInput
                  label={t("endAt")}
                  id={`${id}-end`}
                  value={end}
                  onChange={(e) => setEnd(e.target.value)}
                  aria-describedby={`${id}-hint`}
                />
              </div>
            </div>
            <p
              className="supporting-text box-editor-date-hint"
              id={`${id}-hint`}
            >
              {t("dateHint")}
            </p>
          </div>
        </fieldset>
        <WriteFeedback state={state} />
        <div className="actions">
          <Button type="button" disabled={state.busy} onClick={close}>
            {t("cancel")}
          </Button>
          <Button
            variant="filled"
            type="submit"
            disabled={state.busy || state.locked}
          >
            {t(state.busy ? "saving" : "save")}
          </Button>
        </div>
      </form>
    </Dialog>
  );
}
