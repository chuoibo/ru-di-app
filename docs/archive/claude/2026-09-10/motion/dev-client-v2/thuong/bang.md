| chuỗi | khung | janky | p50 | p90 | p95 | p99 | khung>150ms | slow UI | slow draw | missed vsync | rc | pid |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| m1-doi-tab | 271 | 13.28% | 26ms | 31ms | 32ms | 85ms | 0 | 21 | 34 | 0 | rc=0 | 22205 |
| m2-cuon-kham-pha | 907 | 2.54% | 16ms | 26ms | 29ms | 31ms | 0 | 0 | 20 | 0 | rc=0 | 22205 |
| m3-sheet-tao | 406 | 8.62% | 16ms | 29ms | 31ms | 34ms | 0 | 2 | 28 | 0 | rc=0 | 22205 |
| m4-back-chi-tiet | 273 | 10.62% | 27ms | 32ms | 36ms | 65ms | 0 | 6 | 28 | 0 | rc=0 | 22205 |

Chế độ: thuong · scale lúc đo: window_animation_scale=1 transition_animation_scale=1 animator_duration_scale=1  · scale gốc: window_animation_scale=1 transition_animation_scale=1 animator_duration_scale=1 
p50…p99 là nhãn bucket của histogram gfxinfo chứa percentile ấy (histogram tới 4950 ms); khung>150ms là tổng các bucket từ 150 ms.
