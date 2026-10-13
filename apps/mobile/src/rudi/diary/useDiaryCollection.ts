import { useFocusEffect } from "expo-router";
import { useCallback, useRef, useState } from "react";
import { ApiError } from "../../api";
import { listDiaries, type Diary } from "./api";

type CollectionState = {
  scope: string;
  books: Diary[];
  next: string | null;
  loading: boolean;
  error: string | null;
  pendingBefore: string | null;
};

export type DiaryCollection = {
  books: Diary[];
  next: string | null;
  loading: boolean;
  error: string | null;
  reload: () => void;
  retry: () => void;
  more: () => void;
};

/** One focus-scoped reader for a person's preview and full shelf. */
export function useDiaryCollection(person: string, owner: string): DiaryCollection {
  const scope = `${person}\u0000${owner}`;
  const [state, setState] = useState<CollectionState>({ scope, books: [], next: null, loading: true, error: null, pendingBefore: null });
  const active = useRef({ scope, focused: false, generation: 0, busy: false });
  // Hide old actor data immediately, before the focus effect replaces the reader.
  if (active.current.scope !== scope) {
    active.current = { scope, focused: false, generation: active.current.generation + 1, busy: false };
  }
  const read = useCallback(async (before: string | null) => {
    const current = active.current;
    if (current.scope !== scope || !current.focused || current.busy) return;
    current.busy = true;
    const generation = ++current.generation;
    const stillCurrent = () => active.current === current && current.focused && current.generation === generation;
    setState(old => ({ scope, books: old.scope === scope ? old.books : [], next: old.scope === scope ? old.next : null, loading: true, error: null, pendingBefore: before }));
    try {
      const page = await listDiaries(person, owner, before ?? "");
      if (!stillCurrent()) return;
      setState(old => {
        const previous = before && old.scope === scope ? old.books : [];
        const ids = new Set(previous.map(book => book.id));
        return { scope, books: [...previous, ...page.diaries.filter(book => !ids.has(book.id))], next: page.next_cursor, loading: false, error: null, pendingBefore: null };
      });
    } catch (error) {
      if (!stillCurrent()) return;
      setState(old => ({ ...old, loading: false, error: error instanceof ApiError || error instanceof Error ? error.message : "Chưa mở được những trang đã giữ.", pendingBefore: before }));
    } finally {
      if (stillCurrent()) current.busy = false;
    }
  }, [person, owner, scope]);
  useFocusEffect(useCallback(() => {
    const current = active.current;
    current.focused = true;
    current.busy = false;
    void read(null);
    return () => { current.focused = false; current.busy = false; current.generation += 1; };
  }, [read]));
  const visible = state.scope === scope ? state : { scope, books: [], next: null, loading: true, error: null, pendingBefore: null };
  return {
    books: visible.books, next: visible.next, loading: visible.loading, error: visible.error,
    reload: useCallback(() => { void read(null); }, [read]),
    retry: useCallback(() => { void read(visible.pendingBefore); }, [read, visible.pendingBefore]),
    more: useCallback(() => { if (visible.next !== null) void read(visible.next); }, [read, visible.next]),
  };
}
