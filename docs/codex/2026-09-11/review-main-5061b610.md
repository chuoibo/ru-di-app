# Review độc lập main 5061b610 — 11/09/2026

Method: dual-agent (A: `/root/sep11_art` · B: `/root/sep11_technical`), cộng kiểm tra native và phán quyết trực tiếp của Codex chính. A hoàn tất trên bảng không nhãn/ảnh team; lượt bổ sung ảnh mới cho A gặp giới hạn sử dụng. Codex chính tự xem ảnh mới, không nhận lượt bổ sung chưa chạy là bằng chứng độc lập.

**Phán quyết: APPROVE trong phạm vi sửa UI đã xác minh của R1, R3, R5; R4 đã sửa đúng cấu trúc hình. REQUEST_CHANGES riêng việc đóng R2/cổng motion. F45 có tiến bộ đủ rõ để giữ bản mới, nhưng kiểm chứng nghĩa không nhãn bằng người ngoài vẫn chưa hoàn thành. Không ký “toàn app/release đã đạt”, cũng không giữ các lỗi UI đã sửa mở chỉ vì mục tiêu stunning còn rộng.**

## 1. Hai finding còn tái hiện ở cổng R2

### B11 — P1: reset hỏng hoặc dump thiếu số liệu vẫn cho bảng hợp lệ

Vị trí: `docs/claude/2026-09-10/motion/do-motion.sh:87`, `:92`, `:98–103`, parser `:68–71`.

Codex chính đã đọc script canary độc lập của B và tự chạy lại vào thư mục riêng [root-canary-evidence](root-canary-evidence/). PATH chỉ có executable giả và `/usr/bin:/bin`, không đụng thiết bị. Kết quả giống B:

| Đầu vào cố tình gây lỗi | Kết quả runner hiện tại | Tại sao phải sửa |
|---|---|---|
| Chỉ lệnh `gfxinfo reset` trả exit 1 | **Exit 0, cả 4 hàng hợp lệ** | Không biết cửa sổ đo đã được xóa. Khung trước/warm-up có thể bị tính vào thao tác sau. |
| Dump chỉ có `Total frames rendered: 12` | **Exit 0**, percentile trống, counters `?`, số khung chậm = **0** | Histogram thiếu bị biến thành bằng chứng “không có khung chậm”. |
| PID là chuỗi ổn định `pid-not-known` | **Exit 0** | Ràng buộc chỉ kiểm không rỗng/bằng nhau, chưa kiểm định dạng. Đây là probe parser, không khẳng định adb bình thường trả dạng đó. |

Reset/dump exit chưa tham gia điều kiện `hop_le`. Đây là lỗ hổng thực của lời hứa fail-closed. Nó **không chứng minh** tám dump v2 tác giả đã ghi đều sai: B đếm lại tám histogram, tổng khớp frames và không bucket ≥150 ở cả tám. Cần phân biệt dữ liệu đã lưu nhất quán với cổng chưa chặn mọi lỗi cần chặn.

**Điều kiện đóng:** reset/dump lỗi phải làm lượt invalid; không chạy hoặc không công bố cửa sổ hợp lệ sau reset lỗi. PID phải parse đúng; mọi metric được công bố phải hiện diện và hợp lệ. Histogram thiếu phải invalid, không mặc định 0. Thêm các canary trên, giữ đối chứng dump thật xanh. Không cần dựng release để sửa/kiểm cổng shell này.

Bằng chứng: [reset-failed.log](root-canary-evidence/reset-failed.log), [dump-malformed.log](root-canary-evidence/dump-malformed.log), [bảng 13 canary chạy lại](root-canary-evidence/canary-summary.json).

### B12 — P2: scale chưa biết gốc vẫn bị thay; ghi/restore lỗi vẫn có thể xanh

Vị trí: cùng script `:41–59`, đặc biệt `:45`, `:50`, `:58`; vòng đo `:82`.

- Gốc giả lập là `0.5 / 1.5 / 2`. Riêng đọc gốc `animator_duration_scale` bị rỗng: script vẫn ghi cả ba về 0, chạy warm-up + 4 flow; cuối cùng exit 1 nhưng animator còn **0** vì không có gốc để phục hồi.
- Từ chối ghi scale 0: script vẫn **exit 0**, mang nhãn reduce dù thực tế còn `0.5 / 1.5 / 2`.
- Từ chối ghi trả gốc: script vẫn **exit 0**, để cả ba ở 0. Restore dùng `|| true` và không đối chiếu kết quả đọc lại.

