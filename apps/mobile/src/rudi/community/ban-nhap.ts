/** Session-memory drafts; no post text or selected-file bytes are written to disk. */
import type { Audience, Media } from "./api";
import type { SelectedFile } from "./MediaPicker";

export type NhapBai = {
  body: string;
  audience: Audience;
  topics: string;
  mentions: string[];
  media: Media[];
  group: string | null;
  revision: number;
  logical_id: string;
  pendingFile: SelectedFile | null;
};

let owner: string | null = null;
let epoch = 0;
const drafts = new Map<string, NhapBai>();

/** An old account's delayed completion cannot write into a later session. */
export function datChuSoHuuNhapBai(person: string | null): void {
  if (owner !== person) { drafts.clear(); epoch++; }
  owner = person;
}

export function phienNhapBai(person: string): number | null {
  return owner === person ? epoch : null;
}

export function khoaNhapBai(edit: string | undefined, wall: boolean): string {
  return edit ? `edit:${edit}` : wall ? "new:wall" : "new:community";
}

function clone(value: NhapBai): NhapBai {
  return { ...value, mentions: [...value.mentions], media: value.media.map((m) => ({ ...m })), pendingFile: value.pendingFile ? { ...value.pendingFile, asset: { ...value.pendingFile.asset }, uploaded: value.pendingFile.uploaded ? { ...value.pendingFile.uploaded } : undefined } : null };
}

export function docNhapBai(person: string, scope: string): NhapBai | null {
  const value = owner === person ? drafts.get(scope) : null;
  return value ? clone(value) : null;
}

export function ghiNhapBai(person: string, scope: string, value: NhapBai, lease: number | null): void {
  if (owner !== person || lease !== epoch) return;
  drafts.set(scope, clone(value));
}

/** An acknowledgement clears only the submitted draft, never newer input. */
export function boNhapBai(person: string, scope: string, lease: number | null, logical?: string): void {
  if (owner !== person || lease !== epoch) return;
  if (logical !== undefined && drafts.get(scope)?.logical_id !== logical) return;
  drafts.delete(scope);
}
