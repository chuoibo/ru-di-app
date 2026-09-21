# Lượt cổng trên `a305b25f`: canary đã xanh, cổng đỏ vì một khác biệt không phải hành vi

Cây `/home/lakiet/wt-go-con-lai`, nhánh `go/p0-w-con-lai`, HEAD `a305b25f`, working tree sạch.
`scripts/gate.sh --strict parity`, 1850 giây, **HỎNG** (exit 1).

```
parity: scenarios=333 steps=10235 scenarios_diff=1 differences=8 database_lane=on media_lane=on
tap:    steps=10235 answered_in_core=9730 served_routes=144→151 unserved=0
```

So với lượt ở `3c3298d6` (`docs/claude/2026-09-19/ket-qua-t5-w7-outings.md`, nhánh go0):
`served_routes` 144 → **151** (+7 route W9 của `a305b25f`), `answered_in_core` **vẫn 9730** — đúng như
phải thế, vì lúc đó chưa kịch bản nào chạm bảy route ấy.

## 1. Canary: XANH

Cổng nối `run && canary && …` bằng `&&` (`scripts/gate.sh:471-491`), nên `run` đỏ thì canary
không chạy. Chạy riêng trên đúng năm kịch bản có byte NUL — tập làm Python 500, tức tập đã
làm canary đỏ ở lượt trước:

```
mode                     applied  differences  verdict
identity                 0        0            ok
status-off-by-one        276      278          ok
...
canary: identity equal, every exercised damage caught      (exit 0)
```

Bản vá `3fda7974` (tắt keep-alive trên `canary.Proxy`) **hoạt động**. Bốn khác biệt identity
của lượt trước đã hết.

Nhắc lại một đính chính: bàn giao bảo sửa ở `internal/proxy`. Chỗ đó đã tắt keep-alive từ W0
(`e0387222`), `httpclient` cũng vậy, `tap` từ W1 (`a84f7f58`). Proxy duy nhất còn sót là
`canary`, và `3fda7974` vá đúng chỗ.

## 2. `DIFF w10/concurrency/post-people-person_id-block`: lệch hạng thời gian, không phải hành vi

Tám khác biệt, và **cả tám chỉ khác nhau ở `<ts#N>`**. Bóc từng cặp reference/candidate rồi
xoá số hạng đi thì hai vế **giống hệt nhau từng byte**. Mười lăm phép so hạng đều lệch **đúng +1**:

```
reference: ts#6  ts#7  ts#8  ts#9  ts#10
candidate: ts#7  ts#8  ts#9  ts#10 ts#11
```

`<ts#N>` không phải giá trị mà là **thứ hạng của mốc thời gian trong kịch bản**:
`Binder.freeze` (`parity/internal/normalize/normalize.go:291-305`) gom các `UnixNano()` phân
biệt, sắp xếp, rồi đánh số từ 1. Hai lượt ghi rơi trúng **cùng một micro-giây** ở một bên sẽ
gộp hai hạng thành một và đẩy mọi hạng sau lệch một. Một dịch chuyển +1 đồng loạt là đúng
dấu vân tay của việc đó.

Cây đã có thuốc: `Binder.TieInstantsSince(mark)` gộp mọi mốc quan sát được trong một bước
thành một hạng. Nhưng `runner.go:259-263` **chỉ gọi nó cho bước `concurrent`**, không cho bước
tuần tự — nên va chạm ở bước tuần tự vẫn lọt.

**Chưa tái lập được.** Chạy riêng kịch bản đó 12 lượt với đủ làn database và media: 0/12 ra
DIFF. Chạy cả thư mục `w10/concurrency` 5 lượt với đủ cờ như cổng: 0/5. Nó phụ thuộc tải máy
và hiếm — một lần trên 333 kịch bản trong lượt cổng này. Cái **đã** chứng minh là bản chất
của tám khác biệt; cái **chưa** chứng minh là điều kiện kích hoạt.

Hệ quả cho chiến dịch: con số «0 khác biệt trên 10 235 bước» ở `3c3298d6` có phần may rủi.
Nó không sai, nhưng nó không lặp lại theo yêu cầu, nên đừng treo kết luận lên một lượt.

## 3. Mười ba `RACY`: thứ tự persona id, không phải Python bất định

Bàn giao đoán 502 là nghi phạm hàng đầu. Không thể đúng: `RACY` tính bằng
`runner.RacySteps(refRuns)` (`parity/cmd/parity/main.go:297`), tức so **reference với chính
reference** qua nhiều lượt lặp, **trước khi** candidate chạy (`main.go:302`); candidate không
tham gia, và đường reference đã tắt keep-alive từ W0.

