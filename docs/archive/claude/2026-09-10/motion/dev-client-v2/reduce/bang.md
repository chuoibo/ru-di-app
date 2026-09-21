| chuỗi | khung | janky | p50 | p90 | p95 | p99 | khung>150ms | slow UI | slow draw | missed vsync | rc | pid |
|---|---|---|---|---|---|---|---|---|---|---|---|---|
| m1-doi-tab | 45 | 80.00% | 30ms | 61ms | 69ms | 97ms | 0 | 20 | 29 | 0 | rc=0 | 23771 |
| m2-cuon-kham-pha | 704 | 1.99% | 16ms | 21ms | 24ms | 27ms | 0 | 0 | 14 | 0 | rc=0 | 23771 |
| m3-sheet-tao | 63 | 33.33% | 16ms | 16ms | 26ms | 31ms | 0 | 0 | 19 | 0 | rc=0 | 23771 |
| m4-back-chi-tiet | 60 | 31.67% | 23ms | 29ms | 32ms | 40ms | 0 | 5 | 18 | 0 | rc=0 | 23771 |

Chế độ: reduce · scale lúc đo: window_animation_scale=0 transition_animation_scale=0 animator_duration_scale=0  · scale gốc: window_animation_scale=1 transition_animation_scale=1 animator_duration_scale=1 
p50…p99 là nhãn bucket của histogram gfxinfo chứa percentile ấy (histogram tới 4950 ms); khung>150ms là tổng các bucket từ 150 ms.
