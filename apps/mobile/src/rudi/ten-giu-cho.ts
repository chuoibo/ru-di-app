/**
 * The server's placeholder name for a person who has not chosen one yet.
 *
 * `POST /auth/otp/verify` creates the person row under this exact string
 * (Go `authsteps.NewPersonName`, Python `NEW_PERSON_NAME`). It is not a name:
 * greeting somebody with it reads like a stranger's name, and showing it to the
 * person who looked them up by phone gives them no way to know they found the
 * right one (QA 23/09). `tests/ten-giu-cho.test.mjs` holds this equal to both
 * server spellings.
 *
 * Decide by VALUE, not by `is_new_person`: that flag is only true on the first
 * sign-in, and the second one greeted «Xin chào Thành viên mới».
 */
export const TEN_GIU_CHO = "Thành viên mới";

export function laTenGiuCho(ten: string | null | undefined): boolean {
  return ten === undefined || ten === null || ten.trim() === "" || ten.trim() === TEN_GIU_CHO;
}

/** A name fit to print, or null when all the server has is the placeholder. */
export function tenThat(ten: string | null | undefined): string | null {
  return laTenGiuCho(ten) ? null : (ten as string).trim();
}
