/**
 * Invite somebody into a group by telephone number, as three server steps.
 *
 * The number becomes a person id, the id gets the name the inviter knows them
 * by, and the membership is created as `invited`. The middle step is refused
 * (403 `permission_denied`) when the person already signed in and so already
 * owns a name -- the server keeps theirs, and only they may change it
 * (`rename_person_identity`, is_self). That refusal used to abort the whole
 * invitation, so an inviter could only get through by typing the server's
 * placeholder «Thành viên mới» (QA 23/09). The membership step never needed the
 * name (`POST /contexts/{id}/members` asks only that the person exists), so a
 * name refusal now means "their own name stands", and the invitation goes on.
 *
 * Kept free of React Native so `tests/moi-bang-so.test.mjs` runs it in node.
 */
export type BuocMoi = {
  layId: (so: string) => Promise<string>;
  datTen: (personId: string, ten: string) => Promise<void>;
  moi: (personId: string) => Promise<void>;
};

export type KetQuaMoi = {
  /** False when the person already had a name of their own, which stands. */
  tenDaDat: boolean;
};

/** A refusal to rename somebody who owns their name, and nothing broader. */
export function laTuChoiDoiTen(error: unknown): boolean {
  if (typeof error !== "object" || error === null) return false;
  const e = error as { status?: unknown; code?: unknown };
  return e.status === 403 && e.code === "permission_denied";
}

export async function moiBangSo(buoc: BuocMoi, so: string, ten: string): Promise<KetQuaMoi> {
  const personId = await buoc.layId(so);
  let tenDaDat = true;
  try {
    await buoc.datTen(personId, ten);
  } catch (error) {
    if (!laTuChoiDoiTen(error)) throw error;
    tenDaDat = false;
  }
  await buoc.moi(personId);
  return { tenDaDat };
}
