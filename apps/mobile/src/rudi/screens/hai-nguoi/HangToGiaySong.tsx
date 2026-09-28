import { useRouter } from "expo-router";

import { hangGhimChat } from "../../to-giay/so-doi-map";
import { useToGiay } from "../../to-giay/useToGiay";
import { HangLoiDeNghi, HangToGiay } from "./HangToGiay";

/**
 * The pinned line in a two-person chat, reading the real notebook.
 *
 * A couple (`cap_doi`: both turned «Một đôi» on) gets the full «Tờ giấy» row
 * (owner decision 2026-09-28). A friends' pair plans with «Tờ hẹn» like a group
 * and reaches the paper from the settings row «Tờ giấy của hai mình» -- so it
 * gets a slim line only while the other person's proposal (open the notebook,
 * or «Một đôi») waits for its answer, and nothing otherwise. `hangGhimChat`
 * decides which; this component only renders it.
 *
 * A component rather than a hook call in `GroupChatLive`, and that is the whole
 * reason it exists: hooks cannot be called conditionally, so reading the
 * notebook up there would fire a request for every GROUP conversation too --
 * a 404 per chat open, to render nothing. Mounting this only for a pair puts
 * the condition where React can honour it.
 *
 * `nhip: 0`: read on focus, never on a timer. The row says one sentence about
 * the sheet, and a second four-second poll beside the conversation's own would
 * double this screen's traffic to say it a beat sooner. The same one read
 * serves both kinds of pair, so a pair turning into a couple does not refetch.
 */
export function HangToGiaySong({
  contextId,
  toiId,
  tenNguoiKia,
  capDoi,
}: {
  contextId: string;
  toiId: string;
  tenNguoiKia?: string;
  capDoi: boolean;
}) {
  const router = useRouter();
  const so = useToGiay(contextId, toiId, { nhip: 0 });
  const hang = hangGhimChat({ haiNguoi: true, capDoi, so: so.so, toiId });
  if (hang === null) return null;
  const moGiay = () => router.push(`/groups/${contextId}/to-giay` as never);
  const deNghi = hang.deNghi ? { purpose: hang.deNghi.purpose, ten: tenNguoiKia ?? "Người ấy" } : undefined;
  if (hang.loai === "loi-moi" && deNghi) return <HangLoiDeNghi deNghi={deNghi} onPress={moGiay} />;
  return (
    <HangToGiay
      deNghiDenToi={deNghi}
      onPress={moGiay}
      tenNguoiKia={tenNguoiKia}
      toMo={so.to ?? undefined}
      toiId={toiId}
    />
  );
}
