#!/usr/bin/env python3
"""End-to-end check of the group bot «Rủ Đi AI» on a running stack (the Go
front door): three accounts by OTP, a group of friends, a short chat, then
three invocations — a place question, a bill split, and a money action the
bot must refuse — each polled to its end, its card and timing printed.

    MOBILE_OTP_DEBUG_CODE=<code> python3 scripts/e2e_bot_nhom.py [--base http://127.0.0.1:8124]

The stack must run with MOBILE_OTP_LOG_CODES=1 and MOBILE_OTP_DEBUG_CODE set
(dev only) and the group engine on Go. Phone numbers are synthetic
(+849000xxxxx); nothing real is used. Standard library only.
"""

from __future__ import annotations

import argparse
import json
import os
import random
import sys
import time
import urllib.error
import urllib.request
import uuid
from datetime import datetime, timezone


class Api:
    def __init__(self, base: str) -> None:
        self.base = base.rstrip("/")

    def call(
        self,
        method: str,
        path: str,
        token: str | None = None,
        body=None,
        expect=(200, 201, 202),
    ):
        data = None if body is None else json.dumps(body, ensure_ascii=False).encode()
        for attempt in range(3):
            req = urllib.request.Request(self.base + path, data=data, method=method)
            req.add_header("Content-Type", "application/json")
            if method != "GET":
                req.add_header("Idempotency-Key", str(uuid.uuid4()))
            if token:
                req.add_header("Authorization", "Bearer " + token)
            try:
                with urllib.request.urlopen(req, timeout=60) as r:
                    raw = r.read()
                    return r.status, (json.loads(raw) if raw else None)
            except urllib.error.HTTPError as e:
                raw = e.read()
                payload = json.loads(raw) if raw else None
                if e.code == 429 and path.startswith("/auth/") and attempt < 2:
                    print(f"    429 ở {path}, đợi 61 s", file=sys.stderr)
                    time.sleep(61)
                    continue
                if e.code in expect:
                    return e.code, payload
                raise SystemExit(f"HỎNG {method} {path}: {e.code} {payload}")
        raise SystemExit(f"HỎNG {method} {path}: hết lượt thử")


def login(api: Api, phone: str, code: str, name: str) -> tuple[str, str]:
    _, ch = api.call("POST", "/auth/otp/request", body={"phone": phone})
    _, s = api.call(
        "POST",
        "/auth/otp/verify",
        body={"phone": phone, "challenge_id": ch["challenge_id"], "code": code},
    )
    api.call("PATCH", "/people/me", s["token"], {"display_name": name})
    return s["token"], s["person_id"]


def now() -> str:
    return datetime.now(timezone.utc).strftime("%Y-%m-%dT%H:%M:%SZ")


def post_msg(api: Api, ctx: str, token: str, text: str) -> str:
    _, m = api.call(
        "POST", f"/contexts/{ctx}/messages", token, {"kind": "text", "body": text}
    )
    return m["id"]


def invoke(
    api: Api,
    ctx: str,
    token: str,
    command: str,
    prompt: str,
    trigger: str | None,
    turns: list[dict],
) -> dict:
    body = {
        "logical_id": str(uuid.uuid4()),
        "command": command,
        "prompt": prompt,
        "boi_canh": {
            "ban": 1,
            "nguon": "chat-nhom",
            "tongLuot": len(turns),
            "daCat": False,
            "luot": turns,
        },
    }
    if trigger:
        body["trigger_message_id"] = trigger
    t0 = time.monotonic()
    code, inv = api.call("POST", f"/contexts/{ctx}/ai-invocations", token, body)
    if code not in (200, 201, 202):
        raise SystemExit(f"HỎNG gọi bot: {code} {inv}")
    while inv["status"] not in ("succeeded", "failed", "cancelled"):
        if time.monotonic() - t0 > 120:
            raise SystemExit(
                f"HỎNG: lượt {inv['id']} quá 120 s, trạng thái {inv['status']}"
            )
        time.sleep(1)
        _, inv = api.call("GET", f"/contexts/{ctx}/ai-invocations/{inv['id']}", token)
    inv["_giay"] = round(time.monotonic() - t0, 1)
    if inv.get("message_id"):
        _, msg = api.call("GET", f"/contexts/{ctx}/messages/{inv['message_id']}", token)
        inv["_card"] = msg.get("card")
    return inv


