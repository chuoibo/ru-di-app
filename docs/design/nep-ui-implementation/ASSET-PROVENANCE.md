# Provenance asset Nếp đang dùng trong runtime

Ghi nhận tại checkout tiếp tục `/tmp/rudi-nep-ui-final-20261006`, nhánh `nep-ui-final-20261006` ngày 06/10/2026; packet execution cũ được giữ để đối chiếu. Base nghiệp vụ/repo `811c893fa76ab0d3f5583f18691c117c41439465`; đây là checksum file hiện hữu, không phải commit triển khai sạch hoặc chứng nhận gate ở exact SHA.

## Nguồn bàn giao bất biến

Bộ production asset v1.0.0 ngày 05/10/2026 ở [production-assets](</home/lakiet/project/mobile/ru-di-app/docs/design/2026-10-05-nep-full-audit/production-assets/README.md>); manifest ghim source HEAD `1f8f77a13f834fda2e34ea55281f65bb4a8b06c9`. Đã tính lại **686/686 checksum khớp, 0 sai/mất file** theo [checksums.sha256](</home/lakiet/project/mobile/ru-di-app/docs/design/2026-10-05-nep-full-audit/production-assets/checksums.sha256>). Không chạy builder, retouch, recolour hoặc sửa handoff.

- SHA-256 manifest: `cea5c74bbddb06a85fa3b032bd61841cf21c5d0f7ff9e49f4981537e1212842a`.
- SHA-256 danh sách checksums: `ec480b6f5602038b1be0aabc6091c6f3b4f0093426c782813bd22e4332b7514e`.
- [ART-DIRECTION.md](</home/lakiet/project/mobile/ru-di-app/docs/design/2026-10-05-nep-full-audit/production-assets/source/ART-DIRECTION.md>) ghi 12 cảnh/spot/decor được sinh bằng OpenAI image generation trong phiên design từ 10 reference boards, sau đó accepted-design-v 1; không chứa UI/text/logo/Nếp baked. 61 icon và 11 vật thể giấy là vector do builder nội bộ dựng, có geometry/roles; không lấy từ thư viện icon ngoài.
- [QC kỹ thuật](</home/lakiet/project/mobile/ru-di-app/docs/design/2026-10-05-nep-full-audit/production-assets/qc/pack-verification.json>) ghi master WebP cùng pixel RGBA với PNG và alpha bounds. Đây là QC handoff lịch sử của tác giả, không thay visual review màn thật hoặc kiểm chứng độc lập. Bản 384/768 đã resample; không tuyên bố giống từng pixel với master.
- [Hướng dẫn tích hợp](</home/lakiet/project/mobile/ru-di-app/docs/design/2026-10-05-nep-full-audit/production-assets/HUONG-DAN-SU-DUNG.md>) và [manifest.json](</home/lakiet/project/mobile/ru-di-app/docs/design/2026-10-05-nep-full-audit/production-assets/manifest.json>) là provenance/mapping; theme/token source hiện tại quyết định màu UI.

## Những file được ship

Runtime có **36 WebP = 12 thiết kế × 3 resolution**, tổng **29.806.924 byte** (28,426 MiB). Tất cả 36 file khớp byte/SHA-256 với cùng relative path trong handoff. Tổng theo variant: small/384 **1.589.886 byte**, medium/768 **6.034.952 byte**, master **22.182.086 byte**. Có 27 file `illustrations/` và 9 file `decor/`; registry vẫn require cả ba variant nên cả 36 có trong web export, dù một slot chỉ render variant đã chọn.

Không ship reference boards, composition/QC, master PNG, pose Nếp static, snapshot font/palette hoặc source builder vào asset-pack runtime. Asset artwork không đóng vai ảnh venue, ảnh người, ảnh bill hay memory do người dùng upload; các media đó tiếp tục đọc từ API/bearer/cache adapter hiện có.

