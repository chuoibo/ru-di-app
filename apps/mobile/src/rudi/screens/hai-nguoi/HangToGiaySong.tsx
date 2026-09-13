import { useRouter } from "expo-router";

import { useToGiay } from "../../to-giay/useToGiay";
import { HangToGiay } from "./HangToGiay";

/**
 * The pinned line in a pair's chat, reading the real notebook.
 *
 * A component rather than a hook call in `GroupChatLive`, and that is the whole
 * reason it exists: hooks cannot be called conditionally, so reading the
 * notebook up there would fire a request for every GROUP conversation too --
 * a 404 per chat open, to render nothing. Mounting this only for a pair puts
 * the condition where React can honour it.
 *
 * `nhip: 0`: read on focus, never on a timer. The row says one sentence about
 * the sheet, and a second four-second poll beside the conversation's own would
 * double this screen's traffic to say it a beat sooner.
 */
export function HangToGiaySong({ contextId, toiId }: { contextId: string; toiId: string }) {
  const router = useRouter();
  const so = useToGiay(contextId, toiId, { nhip: 0 });
  return (
    <HangToGiay
      onPress={() => router.push(`/groups/${contextId}/to-giay` as never)}
      toMo={so.to ?? undefined}
      toiId={toiId}
    />
  );
}
