"""The OTP door on the real repository, real HTTP, `prod` auth mode.

What only PostgreSQL can show: the challenge row persists its attempt count
across requests, `account_identities` refuses a second row for the same proof
and refuses a provider it does not know, and a person a friend already named by
phone (a `people` row at the derived id plus an INVITED membership) signs in to
that same row and sees the invitation in `contexts`.

No telephone number is spelled out whole; the repo guard is right to refuse it.
"""

from __future__ import annotations

import uuid
from datetime import UTC, datetime, timedelta

import anyio
import httpx
import pytest
from sqlalchemy import select
from sqlalchemy.exc import IntegrityError
from sqlalchemy.orm import Session

from app.api.deps import get_repository
from app.api.main import create_app
from app.api.person_identity import KEY_ENV_VAR, canonical_mobile, derive_person_id
from app.api.repository import SqlAlchemyApiRepository
from app.api.sms import SmsDeliveryError
from app.db.models import (
    AccountIdentity,
    Context,
    Membership,
    MembershipOrigin,
    MembershipRole,
    MembershipState,
    OtpChallenge,
    Person,
)

pytestmark = pytest.mark.postgres

KEY = "otp-postgres-test-key-for-person-id-derivation"
PHONE = "0912" + "345678"


class RecordingSender:
    def __init__(self):
        self.sent = []

    def send_otp(self, *, canonical_phone, code, challenge_id):
        del canonical_phone
        self.sent.append((code, challenge_id))


@pytest.fixture(autouse=True)
def key(monkeypatch):
    monkeypatch.setenv(KEY_ENV_VAR, KEY)


def _app(session: Session, sender):
    app = create_app(auth_mode="prod")
    app.dependency_overrides[get_repository] = lambda: SqlAlchemyApiRepository(session)
    app.state.sms_sender = sender
    app.state.otp_debug_code = None
    return app


def _call(app, method, path, *, json=None, token=None):
    async def go():
        headers = {"Authorization": f"Bearer {token}"} if token else {}
        async with httpx.AsyncClient(
            transport=httpx.ASGITransport(app=app), base_url="http://testserver"
        ) as client:
            return await client.request(method, path, json=json, headers=headers)

    return anyio.run(go)


def _login(app, sender, phone=PHONE):
    requested = _call(app, "POST", "/auth/otp/request", json={"phone": phone})
    assert requested.status_code == 202, requested.text
    code, challenge_id = sender.sent[-1]
    verified = _call(
        app,
        "POST",
        "/auth/otp/verify",
        json={"challenge_id": str(challenge_id), "phone": phone, "code": code},
    )
    assert verified.status_code == 201, verified.text
    return verified.json(), challenge_id


def test_a_new_number_becomes_a_person_a_session_and_one_identity_row(postgres_session):
    sender = RecordingSender()
    app = _app(postgres_session, sender)
    body, challenge_id = _login(app, sender)

    assert body["issued_via"] == "otp" and body["is_new_person"] is True
    row = postgres_session.get(OtpChallenge, challenge_id)
    assert row.consumed_at is not None and row.attempts == 1
    identities = postgres_session.scalars(select(AccountIdentity)).all()
    assert len(identities) == 1 and identities[0].provider == "phone"
    assert identities[0].person_id == uuid.UUID(body["person_id"])
    # The bearer is real on the real repository too.
    mine = _call(app, "GET", "/people/me/contexts", token=body["token"])
    assert mine.status_code == 200 and mine.json()["contexts"] == []


def test_a_friend_named_this_number_first_so_the_login_lands_in_that_row(
    postgres_session,
):
    owner = Person(id=uuid.uuid4(), display_name="Minh Anh")
    named_id = derive_person_id(canonical_mobile(PHONE), KEY.encode())
    named = Person(id=named_id, display_name="Hà")
    postgres_session.add_all([owner, named])
    postgres_session.flush()
    context = Context(
        id=uuid.uuid4(), display_name="Hội đi Đà Lạt", created_by_id=owner.id
    )
    postgres_session.add(context)
    postgres_session.flush()
    invited = Membership(
        id=uuid.uuid4(),
        context_id=context.id,
        person_id=named_id,
        state=MembershipState.INVITED,
        role=MembershipRole.MEMBER,
        origin=MembershipOrigin.NAMED,
        invited_by_id=owner.id,
    )
    postgres_session.add(invited)
    postgres_session.flush()

    sender = RecordingSender()
    body, _ = _login(_app(postgres_session, sender), sender)

    assert body["person_id"] == str(named_id)
    assert body["is_new_person"] is False
    assert body["profile"]["display_name"] == "Hà"
    assert [c["my_state"] for c in body["contexts"]] == ["invited"]
    assert body["contexts"][0]["membership_id"] == str(invited.id)


def test_wrong_guesses_are_counted_in_the_row_not_in_memory(postgres_session):
    sender = RecordingSender()
    app = _app(postgres_session, sender)
    requested = _call(app, "POST", "/auth/otp/request", json={"phone": PHONE})
    assert requested.status_code == 202
    code, challenge_id = sender.sent[-1]
    bad = "000000" if code != "000000" else "111111"
    for _ in range(2):
        r = _call(
            app,
            "POST",
            "/auth/otp/verify",
            json={"challenge_id": str(challenge_id), "phone": PHONE, "code": bad},
        )
        assert r.status_code == 422
    assert postgres_session.get(OtpChallenge, challenge_id).attempts == 2


