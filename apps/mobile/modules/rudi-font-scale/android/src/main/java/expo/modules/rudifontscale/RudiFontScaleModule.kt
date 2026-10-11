package expo.modules.rudifontscale

import android.content.ComponentCallbacks
import android.content.Context
import android.content.res.Configuration
import android.os.Handler
import android.os.Looper
import android.view.ViewGroup
import com.facebook.react.ReactApplication
import expo.modules.kotlin.modules.Module
import expo.modules.kotlin.modules.ModuleDefinition

class RudiFontScaleModule : Module() {
  private val gate = Any()
  private val mainHandler = Handler(Looper.getMainLooper())
  private var context: Context? = null
  private var active = false
  private var unavailableReason = "module-inactive"
  private var lastNotified: Snapshot? = null
  private var lastAvailableFontScale: Float? = null
  private var layoutRequestPending = false

  // A queued pass reads the current activity; it never retains an old one.
  private val requestFontLayout = Runnable {
    synchronized(gate) {
      layoutRequestPending = false
      if (!active) return@Runnable
      val activity = appContext.currentActivity ?: return@Runnable
      if (activity.isFinishing || activity.isDestroyed) return@Runnable
      val content = activity.findViewById<ViewGroup>(android.R.id.content) ?: return@Runnable
      val host = (activity.application as? ReactApplication)?.reactHost
      host?.onConfigurationChanged(activity)
      content.requestLayout()
      for (i in 0 until content.childCount) {
        val child = content.getChildAt(i)
        child.requestLayout()
        // Expo's debug host can put one container around the surface view.
        if (child is ViewGroup) {
          for (j in 0 until child.childCount) {
            child.getChildAt(j).requestLayout()
          }
        }
      }
    }
  }

  private fun requestFontLayoutLocked() {
    if (layoutRequestPending) return
    layoutRequestPending = true
    if (!mainHandler.post(requestFontLayout)) layoutRequestPending = false
  }

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
        val scale = next.fontScale
        if (scale != null) {
          val previous = lastAvailableFontScale
          lastAvailableFontScale = scale
          if (scale != previous) requestFontLayoutLocked()
        }
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
        lastAvailableFontScale = lastNotified?.fontScale
      }
    }

    Function("getFontScale") {
      synchronized(gate) { readSnapshotLocked().toMap() }
    }

    OnDestroy {
      synchronized(gate) {
        // Cancel Android work directly, without the Expo queue being destroyed.
        active = false
        mainHandler.removeCallbacks(requestFontLayout)
        layoutRequestPending = false
        unavailableReason = "module-inactive"
        val heldContext = context
        context = null
        lastNotified = null
        lastAvailableFontScale = null
        heldContext?.unregisterComponentCallbacks(callbacks)
      }
    }
  }
}
