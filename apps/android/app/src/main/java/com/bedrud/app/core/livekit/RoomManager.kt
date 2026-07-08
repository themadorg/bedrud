package com.bedrud.app.core.livekit

import android.app.Application
import android.app.NotificationChannel
import android.app.NotificationManager
import android.content.Intent
import android.os.Build
import android.util.Log
import androidx.core.app.NotificationCompat
import com.bedrud.app.R
import com.bedrud.app.core.call.CallConnectionService
import com.bedrud.app.core.meeting.stage.StageWire
import io.livekit.android.AudioOptions
import io.livekit.android.AudioType
import io.livekit.android.LiveKit
import io.livekit.android.LiveKitOverrides
import io.livekit.android.events.RoomEvent
import io.livekit.android.events.collect
import io.livekit.android.room.Room
import io.livekit.android.room.RoomException
import io.livekit.android.room.track.DataPublishReliability
import io.livekit.android.room.participant.LocalParticipant
import io.livekit.android.room.track.CameraPosition
import io.livekit.android.room.track.LocalVideoTrack
import io.livekit.android.room.track.Track
import io.livekit.android.room.track.screencapture.ScreenCaptureParams
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.delay
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow
import kotlinx.coroutines.launch
import org.json.JSONObject

enum class ConnectionState {
    DISCONNECTED,
    CONNECTING,
    CONNECTED,
    RECONNECTING,
    FAILED
}

data class ChatAttachment(
    val kind: String,   // "image"
    val url: String,
    val mime: String,
    val w: Int = 0,
    val h: Int = 0,
    val size: Int = 0,
)

data class ChatMessage(
    val senderName: String,
    val text: String,
    val timestamp: Long = System.currentTimeMillis(),
    val isLocal: Boolean = false,
    val attachments: List<ChatAttachment> = emptyList(),
)

class RoomManager(private val application: Application) {

    private var _room: Room? = null
    val room: Room? get() = _room

    private var eventScope: CoroutineScope? = null

    private val _connectionState = MutableStateFlow(ConnectionState.DISCONNECTED)
    val connectionState: StateFlow<ConnectionState> = _connectionState.asStateFlow()

    private val _isMicEnabled = MutableStateFlow(true)
    val isMicEnabled: StateFlow<Boolean> = _isMicEnabled.asStateFlow()

    private val _micMediaError = MutableStateFlow(false)
    val micMediaError: StateFlow<Boolean> = _micMediaError.asStateFlow()

    private val _isCameraEnabled = MutableStateFlow(false)
    val isCameraEnabled: StateFlow<Boolean> = _isCameraEnabled.asStateFlow()

    private val _cameraMediaError = MutableStateFlow(false)
    val cameraMediaError: StateFlow<Boolean> = _cameraMediaError.asStateFlow()

    private val _isScreenShareEnabled = MutableStateFlow(false)
    val isScreenShareEnabled: StateFlow<Boolean> = _isScreenShareEnabled.asStateFlow()

    // Incremented on every participant change to trigger recomposition
    private val _participantVersion = MutableStateFlow(0)
    val participantVersion: StateFlow<Int> = _participantVersion.asStateFlow()

    private val _chatMessages = MutableStateFlow<List<ChatMessage>>(emptyList())
    val chatMessages: StateFlow<List<ChatMessage>> = _chatMessages.asStateFlow()

    private val _roomName = MutableStateFlow<String?>(null)
    val roomName: StateFlow<String?> = _roomName.asStateFlow()

    var onDisconnected: (() -> Unit)? = null

    private val _wasKicked = MutableStateFlow(false)
    val wasKicked: StateFlow<Boolean> = _wasKicked.asStateFlow()

    private val _error = MutableStateFlow<String?>(null)
    val error: StateFlow<String?> = _error.asStateFlow()

    private val _activeStage = MutableStateFlow<StageWire.MeetingStage?>(null)
    val activeStage: StateFlow<StageWire.MeetingStage?> = _activeStage.asStateFlow()

    private var activeStageLocal: StageWire.MeetingStage? = null

    suspend fun connectIfNeeded(
        url: String,
        token: String,
        roomName: String? = null,
        avatarUrl: String? = null,
    ) {
        if (_connectionState.value == ConnectionState.CONNECTED && _roomName.value == roomName) {
            return
        }
        if (_connectionState.value != ConnectionState.DISCONNECTED) {
            disconnect()
        }
        connect(url, token, roomName, avatarUrl)
    }

