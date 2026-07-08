package com.bedrud.app.core.call

import android.content.Context
import android.net.Uri
import android.os.Build
import android.telecom.Connection
import android.telecom.ConnectionRequest
import android.telecom.ConnectionService
import android.telecom.DisconnectCause
import android.telecom.PhoneAccountHandle
import android.telecom.TelecomManager
import android.util.Log
import com.bedrud.app.R

class CallConnectionService : ConnectionService() {

    override fun onCreateOutgoingConnection(
        connectionManagerPhoneAccount: PhoneAccountHandle?,
        request: ConnectionRequest?,
    ): Connection {
        val roomName = parseRoomName(request?.address)
            ?: getString(R.string.call_default_room_name)
        val connection = BedrudConnection(applicationContext, roomName)
        connection.setInitializing()
        connection.setDialing()
        connection.setActive()
        activeConnection = connection
        Log.d(TAG, "Outgoing connection active for room: $roomName")
        return connection
    }

    override fun onCreateOutgoingConnectionFailed(
        connectionManagerPhoneAccount: PhoneAccountHandle?,
        request: ConnectionRequest?,
    ) {
        Log.e(TAG, "Failed to create outgoing connection")
        CallService.stop(applicationContext)
    }

    private class BedrudConnection(
        private val context: Context,
        private val roomName: String,
    ) : Connection() {
        init {
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                connectionProperties = PROPERTY_SELF_MANAGED
            }
            connectionCapabilities = CAPABILITY_MUTE or CAPABILITY_SUPPORT_HOLD
            if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.O) {
                audioModeIsVoip = true
            }
            val address = roomUri(roomName)
            setCallerDisplayName(roomName, TelecomManager.PRESENTATION_ALLOWED)
            setAddress(address, TelecomManager.PRESENTATION_ALLOWED)
        }

        override fun onDisconnect() {
            setDisconnected(DisconnectCause(DisconnectCause.LOCAL))
            destroy()
            activeConnection = null
            CallService.stop(context)
        }

        override fun onAbort() {
            onDisconnect()
        }

        override fun onMuteStateChanged(isMuted: Boolean) {
            muteListener?.invoke(isMuted)
        }

        override fun onCallAudioStateChanged(state: android.telecom.CallAudioState?) {
            // LiveKit manages capture/playback; system routes call audio.
        }
    }

    companion object {
        private const val TAG = "CallConnectionService"
        const val SCHEME = "bedrud"
        private var activeConnection: Connection? = null
        var muteListener: ((Boolean) -> Unit)? = null

        fun placeCall(context: Context, roomName: String): Boolean {
            val telecom = context.getSystemService(Context.TELECOM_SERVICE) as? TelecomManager ?: return false
            CallTelecom.registerPhoneAccount(context)

            val address = roomUri(roomName)
            val extras = android.os.Bundle().apply {
                putParcelable(TelecomManager.EXTRA_PHONE_ACCOUNT_HANDLE, CallTelecom.phoneAccountHandle(context))
            }

            return try {
                telecom.placeCall(address, extras)
                Log.d(TAG, "Placed self-managed call for room: $roomName")
                true
            } catch (e: SecurityException) {
                Log.e(TAG, "Cannot place call - missing permission", e)
                false
            } catch (e: Exception) {
                Log.e(TAG, "Failed to place call", e)
                false
            }
        }

        fun endCall() {
            try {
                activeConnection?.apply {
                    setDisconnected(DisconnectCause(DisconnectCause.LOCAL))
                    destroy()
                }
            } catch (e: Exception) {
                Log.e(TAG, "Failed to end call connection", e)
            }
            activeConnection = null
        }

        fun updateMuteState(muted: Boolean) {
            try {
                activeConnection?.setActive()
            } catch (e: Exception) {
                Log.e(TAG, "Failed to update mute state", e)
            }
        }

        fun roomUri(roomName: String): Uri =
            Uri.parse("$SCHEME://room/${Uri.encode(roomName)}")

        private fun parseRoomName(address: Uri?): String? {
            address ?: return null
            return when (address.scheme) {
                SCHEME -> address.lastPathSegment?.let(Uri::decode)?.takeIf { it.isNotBlank() }
                "tel" -> address.schemeSpecificPart?.takeIf { it.isNotBlank() }
                else -> address.lastPathSegment?.let(Uri::decode)?.takeIf { it.isNotBlank() }
                    ?: address.schemeSpecificPart?.takeIf { it.isNotBlank() }
            }
        }
    }
}