| File dưới `apps/mobile/src/rudi/asset-pack/` | Byte | SHA-256 | So với handoff |
|---|---:|---|---|
| `decor/decor-branch-384.webp` | 56656 | `85c20425bc13828baec40f8524693cd2d0a54079d7c4ea9f5620aa70a3fe0dcc` | Khớp |
| `decor/decor-branch-768.webp` | 185394 | `eaa989715f900fe3684a5f2cf3893e82048638bc7a5550d4af0eb58e89b5d457` | Khớp |
| `decor/decor-branch.webp` | 1120914 | `63dd2eea0b05b7c07d0bfd6104445341a732741fe28f52268060dae92545d599` | Khớp |
| `decor/decor-cover-384.webp` | 224074 | `3a22407605fbcb3075f95ef4486b7343e028a744c160ec53a05003bd49b47366` | Khớp |
| `decor/decor-cover-768.webp` | 991554 | `8e932b116e9119749b9c794ca399e09b4684e6a918fa1aefc8ab123bb70668f0` | Khớp |
| `decor/decor-cover.webp` | 1883164 | `e37c3676a29332304d482f3b984726f7431a1ef4a4e18f98d58e12f126a2255a` | Khớp |
| `decor/decor-pines-384.webp` | 74102 | `b698360464898beba14ec1513aa784335221c6980ef295c2ac12b5b1ed8d8ea7` | Khớp |
| `decor/decor-pines-768.webp` | 256164 | `5dcaaea3c81b1bd21dbb8e0ea2d545cd0f040222fa452acbfe9beae2b61cd81b` | Khớp |
| `decor/decor-pines.webp` | 1290088 | `26ddc2f163b600f2251fb5c3825557c49015534326974edfb37e11e674c2dc39` | Khớp |
| `illustrations/scene-atelier-384.webp` | 168844 | `2e077878cd0937d91a805108a372b5ebbcca711b60a5357bc4b67c9cdce54a54` | Khớp |
| `illustrations/scene-atelier-768.webp` | 613160 | `03b2b8399e164bbceeb573bac4f2a752d451fd5f40ef1004c7add777f8fc51e1` | Khớp |
| `illustrations/scene-atelier.webp` | 2302612 | `b3bcb76ca401417233be98fe7c39de8560af8a2c976f5b91a8a578910798c87a` | Khớp |
| `illustrations/scene-cafe-384.webp` | 156324 | `cc1e8cf8845da1a28df61e4c667fc8c32bca55c5da77e8fc1285f4116f578feb` | Khớp |
| `illustrations/scene-cafe-768.webp` | 573872 | `d07adddbf432d47ff171590e5cb30bfbe5939417e316aad31a0824eb3c2f148a` | Khớp |
| `illustrations/scene-cafe.webp` | 2173754 | `b27a43770f42c3a5acb529e7a95903aace6364320c9aa0e5f8f28dac380d0754` | Khớp |
| `illustrations/scene-coast-384.webp` | 154368 | `00cc177258c199a939e056c1f65e8155d19d571ecfc67fdc401819917645bc46` | Khớp |
| `illustrations/scene-coast-768.webp` | 576788 | `cfe62e15010680be1be01e630a29bc830e6bf0b1720b1ad317a8f1bd8ed92c7c` | Khớp |
| `illustrations/scene-coast.webp` | 2206624 | `970e1914a62e25147deac915254296448ca749ee78b7df7e19ee2c96fbb4070c` | Khớp |
| `illustrations/scene-hills-384.webp` | 147080 | `4ae397571458866ae8174177e20ff54f0e1585fb90c19a6cf9e184ec08c93891` | Khớp |
| `illustrations/scene-hills-768.webp` | 541160 | `812aacfa7a459a2012b2b0addc0ebaa4d899710496ca502be7f9556a1063067b` | Khớp |
| `illustrations/scene-hills.webp` | 2062714 | `bf160d8f1853ec40b98dec82033a12e4755300e5e825a0d2843ac1019d4229c2` | Khớp |
| `illustrations/scene-lanterns-384.webp` | 167506 | `1f71c6e45878401681d20b059462a8e072ab591b59bbfd27b1e9186c1b94be43` | Khớp |
| `illustrations/scene-lanterns-768.webp` | 628154 | `e5e2b87bfc2e8bca8cdd7053557385df9af7f978a575c333f57c7aa2cf9338ab` | Khớp |
| `illustrations/scene-lanterns.webp` | 2410958 | `c16880795de3688aefaf8af731d410df7f15e55ddeee4f6b78b58832816753f0` | Khớp |
| `illustrations/scene-street-384.webp` | 168120 | `a7435c80d1d5d2ab7492366edc6e844fc551ed9c1664235a193c5bcac4cc4e4f` | Khớp |
| `illustrations/scene-street-768.webp` | 609816 | `8204906c9bd6518ee67c854760280f5fc48df6475186e61d19f14666546f5afe` | Khớp |
| `illustrations/scene-street.webp` | 2278700 | `510d78a40e8335846e8152409881fcb6c432e3bc8bf325ceccf6f9136df7c6ff` | Khớp |
| `illustrations/spot-keeps-384.webp` | 84164 | `21606ccdff151f48750d8674032a4f7759095580143cfde72015b7de6583e4a2` | Khớp |
| `illustrations/spot-keeps-768.webp` | 336908 | `b79483049ef8353dcb2a8bb33233aff7f35a3174133ffa44e1dbe157fdef4ba1` | Khớp |
| `illustrations/spot-keeps.webp` | 966034 | `7df260b95a92ad274282fd8a236c5c8a240cb8cb0fb3b31e2c60efd2cc42ae28` | Khớp |
| `illustrations/spot-map-384.webp` | 94928 | `75d3f93d0849af0625aaf0c254a7fb8f933dd261b36879a4393ab105c0b74bc2` | Khớp |
| `illustrations/spot-map-768.webp` | 349900 | `cb3fe812031e3e8e728addf8482a53012038abf7ab2414c3be4417949d9ea3c8` | Khớp |
| `illustrations/spot-map.webp` | 1674638 | `03dbbee4c5b453c3d0ea015793df097916f647d6330d9eac6a6dca8da88d4e54` | Khớp |
| `illustrations/spot-memory-384.webp` | 93720 | `5ea15d3e53866419fe54f34fe81d472d3e2acb227d0a11878c2be60f8f3ff27e` | Khớp |
| `illustrations/spot-memory-768.webp` | 372082 | `0b2380ce9d19ced28f3829fb695a67e9cde848af04ea34c52e3af9554fa2f7e0` | Khớp |
| `illustrations/spot-memory.webp` | 1811886 | `ede361212c31455b0cd887dbd3f223c11d82792c362b22ba982a13d7e67449a0` | Khớp |