    suspend fun connect(url: String, token: String, roomName: String? = null, avatarUrl: String? = null) {
        try {
            _connectionState.value = ConnectionState.CONNECTING
            _error.value = null
            _roomName.value = roomName

            val room = LiveKit.create(
                application,
                overrides = LiveKitOverrides(
                    audioOptions = AudioOptions(
                        audioOutputType = AudioType.CallAudioType(),
                    ),
                ),
            )
            _room = room

            room.connect(url, token)
            room.audioSwitchHandler?.start()

            _connectionState.value = ConnectionState.CONNECTED

            // Set avatar metadata on local participant
            if (!avatarUrl.isNullOrBlank()) {
                try {
                    val metadata = JSONObject().apply {
                        put("avatarUrl", avatarUrl)
                    }.toString()
                    room.localParticipant.updateMetadata(metadata)
                } catch (e: Exception) {
                    Log.e(TAG, "Failed to set avatar metadata", e)
                }
            }

            // Enable mic after connecting; camera stays off until the user turns it on.
            try {
                val micPublished = room.localParticipant.setMicrophoneEnabled(true)
                syncMicrophoneState()
                _micMediaError.value = !micPublished && room.localParticipant.isMicrophoneEnabled.not()
            } catch (e: Exception) {
                Log.e(TAG, "Failed to enable microphone", e)
                syncMicrophoneState()
                _micMediaError.value = true
                _error.value = "Failed to enable microphone"
            }

            try {
                room.localParticipant.setCameraEnabled(false)
                syncCameraState()
            } catch (e: Exception) {
                Log.e(TAG, "Failed to disable camera", e)
                syncCameraState()
            }

            // Notify initial participant state
            _participantVersion.value++

            // Listen for room events
            eventScope?.cancel()
            eventScope = CoroutineScope(Dispatchers.Main + SupervisorJob())
            eventScope?.launch {
                room.events.collect { event ->
                    when (event) {
                        is RoomEvent.DataReceived -> {
                            if (event.topic == StageWire.STAGE_DATA_TOPIC) {
                                handleStageData(event)
                            } else {
                                handleDataReceived(event)
                            }
                        }
                        is RoomEvent.ParticipantConnected -> {
                            _participantVersion.value++
                            requestStageState()
                            pushOwnedStageState()
                        }
                        is RoomEvent.ParticipantDisconnected -> {
                            val disconnectedIdentity = event.participant.identity?.value
                            if (disconnectedIdentity != null &&
                                activeStageLocal?.ownerIdentity == disconnectedIdentity
                            ) {
                                applyRemoteStage(null)
                            }
                            _participantVersion.value++
                        }
                        is RoomEvent.TrackSubscribed -> _participantVersion.value++
                        is RoomEvent.TrackUnsubscribed -> _participantVersion.value++
                        is RoomEvent.TrackPublished -> {
                            if (event.publication.source == Track.Source.SCREEN_SHARE &&
                                event.participant == room.localParticipant
                            ) {
                                _isScreenShareEnabled.value = true
                            }
                            _participantVersion.value++
                        }
                        is RoomEvent.LocalTrackSubscribed -> _participantVersion.value++
                        is RoomEvent.TrackUnpublished -> {
                            if (event.publication.source == Track.Source.SCREEN_SHARE &&
                                event.participant == room.localParticipant
                            ) {
                                _isScreenShareEnabled.value = false
                            }
                            _participantVersion.value++
                        }
                        // When a track is muted/unmuted (e.g. camera turned off/on),
                        // increment participantVersion so UI recomposes and can switch
                        // between video and avatar.
                        is RoomEvent.TrackMuted -> {
                            if (event.participant == room.localParticipant) {
                                when (event.publication.source) {
                                    Track.Source.MICROPHONE -> {
                                        _isMicEnabled.value = false
                                        CallConnectionService.updateMuteState(true)
                                    }
                                    Track.Source.CAMERA -> _isCameraEnabled.value = false
                                    else -> Unit
                                }
                            }
                            _participantVersion.value++
                        }
                        is RoomEvent.TrackUnmuted -> {
                            if (event.participant == room.localParticipant) {
                                when (event.publication.source) {
                                    Track.Source.MICROPHONE -> {
                                        _isMicEnabled.value = true
                                        _micMediaError.value = false
                                        CallConnectionService.updateMuteState(false)
                                    }
                                    Track.Source.CAMERA -> {
                                        _isCameraEnabled.value = true
                                        _cameraMediaError.value = false
                                    }
                                    else -> Unit
                                }
                            }
                            _participantVersion.value++
                        }
                        is RoomEvent.Reconnecting -> {
                            _connectionState.value = ConnectionState.RECONNECTING
                        }
                        is RoomEvent.Reconnected -> {
                            _connectionState.value = ConnectionState.CONNECTED
                            _participantVersion.value++
                        }
                        is RoomEvent.Disconnected -> {
                            // "PARTICIPANT_REMOVED" is the LiveKit disconnect reason for kick
                            val kicked = event.reason?.name == "PARTICIPANT_REMOVED"
                            if (kicked) _wasKicked.value = true
                            _connectionState.value = ConnectionState.DISCONNECTED
                            onDisconnected?.invoke()
                        }
                        else -> {}
                    }
                }
            }

            scheduleStageStateRequests()

            Log.d(TAG, "Connected to room: ${room.name}")
        } catch (e: RoomException) {
            Log.e(TAG, "Failed to connect to room", e)
            _connectionState.value = ConnectionState.FAILED
            _error.value = e.message ?: "Connection failed"
        } catch (e: Exception) {
            Log.e(TAG, "Unexpected error connecting to room", e)
            _connectionState.value = ConnectionState.FAILED
            _error.value = e.message ?: "Unexpected error"
        }
    }

