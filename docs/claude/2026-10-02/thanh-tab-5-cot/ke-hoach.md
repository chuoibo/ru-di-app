# Thanh tab 5 cột — kế hoạch triển khai

> **Cách thực thi (chủ sản phẩm chọn 02/10):** Native — làm tuần tự trong phiên này bằng superpowers:executing-plans,
> đánh dấu `- [ ]`; cuối nhánh một reviewer mới đọc cả nhánh. Mọi việc frontend đi qua skill **impeccable-pipeline**
> (mục «Pipeline Impeccable» bên dưới).

**Mục tiêu:** thanh tab còn 4 cột + con dấu «Tạo» đúng tâm; Cộng đồng thành mục thứ hai của Khám phá
(`Địa điểm | Cộng đồng`, mặc định Địa điểm); thân Địa điểm mặc lớp áo mockup; thẻ bài Cộng đồng có ảnh rộng hết cột
và nút Lưu 🔖 (giữ bài nằm trên nền giấy) — bám sát mockup ba bảng của chủ sản phẩm.

**Kiến trúc:** `community` vẫn là một route của `Tabs` (giữ URL `/community`, thanh tab, deep link) nhưng `href: null`
và không có cột; một module thuần `thanh-tab.ts` tính cột, cột sáng, ô con dấu, và mục Khám phá xem sau cùng. Hàng
tiêu đề `DauKhamPha` là component trình bày; route file (`app/(tabs)/explore.tsx`, `community.tsx`) giữ lệnh
`router.navigate` để bộ rút hướng dẫn Nếp đọc được cạnh. Bộ hướng dẫn (extractor + Go + test JS) học khái niệm
«route ẩn khỏi thanh, sống trong cột chủ».

**Stack:** Expo 57 / expo-router 57, React Native (+ react-native-web), TypeScript; Go 1.26 (`services/core/internal/huongdan`).

**Spec:** `docs/claude/2026-10-01/thanh-tab-5-cot/spec.md` (commit `93fed2a5`). Mockup duyệt: ba bảng ảnh của chủ sản
phẩm (`/tmp/claude-1000/-home-lakiet-project-mobile-ru-di-app/9a21d299-e1ae-4698-9ce7-5e1286796cef/images/{4,5,6}.png`
— Task 0 chép ra chỗ bền).

**Nơi làm:** worktree `/home/lakiet/project/mobile/wt-thanh-tab-5-cot`, nhánh `claude/thanh-tab-5-cot`. Cây gốc
`ru-di-app` đang có việc dở của phiên khác (B4 chia bill, `tsconfig.test.json`, `Profile.tsx`, `NepDock.tsx`,
`feature-plan.md`, `qa-handoff.md`) — **không đụng**.

## Context

Chủ sản phẩm chê thanh 6 cột (5 tab + con dấu): số cột chẵn nên «Tạo» không bao giờ ở giữa. Brainstorm đã chốt: gộp
Cộng đồng vào Khám phá (cả hai trả lời «đi đâu?»), giữ Lên plan và khay «Mình làm gì tiếp?», thân Địa điểm giữ bố
cục một thẻ lớn → hai quán so → hàng nhưng mặc áo mockup. Chủ sản phẩm vẽ mockup ba bảng («Bố cục đề xuất», «Tối &
responsive», «Thanh tab 5 cột»), duyệt spec và yêu cầu code «bám sát mockup và các thảo luận», dùng impeccable-pipeline.

## Bổ sung spec khi lập kế hoạch (Task 0 ghi vào spec)

Rà code và hỏi lại chủ sản phẩm ra 8 điểm spec chưa đúng hoặc chưa đủ:

1. **Chủ đề** không sống trên tab: `/community/topic` là route stack re-export `CommunityScreen`. Trang đó giữ đầu màn
   cũ (tiêu đề = chủ đề, cài đặt, nút đăng); không có hàng «Địa điểm | Cộng đồng». Bỏ ý «dòng #chủ đề ×».
2. **Nếp**: hai test Go (`duong_test.go` `tabLayout`, `TestTabRutBangTabLayout`, `TestTabKhopLayout`) và test JS
   `huong-dan-khop-ma` coi mọi route trong `app/(tabs)/` là một cột, cách nhau một chạm. Phải dạy chúng route ẩn.
3. **Maestro**: `26-kham-pha-that.yaml` (dòng 26, 58, 124) chờ `"[0-9]+ nơi ở Đà Lạt"`, `38-anh-dia-diem.yaml:33`
   chờ `"[0-9]+ nơi ở .*"`, subflow `_community.yaml:11` bấm «Đăng khoảnh khắc» (nút bị bỏ).
4. **Cỡ chữ 1.3** không chụp được trên web (react-native-web ghim `fontScale` = 1). Bằng chứng 320 × 1.3 phải từ
   Android emulator.
5. **Thẻ đầu** giữ dòng phụ sao/km/giờ (caption mờ) dưới mô tả: mockup vẽ dữ liệu mẫu ít trường hơn, bỏ thì thẻ đầu
   nghèo thông tin hơn thẻ so sánh bên dưới.
6. **Chi tiết mockup chưa có trong spec**: sân khấu thành phố tràn hết bề ngang ở điện thoại; «· đổi nơi khác ▾»
   màu coral, tên thành phố màu mực; ô tìm Cộng đồng là `SearchField` viền tròn như Khám phá; thanh dọc tablet có
   logo «Rủ Đi» trên con dấu.
7. **Màu nhãn «Tạo»**: chốt coral theo mockup («bám sát mockup»). Đóng mục «Còn mở».
8. **Thẻ bài Cộng đồng** (chủ sản phẩm chọn 02/10, sau khi hỏi ý kiến thẩm mỹ): ảnh/video rộng hết cột, album nhiều
   ảnh lật từng trang rộng hết cột với số «1/3»; nút Lưu 🔖 cuối hàng nút. **Không** bọc thẻ trắng (giữ hàng trên nền
   giấy, nét mảnh — `direction.md`); **không** dòng «· Đà Lạt» (bài không có trường địa điểm).

## Global Constraints

- Code comment/docstring tiếng Anh; tài liệu và commit message tiếng Việt, nói *cái gì đổi, vì sao, số đo cổng*.
- Không token mới (màu, cỡ chữ, radius) — dùng `typography.h2` cho chữ đầu Khám phá, `colors.accent` cho băng,
  băng 28×4 như thanh tab (`DESIGN.md:2146-2179`).
- Mục tiêu chạm ≥ 48dp. Không toast, modal lỗi, gradient, bóng nặng (`direction.md:116-121`).
- Tiền và giá không bao giờ bị cắt `…`; chật thì xuống dòng.
- Không thêm dòng chữ khẳng định điều dữ liệu không có: không «Gần bạn», không «đúng gu» (server `ORDER BY places.id`).
- Không reseed/wipe stack QA chung, không kill server cổng 58280 khi phiên khác đang chạy `run-b4-sach.sh`.
- Không đưa mockup (ảnh người do AI vẽ) vào Git. Ảnh bằng chứng vào Git phải pin trong `.repo-guard-allowlist.json`.
- Không push lên origin trừ khi người dùng bảo.

## Review Focus

1. **Chạm cột Khám phá khi đang ở Cộng đồng**: không làm gì (không nảy về Địa điểm); từ tab khác chạm Khám phá →
   mở lại mục xem sau cùng. → probe ở Task 8 (`thanh-tab-5.mjs`), logic ở test Task 1.
2. **Mở lạnh `/community`** (thông báo, link): cột Khám phá sáng, đầu màn chọn «Cộng đồng», con dấu mở khay với
   «Viết bài» đầu. → probe Task 8.
3. **Chưa đăng nhập ở mục Cộng đồng**: vẫn có hàng «Địa điểm | Cộng đồng» để quay lại, không bị kẹt. → probe Task 8.
4. **320dp × 1.3**: hàng tiêu đề không cắt chữ (cuộn ngang nếu tràn), nhãn tab xuống 2 dòng, con dấu vẫn tâm.
   → emulator Task 8, số đo vào commit.
5. **Tên dài / giá dài**: «Chỗ hay ở Bà Rịa – Vũng Tàu» xuống dòng; «200.000đ – 250.000đ mỗi người» không `…`.
   → test `the-dan` Task 5 + ảnh C2.

---

## Pipeline Impeccable (áp cho Task 0, 2–10)

