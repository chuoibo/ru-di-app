package expo.modules.rudifontscale

import android.content.ComponentCallbacks
import android.content.Context
import android.content.res.Configuration
import expo.modules.kotlin.modules.Module
import expo.modules.kotlin.modules.ModuleDefinition

class RudiFontScaleModule : Module() {
  private val gate = Any()
  private var context: Context? = null
  private var active = false
  private var unavailableReason = "module-inactive"
  private var lastNotified: Snapshot? = null

  private data class Snapshot(val fontScale: Float?, val reason: String = "") {
    fun toMap(): Map<String, Any?> = if (fontScale != null) {
      mapOf("status" to "available", "fontScale" to fontScale.toDouble())
    } else {
      mapOf("status" to "unavailable", "reason" to reason)
    }
  }

  private val callbacks = object : ComponentCallbacks {
    override fun onConfigurationChanged(newConfig: Configuration) {
      synchronized(gate) {
        if (!active) return
        val next = readSnapshotLocked()
        if (next == lastNotified) return
        lastNotified = next
        // This event is only a notification; JS rereads the current resources.
        sendEvent("onFontScaleChanged", emptyMap<String, Any?>())
      }
    }

    override fun onLowMemory() = Unit
  }

  // The getter and callbacks read the same current application resources.
  private fun readSnapshotLocked(): Snapshot {
    if (!active) return Snapshot(null, unavailableReason)
    val heldContext = context ?: return Snapshot(null, "context-unavailable")
    val scale = try {
      heldContext.resources.configuration.fontScale
    } catch (_: RuntimeException) {
      return Snapshot(null, "read-failed")
    }
    return if (scale.isFinite() && scale > 0f) {
      Snapshot(scale)
    } else {
      Snapshot(null, "invalid-native-value")
    }
  }

  override fun definition() = ModuleDefinition {
    Name("RudiFontScale")
    Events("onFontScaleChanged")

    OnCreate {
      synchronized(gate) {
        if (active) return@OnCreate
        val heldContext = appContext.reactContext?.applicationContext
        if (heldContext == null) {
          unavailableReason = "context-unavailable"
          return@OnCreate
        }
        try {
          heldContext.registerComponentCallbacks(callbacks)
        } catch (_: RuntimeException) {
          unavailableReason = "observer-unavailable"
          return@OnCreate
        }
        context = heldContext
        active = true
        lastNotified = readSnapshotLocked()
      }
    }

    Function("getFontScale") {
      synchronized(gate) { readSnapshotLocked().toMap() }
    }

    OnDestroy {
      synchronized(gate) {
        // Do not enqueue teardown onto the Expo queue that is being destroyed.
        active = false
        unavailableReason = "module-inactive"
        val heldContext = context
        context = null
        lastNotified = null
        heldContext?.unregisterComponentCallbacks(callbacks)
      }
    }
  }
}
