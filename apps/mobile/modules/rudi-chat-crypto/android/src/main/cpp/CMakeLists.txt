cmake_minimum_required(VERSION 3.22)
project(rudi_chat_crypto_jni C)
# The Rust library, built per ABI into ../jniLibs by scripts/build-android.sh.
# Linked by name (IMPORTED_NO_SONAME): without it the shim records the build
# machine's absolute path as DT_NEEDED and the phone's loader never finds the
# library (seen on the emulator 05/10: dlopen failed, ERR_UNEXPECTED).
add_library(rudi_chat_crypto_ffi SHARED IMPORTED)
set_target_properties(rudi_chat_crypto_ffi PROPERTIES IMPORTED_LOCATION
  ${CMAKE_CURRENT_SOURCE_DIR}/../jniLibs/${ANDROID_ABI}/librudi_chat_crypto_ffi.so
  IMPORTED_NO_SONAME TRUE)
add_library(rudi_chat_crypto_jni SHARED rudi_chat_crypto_jni.c)
target_include_directories(rudi_chat_crypto_jni PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})
target_link_libraries(rudi_chat_crypto_jni rudi_chat_crypto_ffi)
