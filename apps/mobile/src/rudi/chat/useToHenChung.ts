import { useCallback, useEffect, useRef, useState } from "react";
import { taoKeoTuChat } from "./ai-invocations";
import {
  boToHenChung,
  docToHenChung,
  moToHenChung,
  suaToHenChung,
  type BanNhapToHen,
  type ToHenChung,
} from "./to-hen-chung";

/**
 * Holds the one shared sheet a screen is editing.
 *
 * The rule this hook exists to keep: an edit always carries the revision the
 * person was looking at. When the server says that copy is stale, the hook
 * re-reads the sheet and hands the fresh one back with a plain sentence —
 * it does not retry silently, because retrying would re-apply an edit to a
 * sheet the person has not seen.
 *
 * Generation guards follow the same shape as `useChatAi`: a response that
 * arrives after the room or the identity changed is dropped, never applied.
 */
export function useToHenChung(contextId: string, personId: string) {
  const [sheet, setSheet] = useState<ToHenChung | null>(null);
  const [busy, setBusy] = useState(false);
  const [error, setError] = useState<string | null>(null);
  const [stale, setStale] = useState(false);
  const generation = useRef(0);
  const alive = useRef(true);

  useEffect(() => {
    generation.current += 1;
    setSheet(null);
    setError(null);
    setStale(false);
  }, [contextId, personId]);

  useEffect(() => {
    alive.current = true;
    return () => {
      alive.current = false;
    };
  }, []);

  const guarded = useCallback(
    async <T,>(work: () => Promise<T>): Promise<T | null> => {
      const mine = generation.current;
      setBusy(true);
      try {
        const value = await work();
        if (!alive.current || mine !== generation.current) return null;
        return value;
      } finally {
        if (alive.current && mine === generation.current) setBusy(false);
      }
    },
    [],
  );

  const open = useCallback(
    async (id: string) => {
      setError(null);
      const value = await guarded(() => docToHenChung(contextId, personId, id));
      if (value) {
        setSheet(value);
        setStale(false);
      } else if (alive.current) setError("Chưa mở được tờ hẹn. Thử lại nhé.");
      return value;
    },
    [contextId, personId, guarded],
  );

  const create = useCallback(
    async (draft: BanNhapToHen) => {
      setError(null);
      try {
        const value = await guarded(() => moToHenChung(contextId, personId, draft));
        if (value) {
          setSheet(value);
          setStale(false);
        }
        return value;
      } catch {
        if (alive.current) setError("Chưa mở được tờ hẹn chung. Thử lại nhé.");
        return null;
      }
    },
    [contextId, personId, guarded],
  );

  const edit = useCallback(
    async (patch: Partial<BanNhapToHen>) => {
      const current = sheet;
      if (!current) return null;
      setError(null);
      try {
        const value = await guarded(() =>
          suaToHenChung(contextId, personId, current.id, current.revision, patch),
        );
        if (value) {
          setSheet(value);
          setStale(false);
        }
        return value;
      } catch {
        // Somebody else saved first. Show them what the sheet says now, and
        // let them decide what to keep.
        const fresh = await guarded(() => docToHenChung(contextId, personId, current.id));
        if (fresh && alive.current) {
          setSheet(fresh);
          setStale(true);
          setError("Có người vừa sửa tờ này. Đây là bản mới nhất; xem rồi sửa lại nhé.");
        } else if (alive.current) setError("Chưa lưu được thay đổi. Thử lại nhé.");
        return null;
      }
    },
    [contextId, personId, sheet, guarded],
  );

  const discard = useCallback(async () => {
    const current = sheet;
    if (!current) return null;
    setError(null);
    try {
      const value = await guarded(() => boToHenChung(contextId, personId, current.id));
      if (value) setSheet(value);
      return value;
    } catch {
      if (alive.current) setError("Chưa bỏ được tờ hẹn. Thử lại nhé.");
      return null;
    }
  }, [contextId, personId, sheet, guarded]);

  /**
   * Turn this sheet into the kèo, from the tray the person is already in.
   * The thread's own card is what changes, so the transformation happens where
   * they are looking instead of on another route.
   */
  const confirm = useCallback(async () => {
    const current = sheet;
    if (!current) return null;
    if (!current.starts_on || !current.ends_on) {
      setError("Thêm ngày đi và ngày về trước khi chốt.");
      return null;
    }
    if (current.stops.length === 0) {
      setError("Tờ hẹn cần ít nhất một chặng.");
      return null;
    }
    setError(null);
    try {
      const promoted = await guarded(() =>
        taoKeoTuChat(
          contextId,
          personId,
          current.message_id,
          {
            title: current.title,
            starts_on: current.starts_on as string,
            ends_on: current.ends_on as string,
            headcount: current.headcount ?? 0,
            budget_per_person_vnd: current.budget_per_person_vnd ?? 0,
          },
          current.stops.map((stop) => ({ at: stop.time_text, label: stop.label, place_name: null })),
        ),
      );
      if (promoted) setSheet({ ...current, status: "promoted" });
      return promoted;
    } catch (failure) {
      if (alive.current) {
        setError(failure instanceof Error && failure.message ? failure.message : "Chưa chốt được kèo. Thử lại nhé.");
      }
      return null;
    }
  }, [contextId, personId, sheet, guarded]);

  const close = useCallback(() => {
    setSheet(null);
    setError(null);
    setStale(false);
  }, []);

  return { sheet, busy, error, stale, open, create, edit, discard, confirm, close };
}