**Điều kiện đóng:** đọc/parse đủ gốc trước mọi mutation; nếu thiếu thì dừng ngay, không `put`/Maestro. Đối chiếu ghi và read-back với chế độ yêu cầu trước đo. Restore phải xác minh hoặc báo thất bại và trả exit lỗi; không hứa phục hồi khi adb không hoạt động. Thêm canary đọc gốc hỏng, put hỏng, restore hỏng và gốc khác 1.

Nhánh đã tốt: bình thường trả đúng `0.5 / 1.5 / 2`; INT giữa lượt trả exit 130, chỉ chạy warm-up + m1 rồi dừng, trả đúng gốc. Không yêu cầu làm lại nhánh này. [Trace INT](root-canary-evidence/interrupt-trace.json), [gốc rỗng](root-canary-evidence/original-empty-trace.json), [restore lỗi](root-canary-evidence/restore-failed-trace.json).

## 2. R1: xác nhận đóng trên Android dev client

Codex quay mới, không dùng clip tác giả để ký. Ba lượt **scale 1 → 0 → 1**, cùng PID **29173**, app đang foreground. Mỗi lượt gồm chi tiết quán + Back, sheet Tạo mới + Back, modal Check-in + Back. Ba scale đều được đọc lại trước quay, PID được đối chiếu cuối lượt. [Cấu hình và trace](native-evidence/motion-confirm/settings.json).

| Lượt xác nhận | Quan sát |
|---|---|
| a — scale 1 | Có trượt ngang/dọc và sheet đang đi vào; bộ đếm bắt được chuỗi nhiều khung. |
| b — scale 0 giữa phiên | **6 lần chuyển đều 1 khung thay đổi** theo bộ đếm; root nhìn trước/sau thấy chi tiết, sheet và modal xuất hiện đầy đủ, không trượt qua màn. |
| c — scale 1 trở lại | Sáu chuỗi gộp **9/9/7/5/9/7 khung**; root thấy trượt ngang/dọc/sheet trở lại. |

Clip: [thường](native-evidence/motion-confirm/a-normal.mp4), [giảm chuyển động](native-evidence/motion-confirm/b-reduced.mp4), [thường trở lại](native-evidence/motion-confirm/c-normal-again.mp4). Các khung tiêu biểu nằm trong `motion-confirm/selected-frames/`.

Phương pháp đếm dùng script `so-khung.py` của team trên video mới, còn kết luận vị trí màn được root kiểm bằng mắt. Video trích ở 30fps, resize 540×1200 để xem; không lấy video nén làm phép đo độ nét/màu hoặc thời gian touch latency. Chuỗi a bị chia thành nhiều run vì biến đổi không vượt ngưỡng ở một số khung; không sửa thuật toán để ép ra sáu sự kiện. Chuỗi b và c có đúng sáu sự kiện và đối chứng hình trực tiếp. Lượt đầu `native-evidence/motion/` có banner dev “Refreshing…”; giữ làm chẩn đoán, **không dùng làm lượt ký**, đã quay lại một lượt xác nhận sau khi kiểm thử/biên dịch kết thúc.

Đây là đủ để đóng đúng lỗi Android dev-client R1 được báo ngày 10/09. Release, iOS và máy thật là các ô chưa đo riêng; không tự biến thành bằng chứng R1 vẫn hỏng trên máy đã thử. Không chạy lại benchmark gfxinfo mới trong lượt này, không chép số janky v2 thành số root đo hôm nay.

## 3. R3/R5 và nghĩa hình: kết luận trực tiếp trên ảnh native mới

