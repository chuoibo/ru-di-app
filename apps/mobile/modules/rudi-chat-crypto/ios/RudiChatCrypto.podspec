Pod::Spec.new do |s|
  s.name           = 'RudiChatCrypto'
  s.version        = '1.0.0'
  s.summary        = 'MLS for chat v2 (ADR-0057) over packages/chat-crypto-ffi'
  s.author         = 'Rủ Đi'
  s.homepage       = 'https://example.invalid/rudi'
  s.license        = { :type => 'Proprietary' }
  s.platforms      = { :ios => '15.1' }
  s.source         = { :git => '' }
  s.static_framework = true
  s.dependency 'ExpoModulesCore'
  s.source_files   = '*.{swift,h}'
  s.public_header_files = 'rudi_chat_crypto.h'
  # Built by ../scripts/build-ios.sh (macOS: device + simulator slices).
  s.vendored_frameworks = 'RudiChatCryptoFFI.xcframework'
end
