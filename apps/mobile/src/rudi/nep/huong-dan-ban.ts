/**
 * Which build of the app map this app was made with: the first 12 hex
 * characters of the sha256 of services/core/internal/huongdan/data/_rut.json.
 *
 * Written by `tools/rut-huong-dan.mjs` together with `_rut.json`; do not edit.
 * The server computes the same value from the copy it embeds
 * (`huongdan.BanDung()`), so a phiếu carrying it tells Nếp whether its manual
 * describes the app this person holds. `tests/huong-dan-khop-ma.test.mjs` and
 * a Go test in `services/core/internal/huongdan` turn a stale value red.
 */
export const HUONG_DAN_BAN = "4ee7366ddff8";