    private fun handleDataReceived(event: RoomEvent.DataReceived) {
        try {
            val json = JSONObject(String(event.data, Charsets.UTF_8))
            val isChat = event.topic == "chat" || json.optString("type") == "chat"
            if (isChat) {
                val senderName = json.optString("senderName", "").ifBlank {
                    event.participant?.name
                        ?: event.participant?.identity?.value
                        ?: "Unknown"
                }
                // Parse optional attachments (forward-compatible: old clients send none)
                val attachments = mutableListOf<ChatAttachment>()
                val attArray = json.optJSONArray("attachments")
                if (attArray != null) {
                    for (i in 0 until attArray.length()) {
                        val att = attArray.optJSONObject(i) ?: continue
                        attachments.add(ChatAttachment(
                            kind = att.optString("kind", "image"),
                            url = att.optString("url", ""),
                            mime = att.optString("mime", ""),
                            w = att.optInt("w", 0),
                            h = att.optInt("h", 0),
                            size = att.optInt("size", 0),
                        ))
                    }
                }
                val msg = ChatMessage(
                    senderName = senderName,
                    text = json.optString("message", ""),
                    isLocal = false,
                    attachments = attachments,
                )
                _chatMessages.value += msg
            }
        } catch (e: Exception) {
            Log.e(TAG, "Failed to parse data message", e)
        }
    }

    fun disconnect() {
        eventScope?.cancel()
        eventScope = null
        _room?.disconnect()
        _room?.release()
        _room = null
        _connectionState.value = ConnectionState.DISCONNECTED
        _roomName.value = null
        _isMicEnabled.value = true
        _micMediaError.value = false
        _isCameraEnabled.value = false
        _cameraMediaError.value = false
        _isScreenShareEnabled.value = false
        _participantVersion.value = 0
        _chatMessages.value = emptyList()
        _wasKicked.value = false
        _error.value = null
        activeStageLocal = null
        _activeStage.value = null
        Log.d(TAG, "Disconnected from room")
    }

    private fun syncMicrophoneState() {
        val localParticipant = _room?.localParticipant ?: return
        val enabled = localParticipant.isMicrophoneEnabled
        _isMicEnabled.value = enabled
        CallConnectionService.updateMuteState(!enabled)
    }

    suspend fun setMicrophoneEnabled(enabled: Boolean) {
        val localParticipant = _room?.localParticipant ?: return
        if (localParticipant.isMicrophoneEnabled == enabled) {
            syncMicrophoneState()
            if (enabled) _micMediaError.value = false
            return
        }
        try {
            val published = localParticipant.setMicrophoneEnabled(enabled)
            syncMicrophoneState()
            if (!published && enabled) {
                _micMediaError.value = true
                _error.value = "Failed to enable microphone"
            } else {
                _micMediaError.value = false
            }
        } catch (e: Exception) {
            Log.e(TAG, "Failed to set microphone enabled=$enabled", e)
            syncMicrophoneState()
            if (enabled) _micMediaError.value = true
            _error.value = "Failed to toggle microphone"
        }
    }

    suspend fun toggleMicrophone() {
        val localParticipant = _room?.localParticipant ?: return
        setMicrophoneEnabled(!localParticipant.isMicrophoneEnabled)
    }

    private fun syncCameraState() {
        val localParticipant = _room?.localParticipant ?: return
        _isCameraEnabled.value = localParticipant.isCameraEnabled
    }

