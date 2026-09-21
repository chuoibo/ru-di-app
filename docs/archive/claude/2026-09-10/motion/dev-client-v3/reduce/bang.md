| chuỗi | khung | janky | p50 | p90 | p95 | p99 | khung>150ms | slow UI | slow draw | missed vsync | rc | pid |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| m1-doi-tab | 45 | 80.00% | 28 | 46 | 81 | 97 | 0 | 22 | 25 | 2 | rc=0 | 9823 |
| m2-cuon-kham-pha | 708 | 2.26% | 16 | 27 | 28 | 32 | 2 | 1 | 16 | 0 | rc=0 | 9823 |
| m3-sheet-tao | 62 | 24.19% | 14 | 16 | 16 | 16 | 0 | 0 | 13 | 0 | rc=0 | 9823 |
| m4-back-chi-tiet | 59 | 38.98% | 23 | 32 | 32 | 42 | 0 | 7 | 16 | 0 | rc=0 | 9823 |

Chế độ: reduce · scale lúc đo: window_animation_scale=0 transition_animation_scale=0 animator_duration_scale=0  · scale gốc: window_animation_scale=1 transition_animation_scale=1 animator_duration_scale=1 
p50…p99 là nhãn bucket của histogram gfxinfo chứa percentile ấy (histogram tới 4950 ms); khung>150ms là tổng các bucket từ 150 ms.
Hàng hợp lệ = Maestro rc 0, dump rc 0, một pid trước/sau/trong dump, frames > 0, đủ percentile và bộ đếm, histogram có mặt và cộng đúng bằng frames.
