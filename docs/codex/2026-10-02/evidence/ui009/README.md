# UI-009 · Bìa từ native tới Welcome

Bằng chứng Android trên AVD tổng hợp riêng: guest, portrait
1080×2400/density420, light, font1.0, animation scale1. Không có tài khoản,
ảnh bill hoặc dữ liệu người thật.

- [Trước sửa](startup-before.png): release cục bộ `8ac64443`, icon Expo và
  trang trắng trước JS, rồi Welcome trượt vào trên nền trắng.
- [Sau sửa](startup-after.png): cùng trạng thái app, wordmark/indigo trước
  JS rồi Welcome đứng sẵn. Source của APK dựng trước commit đã đối chiếu
  8/8 file khớp `6144ff2c`; fingerprint literal `b10-ui009-review`.
- [Clip sau sửa](startup-after.mp4): recording native thật, bundled release,
  không Metro; ký debug cục bộ. Không phải bản production.

Hai lần quay có wallpaper launcher khác và mốc bắt đầu video khác.
Không dùng chúng để so thời gian cold-start hoặc pixel diff. Montage là
khung lấy mẫu, không loại được flash rất ngắn giữa hai mẫu. Clip không đo
first-brand-frame300ms, FPS app hay độ mượt trên máy thật.

APK sau dựng/cài trùng SHA256:
`3abcfa68341e6114ee83652cc8af9a652f85f731c98cfd28810689c894928945`.
Nguồn, cấu hình narrow/tablet/reduced, review và lệnh QA retest ở
[bàn giao B10](../../b10-handoff.md#ui-009--pattern-native-bằng-chứng-và-retest-bổ-sung).