    suspend fun toggleCamera() {
        val localParticipant = _room?.localParticipant ?: return
        val enabled = !localParticipant.isCameraEnabled
        try {
            val published = localParticipant.setCameraEnabled(enabled)
            syncCameraState()
            _participantVersion.value++
            if (!published && enabled) {
                _cameraMediaError.value = true
                _error.value = "Failed to enable camera"
            } else {
                _cameraMediaError.value = false
            }
        } catch (e: Exception) {
            Log.e(TAG, "Failed to toggle camera", e)
            syncCameraState()
            if (enabled) _cameraMediaError.value = true
            _error.value = "Failed to toggle camera"
        }
    }

    fun switchCamera() {
        if (!_isCameraEnabled.value) return
        val localParticipant = _room?.localParticipant ?: return
        val videoTrack = localParticipant.getTrackPublication(Track.Source.CAMERA)
            ?.track as? LocalVideoTrack ?: return

        try {
            val nextPosition = if (videoTrack.options.position == CameraPosition.FRONT) {
                CameraPosition.BACK
            } else {
                CameraPosition.FRONT
            }
            // Use switchCamera (capturer flip) — restartTrack disposes the RTC track and
            // races with LiveKit's RTCMetricsManager, crashing the app.
            videoTrack.switchCamera(position = nextPosition)
        } catch (e: Exception) {
            Log.e(TAG, "Failed to switch camera", e)
            _error.value = "Failed to switch camera"
        }
    }

    suspend fun startScreenShare(mediaProjectionPermissionResultData: Intent): Boolean {
        val localParticipant = _room?.localParticipant ?: return false
        if (!claimScreenShareStage()) {
            return false
        }

        try {
            ensureScreenShareNotificationChannel()
            val published = localParticipant.setScreenShareEnabled(
                true,
                ScreenCaptureParams(
                    mediaProjectionPermissionResultData = mediaProjectionPermissionResultData,
                    notificationId = SCREEN_SHARE_NOTIFICATION_ID,
                    notification = buildScreenShareNotification(),
                    onStop = {
                        eventScope?.launch {
                            clearOwnedScreenShareStage()
                            _isScreenShareEnabled.value = false
                        }
                    },
                ),
            )
            if (!published) {
                clearOwnedScreenShareStage()
                _isScreenShareEnabled.value = false
                _error.value = "Failed to publish screen share"
                return false
            }
            _isScreenShareEnabled.value = localParticipant.isScreenShareEnabled
            pushOwnedStageState()
            return true
        } catch (e: Exception) {
            Log.e(TAG, "Failed to start screen share", e)
            clearOwnedScreenShareStage()
            _isScreenShareEnabled.value = false
            _error.value = "Screen share is not available: ${e.message}"
            return false
        }
    }

    suspend fun stopScreenShare() {
        val localParticipant = _room?.localParticipant ?: return
        try {
            localParticipant.setScreenShareEnabled(false)
            _isScreenShareEnabled.value = localParticipant.isScreenShareEnabled
            clearOwnedScreenShareStage()
        } catch (e: Exception) {
            Log.e(TAG, "Failed to stop screen share", e)
            _error.value = "Failed to stop screen share: ${e.message}"
        }
    }

    private suspend fun claimScreenShareStage(): Boolean {
        val room = _room ?: return false
        val localParticipant = room.localParticipant
        val ownerIdentity = localParticipant.identity?.value ?: return false
        val ownerName = localParticipant.name ?: ownerIdentity

        val current = activeStageLocal
        if (current != null && current.ownerIdentity != ownerIdentity) {
            _error.value = "${current.ownerName} is already on stage"
            return false
        }

        val stage = StageWire.MeetingStage(
            kind = "screenshare",
            ownerIdentity = ownerIdentity,
            ownerName = ownerName,
            updatedAt = System.currentTimeMillis(),
        )
        activeStageLocal = stage
        _activeStage.value = stage
        publishStageData(StageWire.encodeStageSet(stage))
        return true
    }

    private suspend fun clearOwnedScreenShareStage() {
        val room = _room ?: return
        val ownerIdentity = room.localParticipant.identity?.value ?: return
        val current = activeStageLocal ?: return
        if (current.ownerIdentity != ownerIdentity || current.kind != "screenshare") return

        activeStageLocal = null
        _activeStage.value = null
        publishStageData(
            StageWire.encodeStageClear(ownerIdentity, System.currentTimeMillis()),
        )
    }

    private suspend fun publishStageData(data: ByteArray) {
        val localParticipant = _room?.localParticipant ?: return
        try {
            localParticipant.publishData(
                data = data,
                reliability = DataPublishReliability.RELIABLE,
                topic = StageWire.STAGE_DATA_TOPIC,
            )
        } catch (e: Exception) {
            Log.e(TAG, "Failed to publish stage data", e)
        }
    }

