import { useCallback, useRef, useState } from "react";

/** A draft revision owns only the words that existed when an upload started. */
export function useBanNhap() {
  const [text, setText] = useState("");
  const snapshot = useRef({ text: "", revision: 0 });
  const change = useCallback((next: string) => {
    snapshot.current = { text: next, revision: snapshot.current.revision + 1 };
    setText(next);
  }, []);
  const clearIfUnchanged = useCallback((revision: number) => {
    if (snapshot.current.revision === revision) change("");
  }, [change]);
  return { text, change, snapshot, clearIfUnchanged };
}