- **Launcher**: wrapper `~/.claude/skills/impeccable-pipeline/scripts/imp` không tìm thấy bản 4.4.0 (cài ở
  `~/.claude/plugins/synced/*/impeccable~g2/skills/impeccable`). Dùng thẳng
  `IMP=$(ls -d ~/.claude/plugins/synced/*/impeccable~g2/skills/impeccable)/scripts/impeccable`.
  Chạy `"$IMP" context --target apps/mobile/src/rudi/screens/explore/ExploreLive.tsx` **một lần** từ gốc worktree.
  Nếu launcher lỗi: gửi một tin riêng «Context loading did not run; I'll read the existing project context directly.»
  rồi đọc `PRODUCT.md`, `DESIGN.md`, `.impeccable/config.json`, `.impeccable/surfaces/community.md`.
- **Định tuyến**: *Extension* trong thế giới đã có (giấy/mực/con dấu, `DESIGN.md`) — không concept tournament,
  không `concept-seed`. `buildPath: "code"` → code-led; mockup chủ sản phẩm là **decision comp** (tham chiếu duyệt).
  Mode: thanh tab = Operate; Khám phá = Experience nhẹ (`direction.md:105-114`).
- **Platform `adaptive`** (`PRODUCT.md:19-21`): detector Impeccable **không chạy** (chỉ đọc HTML/CSS); gói gửi
  reviewer ghi rõ «no detector ran» kèm `reference/ios.md`, `reference/android.md`. Đọc `reference/adapt.native.md`
  trước Task 8, `reference/audit.native.md` ở Task 10.
- **Direction contract** (Task 0) ghi ở `.impeccable/surfaces/kham-pha.md` mục `## Direction contract`: THESIS /
  OWN-WORLD / STORY / FIRST VIEWPORT / FORM, dòng `FINISH:` nguyên văn.
- **`reference/craft-floor.md`**: đọc ngay trước lần sửa UI đầu tiên (đầu Task 2), không đọc lúc lập kế hoạch.
- **Kiểm**: một vòng chụp gộp → sửa một mẻ → tối đa một vòng xác nhận (Task 8). Rồi finish reviewer ở ngữ cảnh mới,
  hành động theo đúng một trong bốn chữ `recapture | rebuild | fix | ship`, rồi documenter (Task 9).
- **Số đo**: critique (hai subagent tách biệt) trước (Task 0) và sau (Task 10), audit native /20, xem trend.

## Cấu trúc file

| File | Trách nhiệm |
|---|---|
| `apps/mobile/src/rudi/ui/thanh-tab.ts` (mới) | Thuần: cột nào có trên thanh, cột sáng, ô con dấu, mục Khám phá xem sau cùng |
| `apps/mobile/src/rudi/ui/DauKhamPha.tsx` (mới) | Hàng tiêu đề «Địa điểm \| Cộng đồng» + ô phải; chỉ trình bày, nhận `onDoiMuc` |
| `apps/mobile/src/rudi/kham-pha/the-dan.ts` (mới) | Thuần: tách giá và dòng phụ của thẻ đầu từ `facts` |
| `apps/mobile/app/(tabs)/_layout.tsx` | Thứ tự `explore` đầu, `community` `href: null` |
| `apps/mobile/app/(tabs)/explore.tsx`, `community.tsx` | Ghi mục khi focus, dựng `DauKhamPha`, giữ `router.navigate` |
| `apps/mobile/src/rudi/ui/RudiTabBar.tsx` | Dùng `xepThanh`, chạm cột → `dichCuaCot`, logo đầu thanh dọc |
| `apps/mobile/src/rudi/ui.tsx` | `Chip` thêm `vaiTab` |
| `apps/mobile/src/rudi/screens/explore/ExploreLive.tsx`, `screens/Discovery.tsx` | `header`, dòng điểm đến, sân khấu tràn, tiêu đề mục |
| `apps/mobile/src/rudi/screens/explore/HangDiaDiem.tsx` | `PlaceLead` thành thẻ theo mockup |
| `apps/mobile/src/rudi/community/CommunityScreen.tsx` | `dau`, chip-tab, `SearchField`, bỏ nút đăng ở tab |
| `apps/mobile/src/rudi/community/album.ts` (mới) | Thuần: trang album đang xem, nhãn «1/3» |
| `apps/mobile/src/rudi/community/PostCard.tsx` | ảnh/video rộng hết cột, số trang, nút Lưu 🔖 (`onSave` tuỳ chọn) |
| `apps/mobile/src/rudi/screens/Create.tsx` | dòng phụ «Viết bài» |
| `apps/mobile/tools/rut-huong-dan.mjs` | xuất `muc_trong_tab` vào `_rut.json` |
| `services/core/internal/huongdan/{nap.go,duong.go}` (+ tests) | route ẩn: không là tab, có cạnh tới các cột khác cột chủ |
| `apps/mobile/tests/huong-dan-khop-ma.test.mjs` | gương luật của Go |
| `services/core/internal/huongdan/data/{kham-pha,cong-dong}.md`, `_rut.json`, `apps/mobile/src/rudi/nep/huong-dan-ban.ts` | sổ tay + bản rút |
| `apps/mobile/.maestro/{26-kham-pha-that,38-anh-dia-diem,_community}.yaml` | chữ mới, lối viết bài mới |
| `apps/mobile/tsconfig.test.json` | thêm 3 module mới (có comment lý do) |
| `.impeccable/surfaces/kham-pha.md` (mới) | surface brief + direction contract |
| `docs/claude/2026-10-02/thanh-tab-5-cot/{ke-hoach.md,ket-qua.md,evidence/}` | kế hoạch này, kết quả, ảnh |

---

### Task 0: Chuẩn bị, mốc «trước», direction contract

**Files:** Create `.impeccable/surfaces/kham-pha.md`, `docs/claude/2026-10-02/thanh-tab-5-cot/ke-hoach.md`;
Modify `docs/claude/2026-10-01/thanh-tab-5-cot/spec.md`.

- [ ] Cài phụ thuộc thật (symlink làm hỏng `build:check` của Metro):
  `cd /home/lakiet/project/mobile/wt-thanh-tab-5-cot/apps/mobile && npm ci` (Node 22:
  `export PATH=$HOME/.nvm/versions/node/v22.23.2/bin:$PATH`).
- [ ] Mốc xanh trước khi sửa: `npm test` (ghi số test pass/fail), `npx tsc --noEmit`,
  `cd ../../services/core && go test -count=1 ./internal/huongdan`. Đỏ sẵn có thì ghi lại, không sửa lan.
- [ ] Chép mockup ra chỗ bền ngoài repo: `mkdir -p ~/.local/share/rudi-ux/thanh-tab-5/mockup` rồi copy `4.png` →
  `bo-cuc.png`, `5.png` → `toi-responsive.png`, `6.png` → `thanh-tab.png`. Cắt từng khung điện thoại bằng PIL thành
  `mockup/khung-01-dia-diem.png`, `khung-02-cong-dong.png`, `khung-03-khay.png`, `khung-toi.png`, `khung-320.png`,
  `khung-tablet.png` (để ghép so sánh ở Task 8).
- [ ] Chạy `"$IMP" context --target …` (mục Pipeline). Làm theo directive; không chạy lại.
- [ ] Ghi `.impeccable/surfaces/kham-pha.md` theo dạng `.impeccable/surfaces/community.md`, có `## Direction contract`:
  - THESIS: một tab trả lời «đi đâu?» bằng hai mục; con dấu «Tạo» ở đúng tâm; từ chối thanh 6 cột và băng cuộn ngang.
  - OWN-WORLD: giấy `ground`, mực `ink`, băng coral 28×4 dưới chữ đang mở, chữ Bricolage h2, chip viền `lineStrong`,
    thẻ đầu nền `card` viền `line`, tim tròn trên ảnh, chip AI tím `aiSoft`.
  - STORY: mở app → thấy thành phố và chỗ hay; chạm «Cộng đồng» → chuyện của người khác; «+» → việc hợp chỗ đang đứng.
  - FIRST VIEWPORT (390): hàng «Địa điểm Cộng đồng» trên cùng; dòng 📍 thành phố; sân khấu tràn ngang; ô tìm + ✨;
    chip loại; «Chỗ hay ở …» + «N nơi»; nửa trên thẻ đầu; thanh 5 cột, «+» tâm.
  - FORM: extension của thế giới đã có; tham chiếu = mockup chủ sản phẩm ba bảng.
  - Dòng cuối nguyên văn (`reference/new-work.md` §5): `FINISH: unreviewed and undocumented is unfinished; this build
    ends with the finish review, the verdict, DESIGN.md, and every shipping raster carrying its provenance`.