    private fun handleStageData(event: RoomEvent.DataReceived) {
        val message = StageWire.parse(event.data) ?: return
        val localIdentity = _room?.localParticipant?.identity?.value

        when (message) {
            is StageWire.StageMessage.Set -> applyRemoteStage(message.stage)
            is StageWire.StageMessage.Clear -> applyRemoteStage(null)
            is StageWire.StageMessage.State -> applyRemoteStage(message.stage)
            is StageWire.StageMessage.Request -> {
                val owned = activeStageLocal ?: return
                if (owned.ownerIdentity != localIdentity) return
                eventScope?.launch {
                    publishStageData(
                        StageWire.encodeStageState(owned, System.currentTimeMillis()),
                    )
                }
            }
        }
    }

    private fun applyRemoteStage(stage: StageWire.MeetingStage?) {
        activeStageLocal = stage
        _activeStage.value = stage
        _participantVersion.value++
    }

    private fun scheduleStageStateRequests() {
        eventScope?.launch {
            for (delayMs in listOf(800L, 2000L, 4000L)) {
                delay(delayMs)
                requestStageState()
            }
        }
    }

    private suspend fun requestStageState() {
        publishStageData(StageWire.encodeStageRequest(System.currentTimeMillis()))
    }

    private fun pushOwnedStageState() {
        val owned = activeStageLocal ?: return
        val localIdentity = _room?.localParticipant?.identity?.value ?: return
        if (owned.ownerIdentity != localIdentity) return
        eventScope?.launch {
            for (delayMs in listOf(400L, 1200L, 2500L)) {
                delay(delayMs)
                publishStageData(StageWire.encodeStageState(owned, System.currentTimeMillis()))
            }
        }
    }

    private fun ensureScreenShareNotificationChannel() {
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.O) return
        val channel = NotificationChannel(
            SCREEN_SHARE_CHANNEL_ID,
            application.getString(R.string.screen_share_channel_name),
            NotificationManager.IMPORTANCE_LOW,
        ).apply {
            description = application.getString(R.string.screen_share_channel_description)
            setShowBadge(false)
        }
        val manager = application.getSystemService(NotificationManager::class.java)
        manager.createNotificationChannel(channel)
    }

    private fun buildScreenShareNotification() =
        NotificationCompat.Builder(application, SCREEN_SHARE_CHANNEL_ID)
            .setContentTitle(application.getString(R.string.screen_share_notification_title))
            .setContentText(application.getString(R.string.screen_share_notification_text))
            .setSmallIcon(R.drawable.ic_call_notification)
            .setOngoing(true)
            .setSilent(true)
            .setCategory(NotificationCompat.CATEGORY_CALL)
            .setForegroundServiceBehavior(NotificationCompat.FOREGROUND_SERVICE_IMMEDIATE)
            .build()

    suspend fun sendChatMessage(text: String, attachments: List<ChatAttachment> = emptyList()) {
        val room = _room ?: return
        val localParticipant = room.localParticipant
        val name = localParticipant.name ?: localParticipant.identity?.value ?: "Unknown"
        val identity = localParticipant.identity?.value ?: ""

        val json = JSONObject().apply {
            put("type", "chat")
            put("id", java.util.UUID.randomUUID().toString())
            put("timestamp", System.currentTimeMillis())
            put("message", text)
            put("senderName", name)
            put("senderIdentity", identity)
            if (attachments.isNotEmpty()) {
                val attArray = org.json.JSONArray()
                attachments.forEach { att ->
                    attArray.put(JSONObject().apply {
                        put("kind", att.kind)
                        put("url", att.url)
                        put("mime", att.mime)
                        put("w", att.w)
                        put("h", att.h)
                        put("size", att.size)
                    })
                }
                put("attachments", attArray)
            }
        }
        try {
            localParticipant.publishData(
                data = json.toString().toByteArray(Charsets.UTF_8),
                reliability = DataPublishReliability.RELIABLE,
                topic = "chat"
            )
            val msg = ChatMessage(senderName = name, text = text, isLocal = true, attachments = attachments)
            _chatMessages.value += msg
        } catch (e: Exception) {
            Log.e(TAG, "Failed to send chat message", e)
            _error.value = "Failed to send message"
        }
    }

    fun getLocalParticipant(): LocalParticipant? {
        return _room?.localParticipant
    }

    companion object {
        private const val TAG = "RoomManager"
        private const val SCREEN_SHARE_CHANNEL_ID = "bedrud_screen_share"
        private const val SCREEN_SHARE_NOTIFICATION_ID = 1002
    }
}
