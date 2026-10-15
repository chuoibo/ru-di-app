package expo.modules.rudifontscale

import android.content.ComponentCallbacks
import android.content.Context
import android.content.res.Configuration
import android.content.res.Resources
import android.os.Handler
import android.os.Looper
import android.util.DisplayMetrics
import android.util.TypedValue
import android.view.ViewGroup
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

  private data class Snapshot(
    val fontScale: Float?,
    val reason: String = "",
    val fontMetrics: Map<String, Any?>? = null,
  ) {
    fun toMap(): Map<String, Any?> {
      if (fontScale == null) return mapOf("status" to "unavailable", "reason" to reason)
      val value = mapOf<String, Any?>("status" to "available", "fontScale" to fontScale.toDouble())
      return if (fontMetrics == null) value else value + ("fontMetrics" to fontMetrics)
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

  // Optional metrics failure keeps a valid scale and the normal RN text path.
  private fun sampleFontMetrics(resources: Resources): Map<String, Any?>? {
    return try {
      val metrics = DisplayMetrics().apply { setTo(resources.displayMetrics) }
      val density = metrics.density
      if (!density.isFinite() || density <= 0f) return null
      val bases = listOf(-0.65, -0.3, -0.15, 0.4, 0.8, 1.6,
        11.0, 12.0, 13.0, 14.0, 15.0, 17.0, 18.0, 19.0,
        21.0, 22.0, 23.0, 24.0, 27.0, 28.0, 34.0)
      val rows = bases.map { base ->
        val dp = TypedValue.applyDimension(TypedValue.COMPLEX_UNIT_SP, base.toFloat(), metrics) / density
        if (!dp.isFinite() || (base > 0 && dp <= 0f) || (base < 0 && dp >= 0f)) return null
        mapOf("base" to base, "dp" to dp.toDouble())
      }
      mapOf("density" to density.toDouble(), "spToDp" to rows)
    } catch (_: RuntimeException) {
      null
    }
  }

  // The getter and callback use one copied SP converter. The gate protects this
  // module, not OS resources: reject a visible scale race without claiming atomicity.
  private fun readSnapshotLocked(): Snapshot {
    if (!active) return Snapshot(null, unavailableReason)
    val heldContext = context ?: return Snapshot(null, "context-unavailable")
    return try {
      val resources = heldContext.resources
      val before = resources.configuration.fontScale
      if (!before.isFinite() || before <= 0f) return Snapshot(null, "invalid-native-value")
      val fontMetrics = sampleFontMetrics(resources)
      if (resources.configuration.fontScale != before) {
        Snapshot(null, "invalid-native-value")
      } else {
        Snapshot(before, fontMetrics = fontMetrics)
      }
    } catch (_: RuntimeException) {
      Snapshot(null, "read-failed")
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
