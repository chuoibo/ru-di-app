"use strict";
const DANH_TINH_REFUSALS = {
    identity_key_missing: "Rủ Đi chưa sẵn sàng cho đăng nhập lúc này. Đây là lỗi phía Rủ Đi; thử lại sau ít phút.",
    rate_limited: "Thử lại sau một phút. Rủ Đi đang giới hạn số lần tra số điện thoại.",
    phone_not_mobile: "Chưa đúng dạng số di động Việt Nam.",
    phone_required: "Chưa gửi được số. Nhập lại rồi thử lần nữa.",
};

import { translatedAnonymous } from "../../dist-test/api.js";
import { chuanHoaSo } from "./phone-identity-legacy.mjs";
export async function layIdTuSo(raw) {
    const so = chuanHoaSo(raw);
    if (so === null) {
        // The number itself is not in this message on purpose: a thrown message
        // ends up in a console or in a bug report.
        throw new Error("Số điện thoại không hợp lệ, không thể tạo danh tính.");
    }
    // Anonymous on purpose, and it is the one route where that is unarguable:
    // this call is how the phone finds out which person id it has. There is
    // nobody to act as yet, because asking is what produces them.
    const wire = await translatedAnonymous(DANH_TINH_REFUSALS, "/identity/person-id", { method: "POST", body: { phone: so } });
    return wire.person_id;
}