- [ ] Thêm vào spec mục **«Bổ sung khi lập kế hoạch (02/10)»** đúng 8 điểm ở trên; đóng mục «Còn mở» (coral); bỏ
  thẻ bài khỏi «Ngoài phạm vi» nếu có.
- [ ] Chép file kế hoạch này vào `docs/claude/2026-10-02/thanh-tab-5-cot/ke-hoach.md`.
- [ ] Mốc «trước» (web, chưa sửa code): `EXPO_PUBLIC_API_URL=http://127.0.0.1:58299 npx expo export --platform web
  --output-dir ~/.local/share/rudi-ux/web/tt5-truoc --clear`. Kiểm stack: `curl -s 127.0.0.1:58299/healthz`. Phục vụ
  ở **8081** (CORS cho phép; 58280 đang của phiên khác): `cd ~/.local/share/rudi-ux && node serve.mjs web/tt5-truoc 8081`.
  Chụp `/explore`, `/community` ở C1, C2, C6 (đăng nhập persona `dalat-0` qua `thu-vien/phien.mjs`; nếu stack
  unhealthy hoặc hết lượt OTP thì chụp bản demo chưa đăng nhập và ghi rõ) vào `out/tt5-truoc/`.
- [ ] Critique mốc (Pipeline Step 6): đọc `reference/critique.md`, chạy hai assessment ở hai subagent tách biệt trên
  ảnh `out/tt5-truoc/` cho surface Khám phá + Cộng đồng + thanh tab; đưa báo cáo vào chat trước khi lưu.
- [ ] Commit: `docs(thanh-tab): kế hoạch, bổ sung spec, surface brief Khám phá + direction contract`.

### Task 1: Logic thuần của thanh tab

**Files:** Create `apps/mobile/src/rudi/ui/thanh-tab.ts`, `apps/mobile/tests/thanh-tab.test.mjs`;
Modify `apps/mobile/tsconfig.test.json` (thêm `"src/rudi/ui/thanh-tab.ts"` kèm comment như các mục khác),
`apps/mobile/app/(tabs)/_layout.tsx`.

**Interfaces — Produces:**
`MUC_TRONG_TAB: Readonly<Record<string, string | undefined>>`, `type ThanhTab = { cot: string[]; viTriDau: number;
soCot: number; cotChon: number }`, `xepThanh(tenRoute: readonly string[], dangMo: string, rail: boolean): ThanhTab`,
`oCuaCot(thanh: ThanhTab, i: number): number`, `type MucKhamPha = "explore" | "community"`,
`ghiMucKhamPha(muc: MucKhamPha): void`, `dichCuaCot(cot: string): string`.

- [ ] **Viết test đỏ** `tests/thanh-tab.test.mjs`:

```js
import assert from "node:assert/strict";
import { readFileSync } from "node:fs";
import test from "node:test";

import { MUC_TRONG_TAB, dichCuaCot, ghiMucKhamPha, oCuaCot, xepThanh } from "../dist-test/rudi/ui/thanh-tab.js";

const TAB = ["explore", "community", "plan", "messages", "profile"];

test("cột Khám phá mở Địa điểm khi chưa xem mục nào (lần đầu chạy)", () => {
  assert.equal(dichCuaCot("explore"), "explore");
});

test("thanh: bốn cột + con dấu = năm ô, con dấu ở ô giữa", () => {
  const t = xepThanh(TAB, "plan", false);
  assert.deepEqual(t.cot, ["explore", "plan", "messages", "profile"]);
  assert.equal(t.soCot, 5);
  assert.equal(t.viTriDau, 2);
});

test("thanh: đang ở Cộng đồng thì cột Khám phá sáng", () => {
  assert.equal(xepThanh(TAB, "community", false).cotChon, 0);
  assert.equal(xepThanh(TAB, "explore", false).cotChon, 0);
  assert.equal(xepThanh(TAB, "messages", false).cotChon, 2);
  assert.equal(xepThanh(TAB, "khong-co", false).cotChon, -1);
});

test("thanh: ô của từng cột nhảy qua ô con dấu", () => {
  const t = xepThanh(TAB, "plan", false);
  assert.deepEqual([0, 1, 2, 3].map((i) => oCuaCot(t, i)), [0, 1, 3, 4]);
});

test("thanh dọc: con dấu ở đầu", () => {
  assert.equal(xepThanh(TAB, "plan", true).viTriDau, 0);
});

test("cột Khám phá mở lại mục xem sau cùng; cột khác mở chính nó", () => {
  ghiMucKhamPha("community");
  assert.equal(dichCuaCot("explore"), "community");
  assert.equal(dichCuaCot("plan"), "plan");
  ghiMucKhamPha("explore");
  assert.equal(dichCuaCot("explore"), "explore");
});

test("route href: null của _layout.tsx đúng bằng MUC_TRONG_TAB, cột chủ là một tab hiện", () => {
  const layout = readFileSync(new URL("../app/(tabs)/_layout.tsx", import.meta.url), "utf8");
  const tat = [...layout.matchAll(/<Tabs\.Screen\s+name="([^"]+)"\s+options=\{\{([^}]*)\}\}/g)];
  const an = tat.filter(([, , o]) => /href:\s*null/.test(o)).map(([, ten]) => ten).sort();
  assert.deepEqual(an, Object.keys(MUC_TRONG_TAB).sort());
  const hien = tat.filter(([, , o]) => !/href:\s*null/.test(o)).map(([, ten]) => ten);
  for (const chu of Object.values(MUC_TRONG_TAB)) assert.ok(hien.includes(chu), `${chu} không phải tab hiện`);
});
```

- [ ] Chạy: `npx tsc -p tsconfig.test.json; node tools/fixup-esm.mjs; node --test tests/thanh-tab.test.mjs` → đỏ
  (module chưa có).
- [ ] **Viết** `src/rudi/ui/thanh-tab.ts`:

```ts
/**
 * The tab strip's columns, worked out away from React so a test can hold them.
 *
 * Cộng đồng is a route of the tab navigator — it keeps the strip on screen,
 * its own URL and its deep links — but it has no column: it is the second
 * section of Khám phá, and while it is open the Khám phá column is the lit
 * one. `app/(tabs)/_layout.tsx` marks it `href: null`; this map names the
 * column that hosts it, and a test holds the two together.
 */
export const MUC_TRONG_TAB: Readonly<Record<string, string | undefined>> = { community: "explore" };

export type ThanhTab = {
  /** Route names that get a column, in layout order. */
  cot: string[];
  /** Slot of the «Tạo» stamp among the strip's slots; 0 on the rail. */
  viTriDau: number;
  /** Slots on the strip: the columns plus the stamp's. */
  soCot: number;
  /** Index into `cot` of the lit column; -1 when the open route has none. */
  cotChon: number;
};

export function xepThanh(tenRoute: readonly string[], dangMo: string, rail: boolean): ThanhTab {
  const cot = tenRoute.filter((ten) => MUC_TRONG_TAB[ten] === undefined);
  return {
    cot,
    // An odd slot count has a middle: four columns put the stamp third of five.
    viTriDau: rail ? 0 : Math.floor(cot.length / 2),
    soCot: cot.length + 1,
    cotChon: cot.indexOf(MUC_TRONG_TAB[dangMo] ?? dangMo),
  };
}

/** The strip slot of column `i`, counting the stamp's slot. */
export function oCuaCot(thanh: ThanhTab, i: number): number {
  return i >= thanh.viTriDau ? i + 1 : i;
}

/** Khám phá's two sections, by route name. */
export type MucKhamPha = "explore" | "community";

// Memory only: a fresh start of the app opens Địa điểm (owner's call, 01/10).
let mucCuoi: MucKhamPha = "explore";

/** Remember the section of Khám phá last in view. */
export function ghiMucKhamPha(muc: MucKhamPha): void {
  mucCuoi = muc;
}

/** The route a column opens: Khám phá reopens the section last in view. */
export function dichCuaCot(cot: string): string {
  return cot === "explore" ? mucCuoi : cot;
}
```

