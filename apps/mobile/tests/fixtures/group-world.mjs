import { readFileSync } from "node:fs";
/**
 * The seeded people of the end-to-end harness. TEST DATA ONLY.
 *
 * No screen imports this file, so it is not in the app bundle; the RuDi app
 * has no demo story (2026-10-03: production launch, every screen reads the
 * signed-in person's own data or shows the sign-in door). `tests/e2e/*` use
 * it to reach the group `scripts/seed_demo_data.py` builds on a throwaway
 * stack, and `scripts/e2e_demo_people.py` reads it there.
 *
 * `personId` is the row in the seeded database. Copied values in two files
 * drift, so they are not trusted to match by care:
 * `tests/test_demo_identity_matches_seed.py` re-derives every one of them from
 * `scripts/seed_demo_data.py` and fails if a single character moves. These
 * derive from `uuid5` and carry no long digit run the repo guard would block.
 */
import { khoiDongNhom } from "../../dist-test/screens/chat/nhom.js";
export let DEMO_GROUP_NAME = "Team Đà Lạt";
export const DEMO_PEOPLE = [
    { id: "minh", personId: "46b55e67-932b-5415-a5ee-08fb2641a4ff", name: "Minh", initials: "M" },
    { id: "trang", personId: "49871dab-3bf9-5140-acf3-6c9736b31e8f", name: "Trang", initials: "Tr" },
    { id: "hai", personId: "be2389f9-62cb-5b28-8e5f-874768e9fb75", name: "Hải", initials: "H" },
    { id: "ngoc", personId: "e3a44e25-4547-508a-8f4d-9b2495c3325f", name: "Ngọc", initials: "Ng" },
    { id: "duc", personId: "4421b3f8-26a6-5827-a7e7-548c5a4a10f9", name: "Đức", initials: "Đ" },
    { id: "linh", personId: "cdadf49b-b6a8-5631-8b9d-aee6a7d532de", name: "Linh", initials: "L" },
    { id: "quan", personId: "93c153f7-042a-556d-b227-7b1e54f2d50b", name: "Quân", initials: "Q" },
];
// Isolated e2e uses UUIDs returned by real account registration. Unit oracles
// retain the historical ids; none of this file enters the app bundle.
if (process.env.RUDI_TEST_ACCOUNT_WORLD) {
 DEMO_GROUP_NAME="Nhóm QA tài khoản";
 const world=JSON.parse(readFileSync(process.env.RUDI_TEST_ACCOUNT_WORLD,"utf8"));
 for (const person of DEMO_PEOPLE) {
  if (!world[person.id]?.person_id) throw new Error("incomplete synthetic account world");
  person.personId=world[person.id].person_id;
  person.name="QA "+person.id;
  person.initials="QA";
 }
}

export function personById(id) {
    return DEMO_PEOPLE.find((p) => p.id === id) ?? null;
}
/** The seeded group, as `minh` made it, opened under `nguoi`. */
export function khoiDongNhomDemo(nguoi, opts = {}) {
    return khoiDongNhom(nguoi, { ...opts, chuNhom: personById("minh"), tenNhom: DEMO_GROUP_NAME });
}