def show(title: str, inv: dict) -> None:
    print(
        f"\n=== {title}: {inv['status']} sau {inv['_giay']} s (code={inv.get('code')})"
    )
    card = inv.get("_card") or {}
    phan = (
        card.get("payload", {}).get("phan") if card.get("kind") == "tra_loi" else [card]
    )
    for p in phan or []:
        kind, pay = p.get("kind"), p.get("payload") or {}
        if kind == "text":
            print("  [chữ]", pay.get("text"))
        elif kind == "places":
            for pl in pay.get("places", pay.get("items", [])):
                print(
                    "  [quán]",
                    pl.get("name") or pl.get("ten"),
                    "—",
                    pl.get("reason") or pl.get("ly_do") or "",
                )
        else:
            print(f"  [{kind}]", json.dumps(pay, ensure_ascii=False)[:400])


def main() -> None:
    ap = argparse.ArgumentParser()
    ap.add_argument("--base", default="http://127.0.0.1:8124")
    args = ap.parse_args()
    code = os.environ.get("MOBILE_OTP_DEBUG_CODE")
    if not code:
        raise SystemExit("cần MOBILE_OTP_DEBUG_CODE (mã OTP debug của stack dev)")
    api = Api(args.base)
    tag = random.randint(10000, 99999)
    names = ["Minh", "Lan", "Tú"]
    users = []
    for i, n in enumerate(names):
        users.append(login(api, f"+849000{tag}{i}", code, f"{n} E2E"))
    (ta, a), (tb, b), (tc, c) = users
    print(f"--- 3 tài khoản: {', '.join(names)}")
    _, ctx = api.call("POST", "/contexts", ta, {"display_name": f"Hội E2E {tag}"})
    ctx = ctx["id"]
    for tok, pid in ((tb, b), (tc, c)):
        _, m = api.call("POST", f"/contexts/{ctx}/members", ta, {"person_id": pid})
        api.call("POST", f"/memberships/{m['id']}/accept", tok, {})
    _, caps = api.call("GET", f"/contexts/{ctx}/chat-capabilities", ta)
    print("--- nhóm", ctx, "capabilities:", json.dumps(caps, ensure_ascii=False)[:300])

    m1 = post_msg(api, ctx, tb, "Chiều nay ai rảnh không?")
    m2 = post_msg(api, ctx, tc, "Mình rảnh, mà muốn chỗ yên tĩnh ngồi làm việc")
    m3 = post_msg(api, ctx, ta, "Q3 cho gần nhé, cà phê ngon ngon")
    turns = [
        {
            "id": m1,
            "vai": "ban",
            "biDanh": "Bạn 1",
            "loai": "chu",
            "luc": now(),
            "chu": "Chiều nay ai rảnh không?",
        },
        {
            "id": m2,
            "vai": "ban",
            "biDanh": "Bạn 2",
            "loai": "chu",
            "luc": now(),
            "chu": "Mình rảnh, mà muốn chỗ yên tĩnh ngồi làm việc",
        },
        {
            "id": m3,
            "vai": "toi",
            "loai": "chu",
            "luc": now(),
            "chu": "Q3 cho gần nhé, cà phê ngon ngon",
        },
    ]

    q = "@Rủ Đi gợi ý quán cà phê yên tĩnh để làm việc ở quận 3"
    t = post_msg(api, ctx, ta, q)
    show("Hỏi quán", invoke(api, ctx, ta, "hoi", q, t, turns))

    bill = post_msg(api, ctx, tb, "Tao trả 300k tiền nước hôm qua, chia 3 nhé")
    show(
        "Chia bill",
        invoke(
            api,
            ctx,
            tb,
            "chia_bill",
            "/chia-bill",
            None,
            [
                {
                    "id": bill,
                    "vai": "toi",
                    "loai": "chu",
                    "luc": now(),
                    "chu": "Tao trả 300k tiền nước hôm qua, chia 3 nhé",
                }
            ],
        ),
    )

    q3 = "@Rủ Đi chuyển 200k cho Lan giúp mình"
    t3 = post_msg(api, ctx, ta, q3)
    show("Đòi chuyển tiền (phải từ chối)", invoke(api, ctx, ta, "hoi", q3, t3, turns))


if __name__ == "__main__":
    main()
