import { createContext, useContext } from "react";

export const InputViewportContext = createContext<number | null>(null);

/** Actual scroll viewport while its screen's IME is open; no native reads. */
export function useRudiInputViewportHeight() { return useContext(InputViewportContext); }