| Mục | Bằng chứng mới | Phán quyết trong phạm vi sửa |
|---|---|---|
| R3 — lời hứa lặp ở Khám phá | `01`, `12`, `15`, `20`: 1.0/2.0 × sáng/tối. Cổng XML chạy lại **4/4 xanh**; tên → “View đẹp” → mô tả không nhắc lại. [Ma trận](native-evidence/r3-matrix.json). | **Đóng lỗi lặp lời ở fixture đã thử.** Không bắt PR này phải có ảnh thật mới được đóng. Live reason dài 2.0 vẫn chưa được thử. |
| R5 — tờ AI | `02/03`: đóng/mở sáng 1.0; `13`: tối 1.0; `16/17`: đóng/mở tối 2.0; `21`: sáng 2.0. Đã bấm mở lý do, câu giải thích xuất hiện, CTA và “AI nháp” còn rõ. | **Đóng nhịp AI fixture.** Đổi title và giấu giải thích có tác dụng thật. Thẻ live chưa thử. |
| R5 — Cài đặt | `04/05`, `14`, `18/19`, `22/23`: sáng/tối và chữ 1.0/2.0 ở các phần đã cuộn. Các nhóm nằm trên giấy, đường phân cách mảnh, không còn thẻ bọc mỗi nhóm. | **Đóng sửa bề mặt ở các phần đã nhìn.** Source năm màn đã bỏ Card; không lấy đó thay cho thử hàng phiên/block live hoặc save-error. |
| F45 — Kẹt xe | `07/11`, `25/26`: khay 64dp, chữ 1.0/2.0, sáng/tối. Bánh và đuôi xe rộng làm rõ đây là phương tiện; không còn khối đứng giống điện thoại cũ. | **Chấp nhận sửa đạo cụ.** Nghĩa cụ thể “kẹt” so với “đang đi/lỡ chuyến” còn cần người ngoài; giữ nhãn trong khay. |
| F45 — Trả tiền nè | Cùng bốn ảnh khay. Oval trên tờ trước làm dấu tiền đọc rõ; A trước brief cũng ưu tiên đọc tiền. | **Chấp nhận sửa dấu tờ bạc.** Chưa suy thành tỷ lệ hiểu đúng của người dùng. |
| F45 — chưa có tin nhắn | `09/10`: bong bóng thực sự có đuôi lộ bên ngoài Nếp; root đọc được liên hệ thoại. A trước brief vẫn có thể đọc như bảng thuyết trình. | **Lỗi đuôi bị che đã sửa.** Nghĩa cảnh không nhãn chưa đủ đồng thuận; chuyển sang thử người ngoài thay vì tiếp tục tự vẽ theo tên pose. |
| R4 — bộ lọc | `08/09/10`: không còn vòng hở; tấm che lệch trên tờ có dòng chữ, một ô viền coral. Có chụp đúng hình native, không chỉ tiêu đề mục. | **Đóng lỗi vòng hở và cấu trúc “che”.** Hình có thể vẫn gợi chọn ô/bố cục; chưa coi nó là biểu tượng không cần lời cho “hết kết quả”. |

Khay chữ 2.0: [sáng](native-evidence/25-stickers-light2.png), [tối](native-evidence/26-stickers-dark2.png) — đủ tám lựa chọn, hai cột, cả hàng cuối nằm trong sheet. Đây là ô tác giả nói chưa đo được, nay root đã bổ sung. Không đo 1.3 lần này.

## 4. Đẹp, tinh tế, stunning — nhận xét sau sửa

**App sạch hơn, có kỷ luật hơn và đồng bộ hơn.** Trước mắt ở Khám phá không còn ba tầng “hợp gu”; ô giấy với một chi tiết coral hợp hệ hơn đĩa hồng cũ. Cài đặt bớt lớp nền/bo/bóng, nhường cho nội dung và các lựa chọn hệ thống. Tờ AI không còn tranh tiêu đề với cả màn chat; hành động đọc trước, lý do mở khi cần. Đây là cải thiện về độ tinh tế, không chỉ việc test qua.

**Khám phá vẫn thiên về danh mục để đọc.** Ở đầu trang, hình bát/nồi là dấu loại địa điểm; chúng chưa cho người xem cảm giác ánh sáng, đồ ăn, chỗ ngồi của một nơi cụ thể. Chữ 2.0 hợp lệ nhưng riêng địa điểm dẫn vẫn chiếm phần lớn khung nhìn. Đó là công việc biên tập hình/nội dung và nhịp bố cục tiếp theo, không phải lý do mở lại bug lặp chữ đã sửa. Không bịa ảnh địa điểm; trạng thái không ảnh vẫn cần được đối xử như trạng thái chính thức.

**Nếp đã dễ đọc hơn nhưng chất liệu tối còn mỏng.** Ánh mực sáng trên nền indigo tạo nhận diện, song thân giấy và nền gần nhau khiến một số cảnh giống sơ đồ nét. Bản sáng có cảm giác tờ giấy hữu hình hơn. Không có phép đo tương phản pixel mới trong lượt này; đây là nhận xét mỹ thuật có ảnh đối chiếu, không phải tuyên bố vi phạm tỷ lệ tương phản.

**Một khoản cải thiện nhỏ nên tách khỏi gate cũ:** nút “Mở bình chọn” trong tờ AI chỉ hiện icon cột biểu đồ, người nhìn có thể nghĩ số liệu/kết quả. Accessibility label có tên đúng nhưng chưa giúp người nhìn màn hiểu ngay hành động. Cân nhắc nhãn “Bình chọn” hoặc biểu tượng lá phiếu; kiểm 1.0/2.0. Đây là P2 về khả năng nhận biết, không bằng chứng nút không hoạt động và không phủ quyết sửa nhịp R5.

Giữ hướng giấy–mực/Nếp. Chưa ký “stunning xuyên suốt” từ những màn này; lượt hiện tại cũng không tái duyệt toàn bộ Album/Sở thích/tiền. Đánh giá các màn không đổi ngày 10/09 vẫn là lịch sử có phạm vi, không phải những ảnh mới chụp hôm nay.

