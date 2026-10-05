package expo.modules.rudichatcrypto

import android.content.Context
import android.security.keystore.KeyGenParameterSpec
import android.security.keystore.KeyProperties
import android.util.Base64
import java.io.File
import java.io.FileOutputStream
import java.security.KeyStore
import java.security.SecureRandom
import javax.crypto.Cipher
import javax.crypto.KeyGenerator
import javax.crypto.SecretKey
import javax.crypto.spec.GCMParameterSpec
import org.json.JSONObject

/**
 * Where one device identity's sealed MLS state lives (ADR-0057 §4.4).
 *
 * - The wrapping key is 32 random bytes, stored only encrypted under an
 *   Android Keystore AES-GCM key that never leaves the Keystore.
 * - Everything lives under noBackupFilesDir: never in an OS or cloud backup.
 * - The sealed state is replaced atomically (temp file, fsync, rename) and the
 *   anchor holds {current, next}: written as next before the state, promoted
 *   to current after, so a crash between the two leaves an openable pair, and
 *   no older state than the previous one ever opens.
 */
internal class Vault(context: Context, actor: String, device: String) {
  private val dir = File(context.noBackupFilesDir, "rudi-chat/$actor/$device").apply { mkdirs() }
  private val stateFile = File(dir, "state.json")
  private val anchorFile = File(dir, "anchor.json")
  private val keyFile = File(dir, "key.bin")
  private val alias = "rudi-chat-wrap-$actor-$device"

  fun exists(): Boolean = stateFile.exists() && anchorFile.exists() && keyFile.exists()

  /** Where this device identity's room records live (RoomLog), beside its state. */
  val roomsDir: File get() = File(dir, "rooms")

  private fun keystoreKey(): SecretKey {
    val store = KeyStore.getInstance("AndroidKeyStore").apply { load(null) }
    (store.getKey(alias, null) as? SecretKey)?.let { return it }
    val generator = KeyGenerator.getInstance(KeyProperties.KEY_ALGORITHM_AES, "AndroidKeyStore")
    generator.init(
      KeyGenParameterSpec.Builder(alias, KeyProperties.PURPOSE_ENCRYPT or KeyProperties.PURPOSE_DECRYPT)
        .setBlockModes(KeyProperties.BLOCK_MODE_GCM)
        .setEncryptionPaddings(KeyProperties.ENCRYPTION_PADDING_NONE)
        .setKeySize(256)
        .build()
    )
    return generator.generateKey()
  }

  /** The base64 wrapping key, created once per device identity. */
  fun wrappingKey(): String {
    val key = keystoreKey()
    if (keyFile.exists()) {
      val blob = keyFile.readBytes()
      val cipher = Cipher.getInstance("AES/GCM/NoPadding")
      cipher.init(Cipher.DECRYPT_MODE, key, GCMParameterSpec(128, blob.copyOfRange(0, 12)))
      val raw = cipher.doFinal(blob.copyOfRange(12, blob.size))
      return Base64.encodeToString(raw, Base64.NO_WRAP).also { raw.fill(0) }
    }
    val raw = ByteArray(32).also { SecureRandom().nextBytes(it) }
    val cipher = Cipher.getInstance("AES/GCM/NoPadding")
    cipher.init(Cipher.ENCRYPT_MODE, key)
    writeAtomic(keyFile, cipher.iv + cipher.doFinal(raw))
    return Base64.encodeToString(raw, Base64.NO_WRAP).also { raw.fill(0) }
  }

  /** The anchors this vault accepts: current, and next if a write was interrupted. */
  fun anchors(): List<String> {
    if (!anchorFile.exists()) return emptyList()
    val json = JSONObject(anchorFile.readText())
    return listOfNotNull(json.optString("current", "").ifEmpty { null }, json.optString("next", "").ifEmpty { null })
  }

  fun sealedState(): String = stateFile.readText()

  /** Persists a sealed state and its anchor, crash-safe in this order. */
  fun persist(sealed: String, anchor: String) {
    val current = anchors().firstOrNull()
    writeAtomic(anchorFile, JSONObject().put("current", current ?: anchor).put("next", anchor).toString().toByteArray())
    writeAtomic(stateFile, sealed.toByteArray())
    writeAtomic(anchorFile, JSONObject().put("current", anchor).toString().toByteArray())
  }

  /** Signing out of this device identity: everything goes, the room records and the Keystore key too. */
  fun erase() {
    roomsDir.deleteRecursively()
    listOf(stateFile, anchorFile, keyFile).forEach { it.delete() }
    runCatching { KeyStore.getInstance("AndroidKeyStore").apply { load(null) }.deleteEntry(alias) }
  }

  private fun writeAtomic(target: File, bytes: ByteArray) {
    val temp = File(target.parentFile, target.name + ".tmp")
    FileOutputStream(temp).use { out ->
      out.write(bytes)
      out.fd.sync()
    }
    if (!temp.renameTo(target)) throw IllegalStateException("vault_write_failed")
  }
}
