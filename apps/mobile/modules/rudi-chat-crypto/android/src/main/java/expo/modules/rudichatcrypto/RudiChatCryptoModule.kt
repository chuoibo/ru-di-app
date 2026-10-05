package expo.modules.rudichatcrypto

import expo.modules.kotlin.exception.CodedException
import expo.modules.kotlin.modules.Module
import expo.modules.kotlin.modules.ModuleDefinition
import java.util.concurrent.Executors
import org.json.JSONObject

/**
 * MLS for chat v2 on Android (ADR-0057): one device identity at a time, every
 * call on one thread (the ratchet never races itself), and every change
 * sealed and persisted before its answer reaches JS -- so nothing is sent on
 * the network that the device could lose by crashing.
 */
class RudiChatCryptoModule : Module() {
  private val worker = Executors.newSingleThreadExecutor()
  private var handle = 0L
  private var vault: Vault? = null
  private var generation = -1L

  private fun <T> serial(block: () -> T): T = worker.submit<T> { block() }.get()

  private fun text(bytes: ByteArray?): String {
    val answer = bytes?.toString(Charsets.UTF_8) ?: throw CodedException("ERR_CHAT_CRYPTO", "null", null)
    if (answer.startsWith("{\"error\"")) {
      val code = JSONObject(answer).optString("error", "unknown")
      throw CodedException("ERR_CHAT_CRYPTO_" + code.uppercase(), code, null)
    }
    return answer
  }

  private fun live(): Long = if (handle != 0L) handle else throw CodedException("ERR_CHAT_CRYPTO_CLOSED", "closed", null)

  /** Seals and persists when the client's generation moved. */
  private fun persistIfChanged() {
    val v = vault ?: return
    val now = JSONObject(text(Native.call(live(), "generation".toByteArray(), "{}".toByteArray()))).getLong("generation")
    if (now == generation) return
    val args = JSONObject().put("wrapping_key", v.wrappingKey()).toString().toByteArray()
    val sealed = JSONObject(text(Native.call(live(), "seal".toByteArray(), args)))
    v.persist(sealed.getJSONObject("sealed").toString(), sealed.getJSONObject("anchor").toString())
    generation = now
  }

  private fun <T> mutating(block: () -> T): T = serial {
    val result = block()
    persistIfChanged()
    result
  }

  override fun definition() = ModuleDefinition {
    Name("RudiChatCrypto")

    /** Opens this device identity: resumes its sealed state, or starts it fresh. */
    AsyncFunction("open") { actor: String, device: String ->
      serial {
        if (handle != 0L) { Native.free(handle); handle = 0L }
        val context = appContext.reactContext ?: throw CodedException("ERR_CHAT_CRYPTO_CONTEXT", "context", null)
        val v = Vault(context, actor, device)
        var opened = 0L
        var resumed = false
        if (v.exists()) {
          val key = v.wrappingKey().toByteArray()
          for (anchor in v.anchors()) {
            opened = Native.resume(v.sealedState().toByteArray(), key, anchor.toByteArray())
            if (opened != 0L) break
          }
          key.fill(0)
          if (opened == 0L) throw CodedException("ERR_CHAT_CRYPTO_CHECKPOINT", "checkpoint", null)
          resumed = true
        } else {
          opened = Native.newClient(actor.toByteArray(), device.toByteArray())
          if (opened == 0L) throw CodedException("ERR_CHAT_CRYPTO_INVALID", "invalid", null)
        }
        handle = opened
        vault = v
        generation = -1L
        persistIfChanged()
        resumed
      }
    }

    AsyncFunction("identity") { -> serial { text(Native.identity(live())) } }

    AsyncFunction("createGroup") { conversation: String ->
      mutating { text(Native.createGroup(live(), conversation.toByteArray())) }
    }

    AsyncFunction("encrypt") { conversation: String, logical: String, operation: String ->
      mutating { text(Native.encrypt(live(), conversation.toByteArray(), logical.toByteArray(), operation.toByteArray())) }
    }

    AsyncFunction("receive") { envelope: String, roster: String? ->
      mutating { text(Native.receive(live(), envelope.toByteArray(), roster?.toByteArray())) }
    }

    AsyncFunction("call") { method: String, args: String ->
      mutating { text(Native.call(live(), method.toByteArray(), args.toByteArray())) }
    }

    /** Signs this device identity out: the handle, the sealed state and the keys go. */
    AsyncFunction("erase") { ->
      serial {
        if (handle != 0L) { Native.free(handle); handle = 0L }
        vault?.erase()
        vault = null
      }
    }

    OnDestroy {
      serial { if (handle != 0L) { Native.free(handle); handle = 0L } }
      worker.shutdown()
    }
  }
}
