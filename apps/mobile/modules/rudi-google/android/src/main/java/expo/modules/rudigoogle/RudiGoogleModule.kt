package expo.modules.rudigoogle

import androidx.credentials.CredentialManager
import androidx.credentials.CustomCredential
import androidx.credentials.GetCredentialRequest
import androidx.credentials.exceptions.GetCredentialCancellationException
import com.google.android.libraries.identity.googleid.GetSignInWithGoogleOption
import com.google.android.libraries.identity.googleid.GoogleIdTokenCredential
import expo.modules.kotlin.functions.Coroutine
import expo.modules.kotlin.modules.Module
import expo.modules.kotlin.modules.ModuleDefinition

class RudiGoogleModule : Module() {
  override fun definition() = ModuleDefinition {
    Name("RudiGoogle")
    AsyncFunction("signIn") Coroutine { clientId: String, nonce: String ->
      val activity = appContext.currentActivity ?: throw IllegalStateException("activity_unavailable")
      val option = GetSignInWithGoogleOption.Builder(clientId).setNonce(nonce).build()
      val request = GetCredentialRequest.Builder().addCredentialOption(option).build()
      try {
        val result = CredentialManager.create(activity).getCredential(activity, request)
        val credential = result.credential
        if (credential !is CustomCredential || credential.type != GoogleIdTokenCredential.TYPE_GOOGLE_ID_TOKEN_CREDENTIAL) {
          throw IllegalStateException("google_credential_invalid")
        }
        GoogleIdTokenCredential.createFrom(credential.data).idToken
      } catch (_: GetCredentialCancellationException) {
        null
      }
    }
  }
}
