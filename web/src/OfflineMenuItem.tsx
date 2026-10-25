import { Menu } from "@mantine/core";
import { useQuery, useMutation, useQueryClient } from "@tanstack/react-query";
import { get } from "./api";
import { clientCommand } from "./offline-client";
import {
  offlineOwner,
  offlinePath,
  type OfflineTarget,
} from "./offline-retention";
import { useI18n } from "./i18n";
import { errorMessage } from "./mutations";

export function OfflineMenuItem({
  target,
  prepare,
}: {
  target: OfflineTarget;
  prepare: () => void;
}) {
  const { t } = useI18n();
  const queries = useQueryClient();
  const selections = useQuery({
    queryKey: ["client", "offline"],
    queryFn: ({ signal }) =>
      get<Record<string, unknown>>("/client/offline", signal),
  });
  const retained = !!selections.data?.[offlineOwner(target)];
  const remove = useMutation({
    mutationFn: () => clientCommand(offlinePath(target), {}, "DELETE"),
    onSuccess: () => queries.invalidateQueries({ queryKey: ["client"] }),
  });
  return (
    <>
      <Menu.Item
        closeMenuOnClick={!retained}
        disabled={
          selections.isPending || selections.isError || remove.isPending
        }
        onClick={() => (retained ? remove.mutate() : prepare())}
      >
        {t(
          target.fileset
            ? retained
              ? "cancelKeepCombination"
              : "keepCombinationOffline"
            : retained
              ? "clientStopKeeping"
              : "clientKeepOffline",
        )}
      </Menu.Item>
      {(selections.error || remove.error) && (
        <Menu.Label role="alert">
          {errorMessage(selections.error || remove.error, t)}
        </Menu.Label>
      )}
    </>
  );
}