- [ ] Sửa `app/(tabs)/_layout.tsx`: thứ tự `explore`, `community`, `plan`, `messages`, `profile`; dòng community viết
  đúng `<Tabs.Screen name="community" options={{ href: null, title: "Cộng đồng" }} />` (các dòng khác giữ dạng
  `options={{ title: "…" }}` — regex Go đọc dạng đó). Sửa docblock: «Four columns and the «Tạo» stamp in the middle;
  Cộng đồng is a route with no column (Khám phá's second section, `thanh-tab.ts`)». Giữ đoạn `backBehavior="history"`.
- [ ] Chạy lại test → xanh. Chạy `npx tsc --noEmit` → sạch.
- [ ] **Đột biến** (ghi kết quả, hoàn nguyên): (a) bỏ `.filter` → đỏ ở «bốn cột + con dấu»; (b) `cotChon:
  cot.indexOf(dangMo)` → đỏ ở «đang ở Cộng đồng thì cột Khám phá sáng»; (c) `let mucCuoi = "community"` → đỏ ở
  «mở Địa điểm khi chưa xem mục nào».
- [ ] Commit: `feat(mobile): thanh tab tính cột thuần — Cộng đồng là route không cột, sáng cột Khám phá`.

### Task 2: RudiTabBar dùng `xepThanh`, logo đầu thanh dọc

**Files:** Modify `apps/mobile/src/rudi/ui/RudiTabBar.tsx`.
**Consumes:** `xepThanh`, `oCuaCot`, `dichCuaCot` (Task 1).

- [ ] Đọc `reference/craft-floor.md` (lần sửa UI đầu tiên).
- [ ] Thay khối tính cột (L58-63) và chỉ báo:

```tsx
const ten = state.routes.map((r) => r.name);
const tabDangMo = state.routes[state.index]?.name ?? "";
const thanh = xepThanh(ten, tabDangMo, layout.rail);
const { viTriDau, soCot: columns } = thanh;
// The indicator follows the lit column, not the route index: on Cộng đồng
// (a route with no column) the Khám phá column stays lit.
const viTriSang = Math.max(thanh.cotChon, 0);
const indicator = useSharedValue(viTriSang);
useEffect(() => {
  indicator.value = withTiming(viTriSang, motion.timing("standard"));
}, [viTriSang, indicator, motion]);
const indicatorStyle = useAnimatedStyle(() => {
  if (layout.rail) return { transform: [{ translateY: dauRail + HANG_LOGO_RAIL + HANG_DAU_RAIL + indicator.value * HANG_RAIL }] };
  const column = indicator.value >= viTriDau ? indicator.value + 1 : indicator.value;
  return { left: `${(column / columns) * 100}%` as const };
});
```

- [ ] `items` lặp trên `thanh.cot` (tìm `route` theo tên trong `state.routes`); `focused = i === thanh.cotChon`.
  `onPress`: `const dich = dichCuaCot(route.name); const dichRoute = state.routes.find((r) => r.name === dich) ?? route;`
  emit `tabPress` với `target: dichRoute.key`; nếu `!focused && !event.defaultPrevented` thì haptic +
  `navigation.navigate(dich)`. Đang sáng (kể cả đang ở Cộng đồng) thì không làm gì.
- [ ] Bỏ `community` khỏi `ICONS`. Chỗ splice ô rỗng dùng `viTriDau`; overlay con dấu dùng `viTriDau / columns`.
- [ ] Thanh dọc: thêm `const HANG_LOGO_RAIL = 56;` và ở đầu rail (trước danh sách, ngoài `tablist`) một
  `View style={{ height: HANG_LOGO_RAIL, alignItems: "center", justifyContent: "center" }}` chứa
  `<Wordmark color={colors.ink} height={18} />`; `danhSachRail` thêm `paddingTop: HANG_LOGO_RAIL`; overlay con dấu
  rail `top: dauRail + HANG_LOGO_RAIL`.
- [ ] Cập nhật docblock: bốn cột, con dấu ở ô giữa của năm ô; route không cột sáng cột chủ.
- [ ] `ConDauTao tab={tabDangMo}` giữ nguyên (Cộng đồng → `?tu=community` → «Viết bài» đầu khay). Nhãn «Tạo» giữ
  `colors.accent` (mockup).
- [ ] `npx tsc --noEmit`; `npm test` (đủ bộ, gồm `huong-dan-khop-ma` đọc `<ConDauTao `).
- [ ] Commit: `feat(mobile): thanh tab năm ô — bốn cột, «Tạo» đúng tâm; thanh dọc có logo trên con dấu`.

### Task 3: Hàng tiêu đề `DauKhamPha`

**Files:** Create `apps/mobile/src/rudi/ui/DauKhamPha.tsx`, `apps/mobile/tests/dau-kham-pha.test.mjs`;
Modify `apps/mobile/tsconfig.test.json` (thêm `"src/rudi/ui/DauKhamPha.tsx"`).
**Produces:** `DauKhamPha({ muc, onDoiMuc, phai }: { muc: MucKhamPha; onDoiMuc: (muc: MucKhamPha) => void; phai?: ReactNode })`
và `export type DungDau = (phai?: ReactNode) => ReactNode` (kiểu prop `dau` của `ExploreLiveScreen`, `ExploreScreen`,
`CommunityScreen` ở Task 4 và 6).

- [ ] **Test đỏ** (`react-dom/server` như `tests/o-nhap-muc.test.mjs`):

```js
import assert from "node:assert/strict";
import test from "node:test";
import React from "react";
import { renderToStaticMarkup } from "react-dom/server";
import { Text } from "react-native-web";

import { DauKhamPha } from "../dist-test/rudi/ui/DauKhamPha.js";

const ve = (props) => renderToStaticMarkup(React.createElement(DauKhamPha, { onDoiMuc: () => {}, ...props }));
const cacTab = (html) =>
  [...html.matchAll(/<[^>]*role="tab"[^>]*>/g)].map(([the]) => ({
    nhan: the.match(/aria-label="([^"]*)"/)?.[1],
    chon: the.match(/aria-selected="([^"]*)"/)?.[1],
  }));

test("đầu Khám phá: một tablist, hai tab theo thứ tự Địa điểm, Cộng đồng", () => {
  const html = ve({ muc: "explore" });
  assert.equal((html.match(/role="tablist"/g) ?? []).length, 1);
  assert.deepEqual(cacTab(html), [{ nhan: "Địa điểm", chon: "true" }, { nhan: "Cộng đồng", chon: "false" }]);
  assert.doesNotMatch(html, /aria-pressed/);
});

test("mục đang mở là Cộng đồng thì chỉ Cộng đồng được chọn", () => {
  assert.deepEqual(cacTab(ve({ muc: "community" })).map((t) => t.chon), ["false", "true"]);
});

test("ô phải nằm sau danh sách tab, không phải một tab", () => {
  const html = ve({ muc: "community", phai: React.createElement(Text, { testID: "o-phai" }, "x") });
  const viTriPhai = html.indexOf('data-testid="o-phai"');
  assert.ok(viTriPhai > html.lastIndexOf('role="tab"'));
  assert.equal(cacTab(html).length, 2);
});
```

- [ ] Chạy → đỏ. **Viết** component:

```tsx
import type { ReactNode } from "react";
import { Pressable, ScrollView, StyleSheet, Text, View } from "react-native";

import { TABLIST, tabState } from "../../ui/a11y";
import { typography, useRudiTheme } from "../theme";
import type { MucKhamPha } from "./thanh-tab";

/** How a Khám phá screen asks its route for the header, with its own right-hand control. */
export type DungDau = (phai?: ReactNode) => ReactNode;

const MUC: readonly { muc: MucKhamPha; nhan: string }[] = [
  { muc: "explore", nhan: "Địa điểm" },
  { muc: "community", nhan: "Cộng đồng" },
];

/**
 * Khám phá's header: its two sections as words at heading size, the open one
 * in ink over the strip's coral tape, the other faint; one font for both so
 * nothing shifts when the section changes. It only draws: the route file owns
 * the navigation (the guide's extractor reads a route's exits there). The
 * words scroll sideways rather than clip when a large text size outgrows a
 * narrow phone; `phai` (the feed settings, or the demo door) stays put.
 */
export function DauKhamPha({ muc, onDoiMuc, phai }: { muc: MucKhamPha; onDoiMuc: (muc: MucKhamPha) => void; phai?: ReactNode }) {
  const { colors } = useRudiTheme();
  return (
    <View style={styles.hang} testID="dau-kham-pha">
      <ScrollView contentContainerStyle={styles.cuonTrong} horizontal showsHorizontalScrollIndicator={false} style={styles.cuon}>
        <View {...TABLIST} style={styles.danhSach}>
          {MUC.map((m) => {
            const chon = m.muc === muc;
            return (
              <Pressable
                key={m.muc}
                {...tabState(chon)}
                accessibilityLabel={m.nhan}
                onPress={() => {
                  if (!chon) onDoiMuc(m.muc);
                }}
                style={styles.muc}
              >
                <Text numberOfLines={1} style={[typography.h2, { color: chon ? colors.ink : colors.inkFaint }]}>
                  {m.nhan}
                </Text>
                <View style={[styles.bang, { backgroundColor: chon ? colors.accent : "transparent" }]} />
              </Pressable>
            );
          })}
        </View>
      </ScrollView>
      {phai ? <View style={styles.phai}>{phai}</View> : null}
    </View>
  );
}

const styles = StyleSheet.create({
  hang: { flexDirection: "row", alignItems: "center", gap: 8 },
  cuon: { flexGrow: 1, flexShrink: 1 },
  cuonTrong: { flexGrow: 1 },
  danhSach: { flexDirection: "row", gap: 24 },
  muc: { minHeight: 48, alignItems: "center", justifyContent: "center", paddingTop: 6, gap: 4 },
  bang: { width: 28, height: 4, borderRadius: 2 },
  phai: { flexShrink: 0 },
});
```

- [ ] Test xanh; đột biến (d): đảo `chon ? colors.ink` không đủ — đột biến thật là `{...tabState(!chon)}` → đỏ ở cả
  hai test chọn. Hoàn nguyên.
- [ ] Commit: `feat(mobile): DauKhamPha — hàng «Địa điểm | Cộng đồng» cỡ h2, băng coral dưới mục đang mở`.

### Task 4: Mục Địa điểm (live + demo) và route Khám phá

**Files:** Modify `apps/mobile/app/(tabs)/explore.tsx`, `src/rudi/screens/explore/ExploreLive.tsx`,
`src/rudi/screens/Discovery.tsx`, `.maestro/26-kham-pha-that.yaml`, `.maestro/38-anh-dia-diem.yaml`.
**Consumes:** `DauKhamPha`, `ghiMucKhamPha`.

- [ ] Route file (import `useFocusEffect`, `useRouter` từ `expo-router`; `DauKhamPha`, `type DungDau` từ Task 3;
  `ghiMucKhamPha` từ Task 1):

```tsx
export default function ExploreTab() {
  useNepNguCanh({ /* unchanged */ });
  const router = useRouter();
  const { phien, phienDaDoc } = useRudiSession();
  useFocusEffect(useCallback(() => ghiMucKhamPha("explore"), []));
  // The section switch is a navigation written here, in the route file, so the
  // guide's extractor (tools/rut-huong-dan.mjs) sees explore -> community.
  const dau = (phai?: ReactNode) => <DauKhamPha muc="explore" onDoiMuc={() => router.navigate("/community")} phai={phai} />;
  if (!phienDaDoc) return null;
  if (phien !== null) return <ExploreLiveScreen dau={dau} phien={phien} />;
  return <ExploreScreen dau={dau} />;
}
```

- [ ] `ExploreLive.tsx`: chữ ký `{ phien, dau }: { phien: Phien; dau?: DungDau }`;
  `<RudiScreen bottomInset="tab" header={dau?.()} onRefresh={nap} testID="explore-screen">`; bỏ `Wordmark` khỏi
  `styles.dau` (bỏ import nếu không còn dùng).
  - Dòng điểm đến: tách chữ — tên thành phố `typography.label` màu `colors.ink`, rồi
    `<Text style={[typography.label, { color: colors.accent }]}> · đổi nơi khác</Text>`, chevron `colors.accent`.
    Giữ `accessibilityLabel="Đổi điểm đến"` và hai câu trạng thái «Chưa đọc được…»/«Đang đọc…» như cũ.
  - Sân khấu tràn ngang trên điện thoại: khi `!layout.rail` thêm `marginHorizontal: -16` cho `sanThanhPho` và đo
    `rongSan` từ chính view đó (đã có `onLayout`); `width={Math.min(rongSan, 480)}` giữ nguyên (390 → 390).
  - Tiêu đề mục: không lọc → `title={\`Chỗ hay ở ${diemDen?.name ?? "đây"}\`}` và ngay dưới
    `<Text style={[typography.caption, { color: colors.inkFaint, marginTop: -12 }]}>{\`${trang.places.length.toLocaleString("vi-VN")} nơi\`}</Text>`;
    đang lọc → giữ `«N kết quả»` + «Xóa lọc». **Không đổi** biểu thức `daChon ?? danhMuc.destination?.id`
    (`tests/mac-dinh-am-tham-id.test.mjs:228-233`).
- [ ] `Discovery.tsx` (`ExploreScreen({ dau })`): `header={dau?.(<DemoBadge />)}`; hàng dưới: nhãn vị trí bên trái,
  chuông «Thông báo» bên phải (giữ hộp thư demo); bỏ `Wordmark`; tiêu đề «Gần bạn, đúng gu» →
  `Chỗ hay ở Đà Lạt` + caption `${n} nơi` (giữ «Mẫu minh hoạ» khi có phiên, giữ «N kết quả phù hợp» khi lọc).
- [ ] Maestro: `26-kham-pha-that.yaml:26,58,124` `"[0-9]+ nơi ở Đà Lạt"` → `"Chỗ hay ở Đà Lạt"`;
  `38-anh-dia-diem.yaml:33` `"[0-9]+ nơi ở .*"` → `"Chỗ hay ở .*"`. Đọc ngữ cảnh từng dòng trước khi thay (dòng 58/124
  có thể là sau khi xoá lọc — vẫn đúng chữ mới).
- [ ] `npx tsc --noEmit`, `npm test`.
- [ ] Commit: `feat(mobile): Khám phá › Địa điểm — hàng tiêu đề chung, sân khấu tràn, «Chỗ hay ở …» thay «N nơi ở …»`.

### Task 5: Thẻ đầu theo mockup

**Files:** Create `apps/mobile/src/rudi/kham-pha/the-dan.ts`, `apps/mobile/tests/the-dan.test.mjs`;
Modify `apps/mobile/src/rudi/screens/explore/HangDiaDiem.tsx` (`PlaceLead`), `tsconfig.test.json`.
**Produces:** `tachTheDan(facts: readonly { icon: string; text: string }[]): { gia: string | null; phu: string }`.

- [ ] Xem icon của fact giá ở cả hai nguồn: live `chiTietNgan` (`src/rudi/kham-pha/dia-diem.ts:665-689`,
  `wallet-outline`) và demo `hienThiMau` (`Discovery.tsx:64-68`). Nếu demo dùng icon khác thì cho demo dùng
  `wallet-outline` (một quy ước), ghi lý do trong comment.
- [ ] **Test đỏ**:

```js
import assert from "node:assert/strict";
import test from "node:test";
import { tachTheDan } from "../dist-test/rudi/kham-pha/the-dan.js";

test("giá tách ra chip riêng, nguyên văn, không cắt", () => {
  const gia = "200.000đ – 250.000đ mỗi người";
  const r = tachTheDan([{ icon: "star", text: "4,8" }, { icon: "navigate", text: "1,2 km" }, { icon: "wallet-outline", text: gia }, { icon: "time-outline", text: "Mở tới 22:00" }]);
  assert.equal(r.gia, gia);
  assert.equal(r.phu, "4,8 · 1,2 km · Mở tới 22:00");
});

test("không có giá thì không có chip giá; không có gì thì dòng phụ rỗng", () => {
  assert.deepEqual(tachTheDan([{ icon: "star", text: "4,8" }]), { gia: null, phu: "4,8" });
  assert.deepEqual(tachTheDan([]), { gia: null, phu: "" });
});
```

- [ ] **Viết** `the-dan.ts`:

```ts
/**
 * The lead card's two lines under the name (owner's mockup, 01/10): the price
 * as its own chip, whole — money is never ellipsized — and every other fact
 * as one quiet line. The facts are the ones the compare cards and rows show,
 * so the lead never says less than the cards beneath it.
 */
export const ICON_GIA = "wallet-outline";

export function tachTheDan(facts: readonly { icon: string; text: string }[]): { gia: string | null; phu: string } {
  const gia = facts.find((f) => f.icon === ICON_GIA)?.text ?? null;
  const phu = facts.filter((f) => f.icon !== ICON_GIA).map((f) => f.text).join(" · ");
  return { gia, phu };
}
```

- [ ] `PlaceLead` thành thẻ (mockup khung 01):
  - Vỏ: `View` nền `colors.card`, `borderWidth: StyleSheet.hairlineWidth`, `borderColor: colors.line`,
    `borderRadius: radius.control`, `overflow: "hidden"`.
  - Trên: `MediaSlot` (giữ `ratio={tiLe}`, radius 0 vì vỏ đã bo) hoặc nhánh `KyHoa`; `Stamp` badge giữ góc trên trái.
  - Tim: anh em của `Pressable` mở quán (không lồng nút trong nút), `position: "absolute", top: 10, right: 10`;
    `IconButton` giữ 48×48, nền `card` (trắng trên ảnh như mockup). Bỏ `leadSave` dưới phải và `paddingRight: 56`.
  - Chữ (`padding: 14, gap: 6`): tên `typography.h2` (2 dòng); `sub` `typography.body`/`inkSoft` `numberOfLines={1}`;
    `phu` `typography.caption`/`inkFaint` (khi khác rỗng); hàng chip `flexDirection: "row", flexWrap: "wrap", gap: 8`:
    chip lý do (nếu `dd.lyDo`) nền `colors.aiSoft`, `sparkles` 13 + chữ `typography.label` màu `colors.ai`; chip giá
    nền `colors.ground`, `pricetag-outline` 14 + chữ `typography.label`/`inkSoft` **không** `numberOfLines`.
    Mỗi chip `borderRadius: radius.pill, paddingHorizontal: 10, paddingVertical: 6`.
  - `LyDo` dạng dòng chữ bị thay bằng chip ở thẻ đầu (giữ nguyên ở `PlaceCompare`/`PlaceRow`).
- [ ] Test xanh, `npx tsc --noEmit`, `npm test`.
- [ ] Commit: `feat(mobile): thẻ đầu Khám phá theo mockup — tim trên ảnh, chip lý do và chip giá nguyên văn`.

### Task 6: Mục Cộng đồng

**Files:** Modify `apps/mobile/app/(tabs)/community.tsx`, `src/rudi/community/CommunityScreen.tsx`,
`src/rudi/ui.tsx` (`Chip`), `.maestro/_community.yaml`.

- [ ] Route file:

```tsx
export default function CommunityTab() {
  const router = useRouter();
  useFocusEffect(useCallback(() => ghiMucKhamPha("community"), []));
  // In the route file on purpose: the guide's extractor reads community -> explore here.
  return <CommunityScreen dau={(phai) => <DauKhamPha muc="community" onDoiMuc={() => router.navigate("/explore")} phai={phai} />} />;
}
```

  `app/community/topic.tsx` giữ re-export (không `dau`).
- [ ] `Chip` thêm `vaiTab?: boolean`: khi true trải `tabState(selected)` thay cho thuộc tính nhấn hiện có, và không
  vẽ dấu tích. Nếu `src/rudi/ui.tsx` nằm trong build test (`grep '"src/rudi/ui.tsx"' tsconfig.test.json`) thì thêm ca
  render vào `tests/trang-thai-tro-nang.test.mjs`; không thì probe Task 8 đếm `role=tab` của hàng chip.
- [ ] `CommunityScreen({ dau }: { dau?: DungDau })`:
  - Chưa đăng nhập: `<RudiScreen header={dau?.()}>` + lời mời hiện có.
  - Đã đăng nhập: `<RudiScreen header={dau?.(<nút «Cài đặt bảng tin» options-outline 48×48 />)} scroll={false}
    padded={false} bottomInset={0} …>`.
  - Có `dau`: bỏ `headingRow` (chữ display, caption kết nối, nút «Đăng khoảnh khắc»). Không `dau` (trang chủ đề):
    giữ `headingRow` như cũ.
  - `ListHeaderComponent` theo thứ tự mockup: `SearchField` (giữ `accessibilityLabel="Tìm chủ đề"`, placeholder
    «Đi đâu, ăn gì, trải nghiệm gì?», submit → `/community/search?q=`) → hàng chip `ScrollView horizontal` chứa
    `<View {...TABLIST}>` ba `Chip vaiTab` (`Dành cho bạn · Đang theo dõi · Thịnh hành`, `motion.haptic.select()`)
    → tiêu đề «Bài đã lưu/Bài của tôi» khi `mode` là `saved`/`mine` → thẻ đồng ý → lỗi. Bỏ ô tìm gạch chân cũ và
    `styles.tabs`/`styles.tab` nếu không còn dùng.
  - Nút «Bảng tin có cập nhật»: dời từ overlay `top: 136` vào trong vùng nội dung (anh em của `FlatList`,
    `position: "absolute", top: 8, alignSelf: "center", zIndex: 2`) để luôn nằm ngay dưới hàng tiêu đề.
- [ ] `_community.yaml`: comment đầu → «Requires a real synthetic session and Khám phá › Cộng đồng open (tap
  «Cộng đồng» in the Khám phá header)»; thay `- tapOn: "Đăng khoảnh khắc"` bằng `- tapOn: "Tạo mới"` rồi
  `- tapOn: "Viết bài"`.
- [ ] `npx tsc --noEmit`, `npm test`.
- [ ] Commit: `feat(mobile): Khám phá › Cộng đồng — hàng tiêu đề chung, ba chip-tab, ô tìm như Địa điểm; viết bài qua «Tạo»`.

### Task 6b: Thẻ bài — ảnh rộng hết cột, số trang, nút Lưu 🔖

**Files:** Create `apps/mobile/src/rudi/community/album.ts`, `apps/mobile/tests/album-cong-dong.test.mjs`;
Modify `apps/mobile/src/rudi/community/PostCard.tsx`, `CommunityScreen.tsx`, `PostDetail.tsx`, `tsconfig.test.json`.
**Produces:** `trangAlbum(offsetX: number, rong: number, soTrang: number): number`,
`nhanTrang(i: number, soTrang: number): string | null`; `PostCard` prop mới `onSave?: () => void`.

- [ ] **Test đỏ**:

```js
import assert from "node:assert/strict";
import test from "node:test";
import { nhanTrang, trangAlbum } from "../dist-test/rudi/community/album.js";

test("trang album: làm tròn theo bề rộng trang, kẹp trong [0, n-1]", () => {
  assert.equal(trangAlbum(0, 350, 3), 0);
  assert.equal(trangAlbum(176, 350, 3), 1);
  assert.equal(trangAlbum(9999, 350, 3), 2);
  assert.equal(trangAlbum(-40, 350, 3), 0);
  assert.equal(trangAlbum(120, 0, 3), 0);
});

test("nhãn trang chỉ có khi album nhiều hơn một tấm", () => {
  assert.equal(nhanTrang(0, 1), null);
  assert.equal(nhanTrang(1, 3), "2/3");
});
```

- [ ] **Viết** `album.ts`:

```ts
/**
 * A post's album pages one full-width picture at a time (owner's mockup,
 * 02/10). The page in view comes from the scroll offset, which the web build
 * reports through onScroll and native through the momentum end alike.
 */
export function trangAlbum(offsetX: number, rong: number, soTrang: number): number {
  if (rong <= 0 || soTrang <= 0) return 0;
  return Math.min(soTrang - 1, Math.max(0, Math.round(offsetX / rong)));
}

/** «2/3» over the picture; nothing for a single picture. */
export function nhanTrang(i: number, soTrang: number): string | null {
  return soTrang > 1 ? `${i + 1}/${soTrang}` : null;
}
```

- [ ] `PostCard`:
  - Album: `onLayout` trên khung bọc lấy `rong` (bề rộng cột trong padding 20 của bài); mỗi `mediaFrame`
    `width: rong`, ảnh/video `height: Math.round(rong * 3 / 4)` (tỉ lệ khung mockup); `ScrollView horizontal
    pagingEnabled`, `contentContainerStyle` không `gap`, bỏ `album: { marginHorizontal: -4 }`; `mediaFrame` giữ
    `borderRadius: 14`. Trước khi đo (`rong === 0`) không vẽ album (tránh nháy khung 0).
  - Số trang: `onScroll` (`scrollEventThrottle={16}`) + `onMomentumScrollEnd` → `setTrang(trangAlbum(x, rong, n))`;
    khi `nhanTrang` khác null vẽ viên nhỏ góc trên phải ảnh: nền `colors.card`, chữ `typography.caption`/`colors.ink`,
    `borderRadius: radius.pill`, `paddingHorizontal: 8, paddingVertical: 3`, `pointerEvents="none"`,
    `accessibilityElementsHidden` (trình đọc màn hình đã nghe «Mở ảnh khoảnh khắc» từng tấm).
  - `BookView` (nhật ký) không đổi.
  - Hàng nút: `Action` thích / bình luận / chia sẻ giữ nguyên; khi có `onSave` thêm cuối hàng (`marginLeft: "auto"`)
    một `PressScale` 48×48 chỉ icon `bookmark`/`bookmark-outline` (màu `colors.accent` khi đã lưu, `inkSoft` khi
    chưa), `accessibilityLabel={post.saved ? "Bỏ lưu bài" : "Lưu để đọc lại"}`, `{...giuState(post.saved)}`,
    `disabled={busy}`.
- [ ] `CommunityScreen` truyền `onSave={() => void act(async () => { await feedback(person, item.id, "saved",
  !item.saved); update({ ...item, saved: !item.saved }); })}`; ở chế độ `saved` bỏ lưu thì lọc bài khỏi danh sách.
  `PostDetail` truyền `onSave` dùng đúng lệnh trong sheet của nó (`feedback(...); await load()`). Mục «Lưu để đọc
  lại»/«Lưu bài» trong các sheet giữ nguyên.
- [ ] Test xanh, `npx tsc --noEmit`, `npm test`. Đột biến (f): `Math.round` → `Math.floor` → đỏ ở `176 → 1`.
- [ ] Commit: `feat(mobile): thẻ bài Cộng đồng — ảnh rộng hết cột lật từng trang có số, nút Lưu ngay trên hàng nút`.

### Task 7: Khay Tạo + hướng dẫn Nếp hiểu «route không cột»

**Files:** Modify `apps/mobile/src/rudi/screens/Create.tsx`, `apps/mobile/src/rudi/tao-moi.ts` (comment),
`apps/mobile/tools/rut-huong-dan.mjs`, `services/core/internal/huongdan/{nap.go,duong.go,duong_test.go,nap_test.go}`,
`apps/mobile/tests/huong-dan-khop-ma.test.mjs`, `services/core/internal/huongdan/data/{kham-pha,cong-dong}.md`;
regenerate `data/_rut.json` + `apps/mobile/src/rudi/nep/huong-dan-ban.ts`.

- [ ] `Create.tsx`: «Viết bài» `detail: "Kể một điều hay với cộng đồng"`. `tao-moi.ts`: comment nói Cộng đồng là mục
  của Khám phá (giữ tên route `community` trong `TabTao`).
- [ ] **Go test đỏ trước** (`duong_test.go`):
  - `tabLayout`: đọc mọi `<Tabs.Screen name="x" options={{ … }} />`, lấy `title`, **bỏ** những cái có `href: null`;
    vẫn đòi ≥ 4.
  - Test mới:

```go
// Cộng đồng is Khám phá's second section: a tab route with no column. From
// another tab it is two taps (the Khám phá column, then «Cộng đồng»); from
// Cộng đồng the strip is still on screen, so the other columns are one tap,
// and Khám phá itself is «Địa điểm» in the header, never the lit column.
func TestCongDongLaMucCuaKhamPha(t *testing.T) {
	if soTay.banDo.tab["community"] {
		t.Fatal("community counted as a column of the strip")
	}
	want := map[[2]string][]string{
		{"plan", "community"}:    {"Khám phá", "Cộng đồng"},
		{"explore", "community"}: {"Cộng đồng"},
		{"community", "explore"}: {"Địa điểm"},
		{"community", "plan"}:    {"Lên plan"},
	}
	for cap, nhan := range want {
		got, ok := soTay.duongToi(cap[0], cap[1])
		if !ok || len(got) != len(nhan) {
			t.Errorf("%s -> %s: %v, want %v", cap[0], cap[1], buoc(got), nhan)
			continue
		}
		for i := range nhan {
			if got[i].Nhan != nhan[i] {
				t.Errorf("%s -> %s step %d: %q, want %q", cap[0], cap[1], i, got[i].Nhan, nhan[i])
			}
		}
	}
}
```

  - `TestTabRutBangTabLayout`: thêm đầu dò — xoá `community` khỏi `muc_trong_tab` của bản sao → `lechTab` phải là
    `[+community]`.
- [ ] Chạy `cd services/core && go test -count=1 ./internal/huongdan` → đỏ đúng chỗ dự đoán.
- [ ] Extractor: đọc `app/(tabs)/_layout.tsx` lấy tên có `href: null`; đọc `MUC_TRONG_TAB` từ
  `src/rudi/ui/thanh-tab.ts` bằng AST `ts` (object literal); ném lỗi nếu hai tập khác nhau; ghi
  `{"muc_trong_tab": {"community": "explore"}, "routes": [...]}` (khoá sắp xếp như phần còn lại). Cập nhật docblock.
- [ ] Go `nap.go`: `banRut` thêm `MucTrongTab map[string]string \`json:"muc_trong_tab"\``; `docRut` kiểm mỗi khoá là
  route có `tep` trong `app/(tabs)/`, mỗi giá trị là route có `tep` trong `app/(tabs)/` và không phải khoá; khi dựng
  `s.tab`/`bd.tab` bỏ các khoá; lưu `s.mucTrongTab`/`bd.chu`. `laCanhMa(tu, den)` thêm nhánh
  `bd.chu[tu] != "" && bd.tab[den] && den != bd.chu[tu]` (thanh vẫn hiện trên route không cột). Cập nhật comment
  `canhNgoaiRut`/`laCanhMa`.
- [ ] Go `duong.go` `dungDoThi`: sau vòng tab×tab, với mỗi route không cột `h` (chủ `H`), với mỗi tab `b != H`:
  `them(h, b, tieuDe(b))`. Sửa comment nguồn (2).
- [ ] Gương JS `huong-dan-khop-ma.test.mjs`: `NGU_CANH.tab` bỏ khoá `muc_trong_tab`; thêm `chu`; `laCanhMa` thêm
  nhánh như Go; thêm assert `_rut.json` có `muc_trong_tab` khớp `MUC_TRONG_TAB`.
- [ ] Sổ tay:
  - `kham-pha.md`: `nhanUI` thêm `"Địa điểm"`, `"Cộng đồng"`, `"Chỗ hay ở …"` (chỉ nếu đúng literal rút được —
    kiểm `_rut.json`); bỏ mục nào không còn literal (vd. «… nơi ở …» nếu có). `di_toi` thêm
    `{"nhan": "Cộng đồng", "man": "community"}`. Thân: «Tab «Khám phá» có hai mục ở đầu màn: «Địa điểm» (danh mục
    quán…) và «Cộng đồng» (chuyện mọi người kể).» + một mục `## Sang Cộng đồng`.
  - `cong-dong.md`: `nhanUI` thêm `"Địa điểm"`, `"Tạo mới"`, `"Viết bài"`; `di_toi`
    `[{"nhan": "Địa điểm", "man": "explore"}]`. Thân: ««Cộng đồng» là mục thứ hai của tab Khám phá: mở Khám phá,
    chạm «Cộng đồng» ở đầu màn.»; bước viết bài → «Chạm con dấu «Tạo mới» giữa thanh tab rồi chọn «Viết bài».»
- [ ] Rút lại: `cd apps/mobile && node tools/rut-huong-dan.mjs` rồi `node tools/rut-huong-dan.mjs --check`.
- [ ] Xanh: `go test -count=1 ./internal/huongdan`, `gofmt -l .` rỗng, `go vet ./...`, `npm test`
  (`huong-dan-khop-ma`). Đột biến (e): bỏ nhánh loại khoá khỏi `s.tab` → `TestTabRutBangTabLayout` đỏ `+community`.
- [ ] Commit: `feat(huongdan): Nếp biết Cộng đồng là mục của Khám phá — route không cột, hai chạm từ tab khác`.

### Task 8: Vòng kiểm hình (Impeccable Step 4) — tối đa hai vòng

- [ ] Đọc `reference/adapt.native.md`. Export «sau»: `npx expo export --platform web --output-dir
  ~/.local/share/rudi-ux/web/tt5-sau --clear` (`EXPO_PUBLIC_API_URL=http://127.0.0.1:58299`), phục vụ cổng 8081.
- [ ] Probe mới ngoài repo `~/.local/share/rudi-ux/qa/tests/qa/mobile-ui-audit/kiem-ux/thanh-tab-5.mjs` (theo mẫu
  `spike-dau.mjs`): ở C1/C2/C5, đo tâm `[data-testid="con-dau-tao"]` − tâm viewport (dp); đếm `[role=tab]` của
  `tablist` thanh = 4, đúng một `aria-selected`; trên `/community` tab được chọn là «Khám phá», `dau-kham-pha` chọn
  «Cộng đồng», hàng chip có 3 `role=tab`, không còn «Đăng khoảnh khắc»; trên `/explore` có «Chỗ hay ở», tim nằm
  trong khung ảnh thẻ đầu; Review Focus 1–3 (chạm Khám phá khi ở Cộng đồng giữ URL; `/explore` → «Cộng đồng» →
  «Lên plan» → «Khám phá» = `/community`; mở lạnh `/community`; chưa đăng nhập ở `/community` có `dau-kham-pha`);
  bấm con dấu từ `/community` → thẻ đầu «Viết bài», thẻ hai «Tạo cuộc hẹn»; thẻ bài có ảnh: bề rộng khung ảnh =
  bề rộng cột trong padding (±1dp), có nút «Lưu để đọc lại»/«Bỏ lưu bài» với `aria-pressed`, album nhiều ảnh có
  nhãn «1/n» và cuộn sang trang 2 thì thành «2/n». Sao `vach-rail.mjs` thành bản 4 tab (C6/C7), đòi `trung 8/8`.
- [ ] Chụp gộp một vòng (đã tắt/đợi xong chuyển động vào): C1 sáng `/explore`, `/community`, khay từ `/community`;
  390×844 tối (cấu hình tự viết `colorScheme: "dark"`); C2 320; 1024×768 sáng (rail). Android emulator: `adb -s
  emulator-5554 shell cmd uimode night yes` / `settings put system font_scale 1.3` / `wm size 720x1280` +
  `wm density 360` (320dp) → chụp `/explore`, `/community`; hoàn nguyên (`night no`, `font_scale 1.0`,
  `wm size reset`, `wm density reset`). Không có emulator → ghi rõ «320 × 1.3 chưa đo» và ước tính bề rộng hàng tiêu
  đề từ đo web × 1.3.
- [ ] Mở từng ảnh xem đúng tên, không trống/đen/nửa tải. Ghép so sánh mockup ↔ bản dựng bằng
  `python3 ~/.local/share/rudi-ux/ghep.py` vào `~/.local/share/rudi-ux/out/tt5-sau/so-sanh-*.jpg` (ngoài Git).
- [ ] Phê bình bản dựng so với mockup + direction contract; sửa mọi chỗ lệch vật chất **một mẻ**; dựng lại; chụp lại
  đúng các viewport đó (vòng 2). Hết vòng 2 thì dừng đánh bóng.
- [ ] Chép ảnh vào `.impeccable/review/` (gitignore): `phone.png` (C1 Địa điểm), `phone-cong-dong.png`,
  `phone-khay.png`, `phone-dark.png`, `phone-320.png`, `tablet.png`, `android-320-1.3.png` (nếu có).
- [ ] Commit các sửa của vòng (nếu có): `fix(mobile): vòng kiểm hình thanh tab 5 cột — <điều đã sửa>`.

### Task 9: Finish reviewer + documenter (Impeccable Step 5)

- [ ] Spawn `impeccable-finish-reviewer` (ngữ cảnh mới, không fork). Gói vào: yêu cầu gốc (trích nguyên câu chủ sản
  phẩm), các câu trả lời đã chốt (mặc định Địa điểm, khay giữ nguyên, giữ bố cục thân, «Chỗ hay ở», coral cho «Tạo»,
  thẻ bài ảnh tràn + 🔖 trên nền giấy, 8 điểm bổ sung), đường dẫn mã đã sửa, đường dẫn ảnh `.impeccable/review/*`, direction contract
  (`.impeccable/surfaces/kham-pha.md`), comp tham chiếu = các khung mockup trong `~/.local/share/rudi-ux/thanh-tab-5/mockup/`
  (ghi rõ: decision comp của build code-led), `reference/craft-floor.md`, `reference/ios.md`, `reference/android.md`,
  và dòng «no detector ran: platform adaptive; iOS not captured (Linux host)». Đợi một lần với timeout dài.
- [ ] Theo chữ trả về: `recapture` → chụp lại rồi review đầy đủ; `rebuild` → làm lại vùng nêu tên, review đầy đủ;
  `fix` → sửa một mẻ, chụp lại cùng viewport, gửi lại cùng reviewer chấm từng mục resolved/partial/unresolved;
  `ship` → sang documenter.
- [ ] Spawn `impeccable-documenter`: cập nhật `DESIGN.md` mục thanh tab (`:2146-2179`: bốn cột + con dấu ô giữa; route
  không cột sáng cột chủ; thanh dọc có logo) và thêm thành phần «Hàng tiêu đề hai mục» + «Chip làm tab» + «Thẻ đầu»
  + «Thẻ bài: ảnh rộng hết cột, số trang, nút Lưu»;
  `.impeccable/design.json` nếu cần. Ranh giới ghi: chỉ `DESIGN.md`, `.impeccable/design.json`.
- [ ] Commit: `docs(design): thanh tab 5 cột, hàng tiêu đề hai mục, chip-tab, thẻ đầu — theo bản dựng đã review`.

### Task 10: Số đo đóng vòng (Impeccable Step 6)

- [ ] Critique sau (hai subagent tách biệt) trên ảnh Task 8 vòng 2; báo cáo vào chat trước khi lưu; so trend với
  mốc Task 0.
- [ ] Audit native (`reference/audit.native.md`) trên các file đã sửa: điểm /20 + P0–P3. P0/P1 nào chạm phạm vi đợt
  này thì sửa trong một mẻ (Task 8 đã dùng hết vòng → sửa không đổi bố cục, chụp lại một ảnh xác nhận).

### Task 11: Cổng, kết quả, đưa vào main

- [ ] Cây sạch đúng SHA (worktree tạm từ HEAD nhánh): mobile `npm ci && npx tsc --noEmit && MOBILE_REQUIRE_WEB_A11Y=1
  npm test && EXPO_NO_TELEMETRY=1 npx expo export --platform all --output-dir ~/.local/share/rudi-ux/web/tt5-all`; core
  `gofmt -l .` rỗng, `go vet ./...`, `go test -count=1 ./...`; `python3 scripts/check_screens_reachable.py
  --selftest && python3 scripts/check_screens_reachable.py`; `scripts/eval_kich_ban.sh`;
  `python3 scripts/repo_guard.py range <base> HEAD`.
- [ ] Canary: chạy test thanh-tab trên bản «trước» (checkout `_layout.tsx`/`RudiTabBar.tsx` của `bc8dbdfb` vào cây tạm)
  → đỏ đúng ca «bốn cột»; identity xanh.
- [ ] Ảnh bằng chứng vào Git (chỉ ảnh bản dựng, dữ liệu tổng hợp, không mockup): ghép
  `docs/claude/2026-10-02/thanh-tab-5-cot/evidence/EV-TT5-{THANH,KHAM-PHA,CONG-DONG,TOI-320-TABLET}.jpg`, pin từng
  file (`path`, `sha256`, `rules`, `reason`) vào `.repo-guard-allowlist.json`.
- [ ] `docs/claude/2026-10-02/thanh-tab-5-cot/ket-qua.md`: SHA, số đo (tâm con dấu ±dp ở C1/C2/C5, vạch rail 8/8,
  bề rộng hàng tiêu đề vs chỗ trống ở 320 × 1.3, bề rộng ảnh thẻ bài vs cột), đột biến (a)–(f) đỏ đúng chỗ, verdict reviewer, critique trước/sau,
  audit, cái còn mở (badge Tin nhắn, «+»→«×» B11, khay theo giai đoạn chuyến, iOS chưa chụp).
- [ ] Đưa vào main theo memory «Merge vào main khi cây gốc bận»: `git fetch`; rebase nhánh lên `main` mới nhất trong
  worktree (xử lý xung đột `tsconfig.test.json` nếu phiên kia đã commit); chạy lại cổng; ở cây gốc chụp
  `git diff --cached --name-status` + `git status --porcelain` trước/sau `git merge --ff-only claude/thanh-tab-5-cot`
  và so. Báo phiên kia (`ListAgents` → `SendMessage`) rằng main đã đổi và mục «Con dấu Tạo mới» trong
  `feature-plan.md` có thể chuyển READY_FOR_QA (họ đang giữ file đó). Không push.

---

## Kiểm chứng đầu-cuối (tóm tắt)

- `cd apps/mobile && npm test` (gồm `thanh-tab`, `dau-kham-pha`, `the-dan`, `album-cong-dong`, `huong-dan-khop-ma`) · `npx tsc --noEmit`
  · `expo export --platform all`.
- `cd services/core && go test -count=1 ./...` (gồm `TestCongDongLaMucCuaKhamPha`, `TestTabKhopLayout`,
  `TestTabRutBangTabLayout`, `TestCanhNgoaiRut`) · `gofmt -l .` · `go vet ./...` · `scripts/eval_kich_ban.sh`.
- Probe `kiem-ux/thanh-tab-5.mjs` + `vach-rail` bản 4 tab trên web export cổng 8081; Android emulator cho 320 × 1.3.
- Ảnh mở ra nhìn, so mockup từng khung; finish reviewer `ship`; documenter xong; critique trước/sau + audit.
