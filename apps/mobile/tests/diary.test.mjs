import test from 'node:test';
import assert from 'node:assert/strict';
import { initialPhotos, selectedBundle, movePage, togglePhoto } from '../dist-test/rudi/diary/api.js';
import { LOAI_SO, loaiSoCua, banTinhCua } from '../dist-test/rudi/so/ban-tinh.js';

test('hai quan hệ: pair chưa đồng thuận là hội bạn, không tự thành cặp đôi', () => {
  assert.deepEqual(LOAI_SO, ['hoi', 'doi']);
  assert.equal(loaiSoCua({kind:'pair'}, null), 'hoi');
  assert.equal(loaiSoCua({kind:'pair'}, {bat:false}), 'hoi');
  assert.equal(loaiSoCua({kind:'pair'}, {bat:true}), 'doi');
  assert.equal(loaiSoCua({kind:'group'}, {bat:true}), 'hoi');
  assert.equal(banTinhCua('hoi').tienHien, 'chia-bill');
});
const source = {title:'Synthetic trip',starts_on:'2026-01-01',ends_on:'2026-01-02',kind:'trip',places:[],excerpts:[],photos:[{id:'a',day:'2026-01-01',caption:''},{id:'b',day:'2026-01-03',caption:''}]};
test('gợi ý ảnh theo ngày nhưng gửi đúng lựa chọn, không tự gửi chat', () => {
  assert.deepEqual(initialPhotos(source), ['a']);
  const selected = selectedBundle(source, ['b'], ['Synthetic selected excerpt']);
  assert.deepEqual(selected.photos.map(p=>p.id), ['b']);
  assert.deepEqual(selected.excerpts,['Synthetic selected excerpt']);
  assert.deepEqual(source.excerpts, []);
  assert.deepEqual(selectedBundle(source,['a'],[]).excerpts,[]);
});
test('đủ chỗ mới thêm ảnh; luôn bỏ được lựa chọn cũ', () => {
  assert.deepEqual(togglePhoto(['a','b'],'c',2),['a','b']);
  assert.deepEqual(togglePhoto(['a','b'],'a',2),['b']);
  assert.deepEqual(togglePhoto(['a'],'b',2),['a','b']);
});
test('đổi thứ tự trang không thay đổi bản gốc', () => {
  const d={pages:[{heading:'first'},{heading:'second'}]};
  assert.deepEqual(movePage(d,1,0).pages.map(p=>p.heading),['second','first']);
  assert.equal(d.pages[0].heading,'first');
  assert.equal(movePage(d,0,-1),d);
});
