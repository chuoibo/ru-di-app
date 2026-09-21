| chuỗi | khung | janky | p50 | p90 | p95 | p99 | khung>150ms | slow UI | slow draw | missed vsync | rc | pid |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| m1-doi-tab | 268 | 13.43% | 26 | 31 | 32 | 89 | 0 | 21 | 26 | 1 | rc=0 | 8258 |
| m2-cuon-kham-pha | 924 | 3.14% | 16 | 27 | 30 | 32 | 3 | 3 | 27 | 2 | rc=0 | 8258 |
| m3-sheet-tao | 406 | 9.11% | 20 | 31 | 32 | 36 | 0 | 3 | 30 | 0 | rc=0 | 8258 |
| m4-back-chi-tiet | 274 | 10.22% | 28 | 32 | 32 | 65 | 0 | 6 | 26 | 0 | rc=0 | 8258 |

Chế độ: thuong · scale lúc đo: window_animation_scale=1 transition_animation_scale=1 animator_duration_scale=1  · scale gốc: window_animation_scale=1 transition_animation_scale=1 animator_duration_scale=1 
p50…p99 là nhãn bucket của histogram gfxinfo chứa percentile ấy (histogram tới 4950 ms); khung>150ms là tổng các bucket từ 150 ms.
Hàng hợp lệ = Maestro rc 0, dump rc 0, một pid trước/sau/trong dump, frames > 0, đủ percentile và bộ đếm, histogram có mặt và cộng đúng bằng frames.
