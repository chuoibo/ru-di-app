package expo.modules.rudichatcrypto

/** The JNI shim (src/main/cpp). Strings cross as UTF-8 bytes; a null answer is a refused call. */
internal object Native {
  init {
    System.loadLibrary("rudi_chat_crypto_ffi")
    System.loadLibrary("rudi_chat_crypto_jni")
  }

  @JvmStatic external fun newClient(actor: ByteArray, device: ByteArray): Long
  @JvmStatic external fun resume(sealed: ByteArray, key: ByteArray, anchor: ByteArray): Long
  @JvmStatic external fun free(handle: Long)
  @JvmStatic external fun identity(handle: Long): ByteArray?
  @JvmStatic external fun createGroup(handle: Long, conversation: ByteArray): ByteArray?
  @JvmStatic external fun encrypt(handle: Long, conversation: ByteArray, logical: ByteArray, operation: ByteArray): ByteArray?
  @JvmStatic external fun receive(handle: Long, envelope: ByteArray, roster: ByteArray?): ByteArray?
  @JvmStatic external fun call(handle: Long, method: ByteArray, args: ByteArray): ByteArray?
}
