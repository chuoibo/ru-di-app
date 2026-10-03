/**
 * The (category, tags) pairs a place sketch is drawn from: twelve varied
 * samples, kept as test data after the app's sample catalogue was removed
 * (2026-10-03). The sketch machine itself is live: a place without a photo
 * gets one. Names are labels for assertion messages only.
 */

/** Catalogue category id per sample category, as the live screens map it. */
export const LOAI_MAU = {
  "Quán ăn": "quan-an-local",
  Cafe: "cafe",
  "Vui chơi": "vui-choi",
  "Đi chơi đêm": "di-choi-dem",
};

export const PLACES = [
  { name: "Tiệm Nướng Xóm Lèo", tags: ["View đẹp", "Chill", "Nhóm đông"], category: "Quán ăn" },
  { name: "Bánh căn Lệ", tags: ["Món local", "Bình dân"], category: "Quán ăn" },
  { name: "Lẩu gà lá é Gốc", tags: ["Nhóm đông", "Lẩu"], category: "Quán ăn" },
  { name: "Still Cafe Đà Lạt", tags: ["Nhẹ nhàng", "Cà phê", "Ngoài trời"], category: "Cafe" },
  { name: "Tiệm trà Sương", tags: ["Trà", "Ngoài trời"], category: "Cafe" },
  { name: "The Coffee Hill", tags: ["Cà phê", "View đẹp"], category: "Cafe" },
  { name: "Puppy Farm Đà Lạt", tags: ["Chụp ảnh", "Hoa", "Ngoài trời"], category: "Vui chơi" },
  { name: "Đồi Thiên Phúc Đức", tags: ["Săn mây", "Ngoài trời"], category: "Vui chơi" },
  { name: "Thung lũng Tình Yêu", tags: ["Hoa", "Chụp ảnh"], category: "Vui chơi" },
  { name: "Chợ Đêm Đà Lạt", tags: ["Món local", "Đi đêm", "Nhộn nhịp"], category: "Đi chơi đêm" },
  { name: "Phố đi bộ đêm", tags: ["Đi đêm", "Nhộn nhịp"], category: "Đi chơi đêm" },
  { name: "Hồ Tuyền Lâm về đêm", tags: ["BBQ", "Đi đêm"], category: "Đi chơi đêm" },
];