## Geometry, theme và canonical renderer

| Source runtime | Thiết kế/byte | SHA-256 | Nguồn |
|---|---:|---|---|
| `apps/mobile/src/rudi/asset-pack/source/icon-geometry.json` | 61 thiết kế / 18816 byte | `45ee2665d9d6f6931fc4d03750c5ff19dbcda4a7b01ea4520db170741ac82e1b` | Khớp JSON handoff |
| `apps/mobile/src/rudi/asset-pack/source/paper-geometry.json` | 11 thiết kế / 27940 byte | `313241bae60dfd6d56269a1c5bd1d09f06113fce7511cb344c2b5ae8d1493491` | Khớp JSON handoff |

`AssetIcon` dùng viewBox 24 × 24 từ JSON; host control giữ label, hit target và state, màu semantic từ theme. `PaperObject` giữ width/height theo viewBox, thay palette role memoized; chữ/form/status/control thật nằm ngoài SVG. `PaperPalette.ts` là type neutral, cắt import cycle PaperObject→usePaperPalette→PaperObject; `usePaperPalette` lấy theme hiện tại, tách giấy trang chứa chữ và giấy artwork, không nạp `source/palettes.json` của handoff làm theme thứ hai.

Envelope dùng viewBox 320 × 240, đúng 8 layer `flap/letter/body/left-fold/right-fold/front/seal/closing-flap`; hinge(160, 89), letter translation 44 đơn vị, host ratio 0, 75. `AnimatedEnvelope` scale hinge theo width, không bóp hình bằng scaleX/scaleY khác nhau. `eventKey` từ response, alias `confirmedEventId` chỉ tương thích API mẫu; `useConfirmedProgress` dùng namespace `asset:<kind>:<eventKey>` và registry session. Live reduced motion, route blur/background, cancel/remount settle frame cuối; timing mặc định `useMotion().timing('standard')` từ token canonical 200 ms, reduced 0 ms. Hook không xác nhận nghiệp vụ; receipt stamp không xác nhận nhận tiền và không tween số tiền.