## 5. Những gì thực sự đã kiểm và không được nhận vơ

- SHA `5061b610735df34d189eab61af997ac4783fec14`, bằng local tracking `origin/main`. Không fetch/đọc CI GitHub mới, không xác thực toàn bộ các lần merge từ lời kể.
- B biên dịch test mới, typecheck `tsc --noEmit` qua, chạy **75/75 ca** trong 8 file: motion, lý do, chống mồ côi, Khám phá, art, sticker, không Card, settings. Log trong [technical-evidence](technical-evidence/). Không chạy lại toàn npm/backend rồi dùng số tác giả làm số của reviewer.
- 13 canary độc lập B và 13 lượt root chạy lại cho cùng kết quả. Canary là kiểm shell với executable giả, không phải số hiệu năng native. Không thay dependency/mock thư viện sản phẩm để tạo pass.
- Detector một lần do B: exit 0, `[]`, 0 finding/rule. Native playbook ghi detector không áp dụng cho native; đây chỉ là scan nguồn phụ, không phải native approval. Không browser overlay.
- Hai danh sách lỗi local main/pr593 giống nhau với năm ca `test_android_emulator_may_ket.py`, file test và script emulator không đổi trong diff này. Chỉ xác nhận được tính nhất quán danh sách và source không đổi; chưa xác minh provenance/full trace của bốn CI merge. Không gọi năm ca ấy là “baseline đã tự tái hiện”.
- Chụp bằng ADB trên máy ảo Android 15, 1080×2400px, 420dpi (~411×914dp), APK dev client có sẵn. Metro bật fixture tổng hợp, `EXPO_NO_DOTENV=1`, fingerprint `codex-sep11-5061b610`, API cổng 18999 không có dịch vụ. Không khởi động backend/e2e stack hoặc thay manifest.
- `19-settings-save-error-dark2` là **tên ý định probe**, không phải error đã thấy: không có hồ sơ live, Switch disabled, không tạo request lưu. `24-sessions-state.png` là **skeleton**; XML của nó lỗi idle và bị helper đọc lại dump trước, đã đánh dấu **invalid/excluded**. Helper đã sửa để từ chối dump không báo thành công. Những ảnh này không được tính vào kết quả save-error/live.
- PNG thường chụp trước XML, chỉ ghép cặp khi trạng thái ổn định. Một lượt quay đầu có banner dev nên được giữ như chẩn đoán và quay xác nhận riêng; không xóa thất bại khỏi provenance.

Chưa kiểm: release qua HTTPS, live reason/thẻ AI dài 2.0, save-error Cài đặt có session, hàng phiên/block live, chat hai bên/F42 native, iOS/máy thật/120Hz/TalkBack, tablet/màn hẹp khác, IME, đo tương phản pixel mới. Không cần dùng các ô ngoài phạm vi đó để buộc R1/R3/R5 sửa lại. Chúng là công việc trước một gate toàn app/release.

## 6. Bàn giao cho team

1. **Sửa R2 theo B11/B12**, rồi chạy canary thật sự đỏ trên reset/dump/scale hỏng và xanh với dữ liệu đầy đủ. Điều này có thể chốt độc lập, không chờ TLS.
2. **Giữ các sửa UI đã đạt** R1/R3/R5 và bản hình mới. Không yêu cầu thêm animation hay vẽ lại toàn bộ.
3. **Thử nghĩa không nhãn với người ngoài** trên hình đang có, ghi nguyên câu trả lời trước khi lộ nhãn. Kết quả reviewer AI khác nhau chính là lý do cần bước này; không tạo tỷ lệ từ ý kiến agent.
4. Sau đó, tiến hành release/HTTPS và các luồng live đã liệt kê như một gate riêng. Nâng mức stunning tập trung vào Khám phá có cảm giác nơi chốn và tương quan chất giấy ở tối.

Đã phục hồi font 1.0, sáng, cả ba animation scale = 1; gỡ reverse 8099. Đọc lại tại [cleanup.json](native-evidence/cleanup.json). Metro audit đã dừng. Không sửa sản phẩm, manifest, dependency hoặc backend; không commit/push, không merge PR hay gửi thông báo. Tracked diff trống; các untracked có sẵn được giữ nguyên.

Ảnh/video/scripts, phân loại loại trừ và hash được lưu trong [gallery](native-evidence/index.html), [manifest](native-evidence/manifest.json), [limitations](native-evidence/limitations.json). Báo cáo là kết quả review trong checkout hiện tại, chưa phải artifact đã publish lên remote.