def test_the_database_refuses_a_second_binding_of_one_proof_and_an_unknown_provider(
    postgres_session,
):
    a = Person(id=uuid.uuid4(), display_name="A")
    b = Person(id=uuid.uuid4(), display_name="B")
    postgres_session.add_all([a, b])
    postgres_session.flush()
    now = datetime.now(UTC)
    postgres_session.add(
        AccountIdentity(
            person_id=a.id,
            provider="phone",
            subject="abc",
            created_at=now,
            last_login_at=now,
        )
    )
    postgres_session.flush()
    postgres_session.begin_nested()
    with pytest.raises(IntegrityError):
        postgres_session.add(
            AccountIdentity(
                person_id=b.id,
                provider="phone",
                subject="abc",
                created_at=now,
                last_login_at=now,
            )
        )
        postgres_session.flush()
    postgres_session.rollback()
    postgres_session.begin_nested()
    with pytest.raises(IntegrityError):
        postgres_session.add(
            AccountIdentity(
                person_id=b.id,
                provider="zalo",
                subject="x",
                created_at=now,
                last_login_at=now,
            )
        )
        postgres_session.flush()
    postgres_session.rollback()


def test_a_failed_delivery_leaves_a_consumed_challenge_behind(postgres_session):
    class Failing:
        def send_otp(self, **kwargs):
            raise SmsDeliveryError("URLError")

    app = _app(postgres_session, Failing())
    response = _call(app, "POST", "/auth/otp/request", json={"phone": PHONE})
    assert response.status_code == 503
    rows = postgres_session.scalars(select(OtpChallenge)).all()
    assert len(rows) == 1 and rows[0].consumed_at is not None


def test_a_number_that_deleted_and_came_back_is_findable_by_phone_again(
    postgres_session, monkeypatch
):
    """ADR-0023 §2.2.2 trên ĐƯỜNG THẬT, không mô phỏng.

    `person_id` của tài khoản OTP suy ra từ số, nên «id suy ra» và «id thật»
    trùng nhau suốt nhiều tháng. Chúng thôi trùng đúng lúc ai đó bấm nút xoá:
    `verify_otp` mint uuid4 mới thay vì hồi sinh một hàng đã ẩn danh.

    Ca này lái cả vòng bằng chính các route của sản phẩm — xin mã, xác minh,
    xoá, xin mã lại, xác minh lại, rồi tra số — chứ không tự tay ghi hàng
    `account_identities`. Bản đầu của ca này có ghi tay, và một reviewer chỉ
    ra đúng chỗ yếu: nó sẽ vẫn xanh vào ngày `verify_otp` đổi thứ nó ghi.

    Nếu `find_person_by_phone` chỉ đọc phép suy ra thì mọi người từng xoá tài
    khoản biến mất vĩnh viễn khỏi «Thêm bạn», và câu từ chối GIỐNG HỆT câu
    «chưa có ai dùng số này» — nên lỗi tự giấu mình.
    """
    dong_ho = {"luc": datetime(2030, 8, 27, 12, tzinfo=UTC)}
    monkeypatch.setattr("app.api.service._now", lambda: dong_ho["luc"])
    sender = RecordingSender()
    app = _app(postgres_session, sender)
    phone = "0913" + "111222"

    # Vòng đời một.
    truoc, _ = _login(app, sender, phone)
    id_cu = uuid.UUID(truoc["person_id"])

    # Một người khác đi tìm, qua đúng route mà màn «Thêm bạn» dùng.
    nguoi_tra, _ = _login(app, sender, "0914" + "333444")

    def tra_so():
        return _call(
            app,
            "POST",
            "/friends/lookup",
            json={"phone": phone},
            token=nguoi_tra["token"],
        )

    thay = tra_so()
    assert thay.status_code == 200, thay.text
    assert thay.json()["person_id"] == str(id_cu)

    # Kết thúc tài khoản, rồi cùng số ấy quay lại.
    SqlAlchemyApiRepository(postgres_session).erase_person(id_cu, now=dong_ho["luc"])
    postgres_session.flush()
    # Nhịp gửi lại mã là 60 giây theo đồng hồ máy chủ; nhích đồng hồ thay vì chờ.
    dong_ho["luc"] = dong_ho["luc"] + timedelta(minutes=5)
    sau, _ = _login(app, sender, phone)
    id_moi = uuid.UUID(sau["person_id"])
    assert id_moi != id_cu, "đăng ký lại phải là một người MỚI, không hồi sinh hàng cũ"

    lai_thay = tra_so()
    assert lai_thay.status_code == 200, (
        "người đăng ký lại phải tìm được: 404 ở đây nghĩa là ai bấm nút xoá thì "
        "biến mất khỏi «Thêm bạn» vĩnh viễn, và câu từ chối không nói ra điều đó"
    )
    assert lai_thay.json()["person_id"] == str(id_moi)
