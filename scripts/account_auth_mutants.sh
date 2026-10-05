#!/usr/bin/env bash
# Exercise four non-equivalent auth defects in disposable worktrees of one SHA.
set -euo pipefail
cd "$(dirname "$0")/.."
root="$PWD"
git diff --quiet && git diff --cached --quiet || {
  echo 'HỎNG: cổng mutant chỉ nhận cây sạch đúng SHA.' >&2; exit 2;
}
sha="$(git rev-parse HEAD)"
artifacts="$(mktemp -d /tmp/rudi-account-mutants.XXXXXX)"
mutant_tree="$artifacts/worktree"
cleanup() { git -C "$root" worktree remove --force "$mutant_tree" >/dev/null 2>&1 || true; }
trap cleanup EXIT INT TERM
tests='TestPostgresAuthHarnessRejectsWrongPassword|TestPostgresAccountProofResetAndSessionRevocation|TestPostgresGoogleNonceNoEmailMergeAndSafeLink|TestPostgresWrongCodesAreCappedPerEmailAcrossResends|TestPostgresLoginFailuresPauseTheGuesserNotTheOwner|TestPostgresTierReachesDatabase'
tier() {
  MOBILE_TEST_POSTGRES_DURABLE=1 scripts/go_postgres_tier.sh -- -run "$tests" ./internal/accountauth ./internal/db
}
echo "harness/source SHA=$sha" >"$artifacts/source.txt"
tier >"$artifacts/identity.log" 2>&1 || { echo "Identity đỏ: $artifacts" >&2; exit 1; }
if RUDI_AUTH_QA_CANARY=1 tier >"$artifacts/canary.log" 2>&1; then
  echo 'Canary sai mật khẩu vẫn xanh.' >&2; exit 1
fi
grep -q 'wrong-password control: HTTP 401, want 201' "$artifacts/canary.log" || {
  echo "Canary đỏ sai bước: $artifacts" >&2; exit 1;
}
for mutant in nonce reset-revocation otp-subject-cap login-pair; do
  git worktree add --detach "$mutant_tree" "$sha" >/dev/null 2>&1
  # Witnesses of non-equivalence: a signed token with the wrong nonce must
  # fail; an old session must fail after a successful password reset; the
  # right code must be refused once its email has spent thirty wrong ones
  # across challenges; and a stranger's ten wrong passwords must not lock the owner.
  python3 - "$mutant_tree" "$mutant" <<'PY'
from pathlib import Path
import sys

base = Path(sys.argv[1]) / "services/core/internal/accountauth"
if sys.argv[2] == "nonce":
    path = base / "google.go"
    old = 'p.Purpose != purpose || claims.Nonce == "" || subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(p.Nonce)) != 1'
    new = 'p.Purpose != purpose || (false && (claims.Nonce == "" || subtle.ConstantTimeCompare([]byte(claims.Nonce), []byte(p.Nonce)) != 1))'
elif sys.argv[2] == "reset-revocation":
    path = base / "challenges.go"
    old = 'UPDATE account_sessions SET revoked_at=clock_timestamp() WHERE person_id=$1 AND revoked_at IS NULL'
    new = 'UPDATE account_sessions SET revoked_at=clock_timestamp() WHERE false AND person_id=$1 AND revoked_at IS NULL'
elif sys.argv[2] == "otp-subject-cap":
    path = base / "challenges.go"
    old = 'if err = h.guessesLeft(ctx, tx, kind, subject); err != nil {'
    new = 'if err = h.guessesLeft(ctx, tx, kind, subject); false && err != nil {'
else:
    path = base / "sessions.go"
    old = '{"login-fail", pairKey(username, ip), 10, 15 * time.Minute}'
    new = '{"login-fail", username, 10, 15 * time.Minute}'
source = path.read_text()
assert source.count(old) == 1, "Mutation anchor drifted"
path.write_text(source.replace(old, new))
PY
  git -C "$mutant_tree" diff --binary >"$artifacts/$mutant.patch"
  if (cd "$mutant_tree" && tier) >"$artifacts/$mutant.log" 2>&1; then
    echo "Mutant $mutant vẫn xanh: $artifacts" >&2; exit 1
  fi
  case "$mutant" in
    nonce) expected=TestPostgresGoogleNonceNoEmailMergeAndSafeLink ;;
    reset-revocation) expected=TestPostgresAccountProofResetAndSessionRevocation ;;
    otp-subject-cap) expected=TestPostgresWrongCodesAreCappedPerEmailAcrossResends ;;
    login-pair) expected=TestPostgresLoginFailuresPauseTheGuesserNotTheOwner ;;
  esac
  grep -q "^--- FAIL: $expected " "$artifacts/$mutant.log" || {
    echo "Mutant $mutant đỏ sai bước: $artifacts" >&2; exit 1;
  }
  if grep -Eq 'build failed|undefined:|syntax error|SKIP' "$artifacts/$mutant.log"; then
    echo "Mutant $mutant không đo được hành vi: $artifacts" >&2; exit 1
  fi
  cleanup
done
echo "Identity xanh, canary và 4 mutant đỏ đúng bước, cùng SHA $sha. Bằng chứng: $artifacts"