Nếp tiếp tục render từ `apps/mobile/src/rudi/art/nep.ts` và `nep-roi.ts`, qua `NepRoi/NepDien` hiện hữu (Skia và SVG fallback cùng clock/rig). Hai source này khớp checksum snapshot handoff; không trace linh vật từ raster, không dùng pose SVG/PNG static thay rig hoặc tạo nhân vật thứ hai. `Wordmark.tsx` giữ glyph repo gốc; [wordmark provenance](</home/lakiet/project/mobile/ru-di-app/docs/design/2026-10-05-nep-full-audit/production-assets/source/wordmark-provenance.md>) ghi Baloo 2 ExtraBold/EkType/OFL 1.1. Font display dùng bốn Bricolage hiện có qua `src/rudi/fonts.ts`; [OFL Bricolage](</home/lakiet/project/mobile/ru-di-app/docs/design/2026-10-05-nep-full-audit/production-assets/source/OFL-BricolageGrotesque.txt>) đi cùng source handoff, không nạp lại font snapshot. Texture vẫn từ `apps/mobile/assets/textures/` và `ui/Grain.tsx`, với [texture provenance](</home/lakiet/project/mobile/ru-di-app/docs/design/2026-10-05-nep-full-audit/production-assets/source/texture-provenance.md>) của tile sinh seed cố định, không thêm ảnh texture phủ nhiều lớp cho mọi màn.

## Lựa chọn variant thực

`AssetIllustration` đọc `raster-metadata.json` (ratio/focal/fit), width/height dp và `PixelRatio.get()`. `illustrationResolution` tính width ảnh thật: contain=`min(slotWidth, slotHeight×ratio)`, cover=`max(slotWidth, slotHeight×ratio)`, rồi nhân DPR. ≤ 384 px→small, ≤ 768 px→medium, lớn hơn→master; dimension/ratio/density không hữu hạn hoặc ≤ 0→small. Override `resolution`, `fit` và `contentPosition` giữ explicit theo host.

`PlaceArtwork` đã bỏ override medium/small để dùng cùng auto dp × DPR registry; high-density hero có thể chọn master. Cảnh mặc định cover và focal point của manifest; cutout/decor/spot mặc định contain. App không đổi màu raster theo dark scheme; artwork nằm trong bề mặt có giới hạn, UI/text palette đổi bằng token. Decorative Image không nhận touch/a 11 y, `transition=0`; loading/status chữ không baked trong asset. Không preload toàn bộ master và không đổi user media thành artwork fallback có vẻ là ảnh thật.

## Dấu vân tay source tích hợp

Bảng dưới ghi source tại lúc đối chiếu; các wrapper/hook được thích nghi với theme/registry/runtime repo, không tuyên bố là bản sample TSX nguyên byte.

