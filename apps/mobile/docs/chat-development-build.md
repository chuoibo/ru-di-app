# Cài bản development để kiểm thử chat trên điện thoại

Expo Go chưa đủ cho đợt chat này. Repo đã dùng `expo-dev-client`, Google Sign-In và
MapLibre native; module MLS/Rust và ghi âm sẽ cần build native riêng. **Bản dev
client đang có chưa chứa MLS, chưa có voice và chưa chứng minh chat E2EE.**

## Android: có thể build tại máy Linux, không cần tài khoản EAS

<!-- repo-guard: allow=long-number reason=public-android-toolchain-version -->
Máy làm việc có SDK Android 35/36, NDK `27.1.12297006`, CMake `3.22.1` và
Gradle wrapper. Java hệ thống chỉ có runtime; JDK 21 đầy đủ đã được đặt riêng
ngoài repo tại `/tmp/rudi-chat-jdk21/root/usr/lib/jvm/java-21-openjdk-amd64`.
Package JDK từ Ubuntu noble-updates được đối chiếu SHA256 theo metadata apt;
không đổi Java mặc định của máy. Build x86_64 với JDK này đã thành công ngày
21-09 (539 task, 37 task thực thi). Khi máy khác hoặc thư mục tạm đã dọn, phải
cài JDK 21 đầy đủ và đổi `JAVA_HOME` tương ứng, kiểm cả `java` lẫn `javac`.

Chưa có điện thoại Android thật kết nối trong lượt kiểm; thiết bị là emulator.
APK x86_64 này không phải artifact cho điện thoại ARM. Đường build output local
là `android/app/build/outputs/apk/debug/app-debug.apk`.

1. Bật Developer options và USB debugging trên điện thoại, cắm USB, chấp nhận
   dấu vân tay máy tính trên điện thoại. Chọn đúng serial, không chạy nhầm emulator.
2. Trong `apps/mobile`, cài dependency theo lockfile nếu máy chưa có:

   ```sh
   npm ci
   ```

3. Dùng API thử nghiệm truy cập được từ điện thoại, với tài khoản và nội dung giả.
   Địa chỉ dưới đây chỉ là chỗ thay bằng URL thật của môi trường thử nghiệm:

   ```sh
   JAVA_HOME=/tmp/rudi-chat-jdk21/root/usr/lib/jvm/java-21-openjdk-amd64 \
   ANDROID_HOME=/home/lakiet/Android/Sdk \
   EXPO_NO_DOTENV=1 EXPO_PUBLIC_API_URL=https://api-test.example.invalid \
     npx expo run:android --device
   ```

   Lệnh chọn thiết bị và dựng development build cho thiết bị được chọn. Đường
   Android native đã được tạo trong môi trường làm việc hiện tại, không được
   Git theo dõi; checkout mới sẽ được Expo tự prebuild khi cần. Không dùng APK
   x86_64 nếu điện thoại chỉ chạy
   ARM; để Gradle/Expo dựng kiến trúc phù hợp.
4. Những lần chỉ thay TypeScript có thể mở lại dev client rồi nạp Metro:

   ```sh
   EXPO_NO_DOTENV=1 EXPO_PUBLIC_API_URL=https://api-test.example.invalid \
     npx expo start --dev-client --lan
   ```

   Máy tính và điện thoại phải cùng mạng và truy cập được cổng Metro. Với USB,
   có thể dùng `adb -s SERIAL reverse tcp:PORT tcp:PORT` cho **đúng cổng Metro/API
   thử nghiệm** rồi mở URL localhost tương ứng trong dev client. `127.0.0.1` trên
   điện thoại là điện thoại, không tự trỏ về máy Linux.

`EXPO_NO_DOTENV=1` tránh tự nạp cấu hình local ngoài ý muốn. Mọi biến
`EXPO_PUBLIC_*` đều có thể đọc từ ứng dụng; không đặt bí mật ở đó. Không gửi ảnh,
đoạn chat hay tài khoản thật vào môi trường thử nghiệm.

## iPhone: cần build iOS đã ký

Có hai đường, chưa đường nào được chứng minh trên môi trường này:

- **Mac + Xcode:** mở checkout trên Mac, cài dependency, cấu hình signing trong
  Xcode và chọn iPhone, rồi dùng `npx expo run:ios --device`. Repo hiện chưa có
  thư mục `ios/`; Expo sẽ tạo native project khi cần. Khả năng signing phụ thuộc
  Apple account/capability thực tế; không coi đây là bước đã hoàn tất.
- **EAS Build:** thiết lập tài khoản Expo, project EAS và Apple signing trước.
  `eas.json` đã có profile `development` với `developmentClient: true` và
  `distribution: internal`. Sau khi CLI/login/project/signing được thiết lập,
  đăng ký iPhone theo quy trình EAS rồi chạy
  `eas build --platform ios --profile development`. Cài build được cấp cho
  thiết bị đó và kết nối Metro của project. Profile `development-simulator`
  dành cho simulator, không cài được lên iPhone thật.

Máy Linux hiện không có `xcodebuild`; chưa thấy EAS CLI, local Expo auth record
hoặc `credentials.json`. Những điều này **không chứng minh** không có Mac runner,
tài khoản Apple hoặc credentials ở dịch vụ từ xa. Cần kiểm chứng các nguồn đó
khi chuẩn bị build iOS, không cần đưa mật khẩu hoặc khoá ký vào repo/chat.

## Cổng kiểm thử sau khi cài

- Trước tiên ghi commit, cấu hình native, API thử nghiệm, phiên bản build,
  thiết bị/OS và nguồn bundle. Hai máy dùng hai account giả khác nhau; thêm
  account thứ ba để kiểm group, cùng một account trên hai máy để kiểm đa thiết bị.
- Android/iOS phải nạp đúng source; ảnh native và log HTTP kiểm thứ đã diễn ra,
  không thay bằng web export. Hàng chờ, đổi mạng, mất ACK và app bị đóng cần ca
  riêng; production backend thật là cổng khác với server fixture trả dữ liệu giả.
- Thay native dependency/plugin thì build và cài lại. Hiện app.json chặn
  `RECORD_AUDIO` và chưa có `NSMicrophoneUsageDescription`; voice phải có thiết kế
  quyền, implementation và build mới trước khi kiểm thử. Không mở quyền chỉ để
  làm nút chưa hoạt động trông như đã sẵn sàng.
- Sau khi tích hợp MLS, thiếu module native hoặc thiếu khoá phải báo chưa sẵn sàng;
  tuyệt đối không chuyển chat mã hoá sang endpoint plaintext. Expo Go và dev
  client cũ đều không phải bằng chứng MLS chạy được.

Hướng dẫn nền tảng: [Expo development builds](https://docs.expo.dev/develop/development-builds/introduction/).
