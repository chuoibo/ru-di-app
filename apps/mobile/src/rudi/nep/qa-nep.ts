/**
 * The QA knob for Nếp, the floating assistant.
 *
 * Nếp paints over whatever route is open and its dock is draggable, so its
 * touch target can land on top of any button on any screen. The native `--otp`
 * table found this the hard way: the dock swallowed the tap on «Đồng ý» in
 * flow 25, the invite was never accepted, and nine flows after it went red for
 * a reason that had nothing to do with what they measure.
 *
 * With `EXPO_PUBLIC_QA_TAT_NEP=1` the floating layer is not mounted, so the
 * twenty-five flows that measure everything else become deterministic again.
 *
 * This is a knob, not a verdict on Nếp: switching it on means the table stops
 * covering Nếp at all, and Nếp still needs a flow of its own. Same shape as
 * `EXPO_PUBLIC_QA_TAT_KAV`, and the same rule applies — plain
 * `process.env.X` member read so Expo inlines it
 * (`tests/env-inlining.test.mjs`), and `tests/cau-hinh-ban-dung.test.mjs`
 * refuses it in any shippable eas.json profile.
 */
declare const process: { env: Record<string, string | undefined> };

export const TAT_NEP_QA: boolean = process.env.EXPO_PUBLIC_QA_TAT_NEP === "1";