| File | SHA-256 |
|---|---|
| `apps/mobile/src/rudi/asset-pack/source/raster-metadata.json` | `764636005e189b8095237f56d5c2ee016923896a13bb326a00475fcd0cc6b64c` |
| `apps/mobile/src/rudi/asset-pack/react-native/AssetIllustration.tsx` | `f083b42f1ccaf151672f4d1a5581465fcaade44795a380c06a67b516a59de826` |
| `apps/mobile/src/rudi/asset-pack/react-native/AssetIcon.tsx` | `3000addef66bda13d20b8d491249ea7407f2e43c96902994aa9d0e080109850c` |
| `apps/mobile/src/rudi/asset-pack/react-native/PaperObject.tsx` | `5d93d8f36c5c00a24b486c7d57f9d44518550c0ce97a2cf32149068b20d2fd7e` |
| `apps/mobile/src/rudi/asset-pack/react-native/AnimatedEnvelope.tsx` | `c326a3fd5e0cab4be26b4b0c8e185333875e469ae347045c9a7ebdcb9381d316` |
| `apps/mobile/src/rudi/asset-pack/react-native/useConfirmedProgress.ts` | `9d94c2d65f5f2196ac6ee9fe59240833da63f1aa1e72389a23935cc15868bb54` |
| `apps/mobile/src/rudi/asset-pack/react-native/usePaperPalette.ts` | `5d20ab36d4444e84d70fc6fc3ac243e2adf66edfb96537490a5bea669a5eb451` |
| `apps/mobile/src/rudi/asset-pack/resolution.ts` | `f350191912627a922d731602b3a4dafa2c1deaef9da8cca736c2aa1c7a74b9fe` |
| `apps/mobile/src/rudi/art/nep.ts` | `4b35fce764f253eb8ff322187718ce79ec92d81abae6c2f4fe96e1a4ca910bd0` |
| `apps/mobile/src/rudi/art/nep-roi.ts` | `ef6eda7ce77a2dabc90d1e76cdbdb8795845c85eb918bca3f1fd275d902639a8` |
| `apps/mobile/src/rudi/ui/Wordmark.tsx` | `c96b9b23606296868f3d7f4eb56b8dd7bb3ce230424c6de4550d7c19dd3536dc` |
| `packages/shared/tokens.json` | `93ec25e6e55d2f7d1046e9949fa211f18209a631349b01fa3f1c0f88679bd4d7` |

Dung lượng trên là byte trên ổ của asset runtime, không phải network transfer gzip, decoded bitmap RAM, cold-start, FPS hoặc UI-thread motion. Checkpoint web export và focused tests xem [TEST-RESULTS.md](./TEST-RESULTS.md); benchmark/thiết bị/gate còn lại xem [BLOCKERS.md](./BLOCKERS.md).

## Guard và canonical geometry ở checkout tiếp tục

61 icon và 11 paper JSON được đối chiếu exact equality với canonical handoff; [geometry review](/tmp/rudi-nep-ui-final-evidence/guard/geometry-canonical-review.json). Long floating coordinates bị scanner long-number/vn-phone bắt false-positive; xử lý bằng allowlist exact path+SHA+content rules+reason, không nới scanner chung. [Final source identity](/tmp/rudi-nep-ui-final-evidence/final-source-identity.json) ghi staged/whole-candidate guard và local feature commit sau root chốt; không suy thành main landing hoặc full strict gate pass. Bản này giữ nguồn 36 raster nguyên byte, không regen/re-encode art hoặc canonical pack.

## Inventory export cuối — các platform dùng cùng 36 ảnh bất biến

[Export-all summary](/tmp/rudi-nep-ui-final-evidence/checks/expo-export-all-final-summary.json) exit 0:220 files, 105.697.773 byte union. Có 72 physical WebP copies (36 hashed web +36 native extensionless dùng chung Android/iOS), nhưng chỉ 36 unique image hashes; tổng WebP physical bytes 59.613.848. Runtime registry/source 36 WebP vẫn 29.806.924 byte. Android metadata 106 asset entries, iOS 102 (gồm asset incumbent khác), không phải 106/102 design mới. Main web JS 5.437.972 byte; native HBC 7.577.845 Android và 7.390.013 iOS là compile artifacts, không APK/IPA launch hoặc performance benchmark.
