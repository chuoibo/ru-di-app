/**
 * Config plugin: keep the screen the person is on when they change the text
 * size or the display size.
 *
 * QA 23/09: changing the system font size while in a direct conversation
 * dropped the person back on Khám phá. Android RECREATES an activity for every
 * configuration change it does not list in `android:configChanges`, and Expo's
 * template lists `uiMode` (dark mode) but neither `fontScale` nor `density`.
 * A recreated React Native activity starts the JavaScript root again, so the
 * router's stack is gone.
 *
 * Listing both hands the change to React Native instead:
 * `ReactActivity.onConfigurationChanged` updates the display metrics and emits
 * the dimension change `useWindowDimensions` reads, and every screen that lays
 * itself out by `fontScale` re-renders where it stands.
 */
const { withAndroidManifest, AndroidConfig } = require("expo/config-plugins");

const THEM = ["fontScale", "density"];

/**
 * Pure transform on the parsed AndroidManifest.xml: add the two changes to the
 * main activity's `android:configChanges`, once, keeping what is there.
 */
function giuManKhiDoiCoChu(manifest) {
  const activity = AndroidConfig.Manifest.getMainActivityOrThrow(manifest);
  const co = (activity.$["android:configChanges"] ?? "").split("|").filter(Boolean);
  for (const muc of THEM) if (!co.includes(muc)) co.push(muc);
  activity.$["android:configChanges"] = co.join("|");
  return manifest;
}

function withGiuManKhiDoiCoChu(config) {
  return withAndroidManifest(config, (c) => {
    c.modResults = giuManKhiDoiCoChu(c.modResults);
    return c;
  });
}

module.exports = withGiuManKhiDoiCoChu;
module.exports.giuManKhiDoiCoChu = giuManKhiDoiCoChu;
module.exports.THEM = THEM;
