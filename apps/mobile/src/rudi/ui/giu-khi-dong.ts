/**
 * What a sheet shows while it closes: the last thing it showed open.
 *
 * A sheet's body follows live state, and the state that closes a sheet often
 * changes what its body would say: the notebook opening while «Lập sổ» is up
 * drops the pending proposal, and the closing sheet redrew as the invitation
 * to propose again for four frames (QA UI-084). Open, the value passes through;
 * closing, the value it had when last open is held.
 */
import { useRef } from "react";

export function useGiuKhiDong<T>(open: boolean, value: T): T {
  const giu = useRef(value);
  if (open) giu.current = value;
  return open ? value : giu.current;
}
