import assert from "node:assert/strict";
import test from "node:test";

import { choicesForRoute, toggleDisplayedBadge } from "../dist-test/rudi/ky-niem/journey-view.js";

test("selected route puts its ongoing ending first without hiding another eligible ending", () => {
  const choices = [
    { id: "many_turns", route_id: "dau_chan", eligible: true, earned: false },
    { id: "open_map", route_id: "dau_chan", eligible: false, earned: false },
    { id: "photos_remain", route_id: "ky_niem", eligible: true, earned: false },
  ];
  assert.deepEqual(choicesForRoute(choices, "dau_chan", "open_map").map((c) => c.id), ["open_map", "many_turns"]);
});

test("display selection accepts only three earned badges and may remove one", () => {
  const earned = ["first_checkin", "first_photo", "first_story", "first_together"];
  assert.deepEqual(toggleDisplayedBadge(["first_checkin", "first_photo", "first_story"], "first_together", earned), ["first_checkin", "first_photo", "first_story"]);
  assert.deepEqual(toggleDisplayedBadge(["first_checkin", "first_photo"], "first_checkin", earned), ["first_photo"]);
  assert.deepEqual(toggleDisplayedBadge([], "invented", earned), []);
});
