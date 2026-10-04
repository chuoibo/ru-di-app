/* --------------------------------------------------- the shape of a number */

const DAU_PHAN_CACH = /[\s.\-()]/g;

/**
 * Nine digits after the trunk prefix, first of them one of 3 5 7 8 9.
 *
 * A copy of `_MOBILE` in `services/api/app/api/person_identity.py`, and a copy
 * is a liability, so it is a *checked* copy: `tests/ban-be.test.mjs` reads the
 * regex out of that Python file and fails if the two stop agreeing. Same trick
 * `tests/publish-refusals.test.mjs` uses on the publish gate codes, for the
 * same reason -- a client rule that drifts from the server rule produces a
 * refusal the person holding the phone cannot act on.
 */
const SO_DI_DONG = /^[35789]\d{8}$/;

/**
 * Does this look like a Vietnamese mobile number at all?
 *
 * Deliberately NOT an authority on validity -- the server decides that, and
 * its 422 says so in Vietnamese. This exists for one narrower job: the lookup
 * route allows thirty calls a minute per caller, and spending one of them on a
 * half-typed number means the person who then types it correctly is the one
 * who gets throttled. So the button stays inert until the field holds
 * something that could be a number.
 *
 * Accepts what `canonical_mobile` accepts: `+84`, a bare `84`, a trunk `0`, or
 * none of the three, with spaces, dots, dashes and brackets anywhere.
 */
export function soCoTheGoi(raw) {
  const packed = raw.replace(DAU_PHAN_CACH, "");
  if (packed === "") return false;
  const rest = packed.startsWith("+84")
    ? packed.slice(3)
    : packed.startsWith("84")
      ? packed.slice(2)
      : packed.startsWith("0")
        ? packed.slice(1)
        : packed;
  return SO_DI_DONG.test(rest);
}