Nguyên nhân thật, đã truy hết:

1. `pair_key(a, b)` **sắp hai id theo chuỗi** rồi nối bằng `:`
   (`services/api/app/domain/friendship.py:117-124`, `services/api/app/domain/direct.py:46-55`).
2. Persona id là hàm của `scenarioID + "@" + nonce` (`parity/internal/runner/runner.go:523-532`).
3. Reference lặp lại **với một nonce mới mỗi lượt** (`parity/cmd/parity/main.go:288-290`).

Nên thứ tự sắp của hai persona là một đồng xu. Đo trên 200 000 nonce ngẫu nhiên cho
`w8/concurrency/post-papers-paper_id-send`: `owner` đứng trước `mate` **49,84%** lượt. Xác
suất ba lượt reference cùng thứ tự là `p³ + (1-p)³ = 25,0%`, tức **75% kịch bản phải ra RACY**.

Đo thực nghiệm, 10 lượt chạy kịch bản đó: **8 lượt RACY**, và bước bị nêu **luôn luôn và chỉ
là `owner_opens_pair`** — một bước *tuần tự* (`POST /people/{person_id}/dm`), không phải bước
burst nào của kịch bản. 8/10 khớp dự đoán 75%.

Cùng lớp, khác chỗ: `w4/concurrency` đỏ ở `batch_one` / `owner_batches_dinner`, cũng là bước
tuần tự (`POST /batches`), 5/6 lượt. `w10/concurrency/post-people-person_id-dm` đỏ 5/5 ở hai
bước tạo pair — card `docs/migration/routes/people/POST-people-person_id-dm.md:85` đã ghi đúng
nguyên nhân này từ trước.

**Kết luận: RACY là hiện vật của harness, không phải Python bất định và không phải lỗi port.**
Reference đổi nonce mỗi lượt lặp, nên bất cứ giá trị nào phụ thuộc *thứ tự* của hai persona id
sẽ đổi theo. `rep.Racy` cũng không làm đỏ cổng (`main.go:420` chỉ fail khi
`Differences > 0 || stale > 0 || Tap.Unserved > 0`), và `Closest` cho candidate khớp bất kỳ
lượt reference nào.

Vì sao phải đổi nonce mỗi lượt: ba lượt reference ghi vào **cùng một database**; dùng lại nonce
thì lượt sau đụng hàng của lượt trước (cùng persona id, cùng khoá idempotency) và thành phát
lại chứ không phải một cuộc đua mới.

Hướng sửa nếu muốn dứt điểm, chưa làm trong lượt này: cho `runner.PersonaID` cấp id sao cho thứ
tự chuỗi của các persona **trùng thứ tự khai báo** trong kịch bản. Việc này không che được lỗi
thật — một bản Go sắp `pair_key` khác Python vẫn ra DIFF ở phép so reference-với-candidate.

## 4. Lỗi hạ tầng thứ ba, tìm ra nhờ kịch bản W9 mới

`MOBILE_OTP_DEBUG_CODE` tới được container Python nhưng **không** tới core Go. Ở stack
candidate, `POST /auth/otp/request` do Go phục vụ, nên Go sinh mã ngẫu nhiên còn reference sinh
`000000`; bước verify bằng mã đúng ra 201 ở reference và **422 `otp_code_invalid`** ở candidate,
kéo theo 25 khác biệt trên 55 bước.

Không phải lỗi logic của Go: Go có đọc biến đó (`services/core/internal/sms/sms.go:26`, giải ở
`:150-168`). Đúng cùng họ với `MOBILE_INTERNAL_TOKEN` mà `8c13e1c3` vừa vá — một biến tới được
Python mà không tới core. Đã sửa bằng một khai báo duy nhất `otp_debug_code`
(`scripts/parity_stacks.sh:114-118`) dùng cho cả hai phía (`:175` và `:209-210`).

Lỗi này **vô hình cho tới khi có kịch bản**. Không kịch bản nào chạm hai route OTP, nên nó nằm
đó qua nhiều commit mà không ai thấy. Đúng điều bàn giao đã nói: trạng thái đo bằng kịch bản,
không đo bằng mã.

## 5. Không lật nhãn nào trong lượt này

Cổng đỏ, nên **không** hàng nào rời `PORTED-UNPROVEN`. Mười một hàng outings W7 vẫn giữ nhãn cũ,
dù lý do đỏ không liên quan tới chúng: điều kiện gỡ nhãn là một lượt `gate.sh parity` **xanh**
trên SHA sạch, và lượt này không xanh.
