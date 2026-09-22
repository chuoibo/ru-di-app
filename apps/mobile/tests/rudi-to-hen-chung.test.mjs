import assert from 'node:assert/strict';
import test from 'node:test';
import { docKhoiNhap, ngayKieuViet, ngayVeISO } from '../dist-test/rudi/chat/to-hen-chung.js';
import { loiBinhChon, loiBinhChonTheoO } from '../dist-test/rudi/chat/ban-nhap-cong-cu.js';

// The shared sheet rides inside an ordinary itinerary card, so the only thing
// that distinguishes it from an AI suggestion is the `draft` block. Reading it
// wrongly would either hide the group's own sheet or offer to edit a card the
// group may only accept.
test('khối bản nháp chỉ được đọc khi thẻ thật sự mang nó', () => {
  const sheet = docKhoiNhap({
    kind: 'itinerary',
    payload: {
      title: 'Tờ hẹn',
      stops: [],
      draft: { id: 'd1', revision: 3, status: 'open', source_vote_id: 'v1', starts_on: '2026-10-03', ends_on: null, headcount: 4, budget_per_person_vnd: 500000 },
    },
  });
  assert.equal(sheet?.id, 'd1');
  assert.equal(sheet?.revision, 3);
  assert.equal(sheet?.status, 'open');
  assert.equal(sheet?.source_vote_id, 'v1');
  assert.equal(sheet?.ends_on, null);

  // An AI card has no draft block and must not become editable by accident.
  assert.equal(docKhoiNhap({ kind: 'itinerary', payload: { title: 'AI', stops: [] } }), null);
  // Nor may a malformed block half-open the door.
  assert.equal(docKhoiNhap({ payload: { draft: { id: 'd1' } } }), null);
  assert.equal(docKhoiNhap({ payload: { draft: { id: 'd1', revision: 1, status: 'dang-nghi' } } }), null);
  assert.equal(docKhoiNhap(null), null);
  assert.equal(docKhoiNhap('itinerary'), null);
});

// The wire stays ISO because every comparison depends on it. Only the spelling
// a person types and reads changes.
test('ngày đọc kiểu Việt nhưng gửi đi vẫn là ISO', () => {
  assert.equal(ngayKieuViet('2026-09-20'), '20/09/2026');
  assert.equal(ngayKieuViet(null), '');
  assert.equal(ngayKieuViet('khong-phai-ngay'), 'khong-phai-ngay');

  assert.equal(ngayVeISO('20/09/2026'), '2026-09-20');
  assert.equal(ngayVeISO(' 5/9/2026 '), '2026-09-05');
  // A date the calendar does not have must not be invented into a nearby one.
  assert.equal(ngayVeISO('31/02/2026'), null);
  assert.equal(ngayVeISO('2026-09-20'), null);
  assert.equal(ngayVeISO(''), null);
});

// One sentence under the whole form said something was wrong but not where, so
// fixing it meant re-reading every box (reviewer C4, heuristic 9).
test('lỗi bình chọn chỉ vào đúng ô có lỗi', () => {
  const trong = loiBinhChonTheoO('', ['', '']);
  assert.match(trong.question, /câu hỏi/i);
  assert.equal(trong.choices.filter(Boolean).length, 1, 'chỉ ô trống đầu tiên được chỉ tên');

  const trung = loiBinhChonTheoO('Đi đâu?', ['Bờ hồ', 'bờ hồ']);
  assert.equal(trung.question, null);
  assert.equal(trung.choices[0], null, 'ô đầu không có lỗi; ô sau mới là ô trùng');
  assert.match(trung.choices[1], /Trùng với lựa chọn 1/);

  const gach = loiBinhChonTheoO('Đi đâu?', ['A|B', 'C']);
  assert.match(gach.choices[0], /dấu \|/);
  assert.equal(gach.choices[1], null);

  const hoi = loiBinhChonTheoO('Đi? ở đâu?', ['A', 'B']);
  assert.match(hoi.question, /cuối câu/);

  const sach = loiBinhChonTheoO('Đi đâu?', ['Bờ hồ', 'Quán nhỏ']);
  assert.equal(sach.question, null);
  assert.deepEqual(sach.choices, [null, null]);
});

// The summary line the live region announces has to agree with the per-field
// answer, or a screen reader and the screen would describe different forms.
test('dòng tóm tắt và lỗi từng ô không được nói khác nhau', () => {
  for (const [question, choices] of [
    ['', ['', '']],
    ['Đi đâu?', ['Bờ hồ', 'bờ hồ']],
    ['Đi | đâu?', ['A', 'B']],
    ['Đi? ở đâu?', ['A', 'B']],
    ['Đi đâu?', ['Bờ hồ', 'Quán nhỏ']],
  ]) {
    const tomTat = loiBinhChon(question, choices);
    const theoO = loiBinhChonTheoO(question, choices);
    const coLoi = theoO.question !== null || theoO.choices.some(Boolean);
    assert.equal(
      tomTat !== null,
      coLoi,
      `«${question}» + ${JSON.stringify(choices)}: tóm tắt nói ${tomTat !== null}, từng ô nói ${coLoi}`,
    );
  }
});

// Caught by looking at a real screenshot, not by any test that existed: every
// stop on a hand-written sheet was dropped because it names no catalogue place,
// the stop list came back empty, and the card fell through to
// «Một thẻ bản này chưa hiển thị được». tsc, the unit tests and the detector
// were all green while the screen showed a fallback.
test('chặng hội tự gõ không có địa điểm catalogue vẫn dựng được thẻ', async () => {
  const { docTheAi } = await import('../dist-test/rudi/chat/tin-song.js');
  const the = docTheAi({
    kind: 'itinerary',
    payload: {
      title: 'Đà Lạt hai ngày',
      stops: [
        { time_text: '08:00', note: '', place: { name: 'Chợ Đà Lạt' } },
        { time_text: '11:30', note: '', place: { name: 'Quán cà phê đồi' } },
      ],
      draft: { id: 'd1', revision: 2, status: 'open', source_vote_id: 'v1', starts_on: null, ends_on: null, headcount: 6, budget_per_person_vnd: null },
    },
  });
  assert.equal(the.loai, 'itinerary', 'thẻ phải là lịch trình, không rơi xuống «khac»');
  assert.equal(the.the.chang.length, 2, 'cả hai chặng phải sống sót qua bước đọc');
  assert.equal(the.the.chang[0].diaDiem.ten, 'Chợ Đà Lạt');
  assert.equal(the.nhapChung?.id, 'd1');
  assert.equal(the.nhapChung?.status, 'open');
});

// The other half of the same rule: a suggestion still has to name a real place,
// or there is nothing to open when somebody taps it.
test('thẻ gợi ý vẫn đòi địa điểm có id', async () => {
  const { docTheAi } = await import('../dist-test/rudi/chat/tin-song.js');
  const the = docTheAi({
    kind: 'places',
    payload: { places: [{ name: 'Không có id' }] },
  });
  assert.equal(the.loai, 'khac', 'gợi ý thiếu id phải bị từ chối như trước');
});
