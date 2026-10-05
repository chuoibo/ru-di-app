cmake_minimum_required(VERSION 3.22)
project(rudi_chat_crypto_jni C)
# The Rust library, built per ABI into ../jniLibs by scripts/build-android.sh.
add_library(rudi_chat_crypto_ffi SHARED IMPORTED)
set_target_properties(rudi_chat_crypto_ffi PROPERTIES IMPORTED_LOCATION
  ${CMAKE_CURRENT_SOURCE_DIR}/../jniLibs/${ANDROID_ABI}/librudi_chat_crypto_ffi.so)
add_library(rudi_chat_crypto_jni SHARED rudi_chat_crypto_jni.c)
target_include_directories(rudi_chat_crypto_jni PRIVATE ${CMAKE_CURRENT_SOURCE_DIR})
target_link_libraries(rudi_chat_crypto_jni rudi_chat_crypto_ffi)
