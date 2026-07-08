package com.bedrud.app.core.call

import android.app.Notification
import android.app.NotificationChannel
import android.app.NotificationManager
import android.app.PendingIntent
import android.app.Service
import android.content.Context
import android.content.Intent
import android.content.pm.PackageManager
import android.content.pm.ServiceInfo
import android.os.Build
import android.os.IBinder
import android.os.PowerManager
import android.os.SystemClock
import android.util.Log
import androidx.core.app.NotificationCompat
import com.bedrud.app.MainActivity
import com.bedrud.app.R
import com.bedrud.app.core.instance.InstanceManager
import com.bedrud.app.core.livekit.RoomManager
import kotlinx.coroutines.CoroutineScope
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.SupervisorJob
import kotlinx.coroutines.cancel
import kotlinx.coroutines.flow.collectLatest
import kotlinx.coroutines.launch
import org.koin.android.ext.android.inject

class CallService : Service() {

    private val instanceManager: InstanceManager by inject()
    private var roomManager: RoomManager? = null
    private var serviceScope: CoroutineScope? = null
    private var callStartTime: Long = 0L
    private var wakeLock: PowerManager.WakeLock? = null

    override fun onBind(intent: Intent?): IBinder? = null

    override fun onStartCommand(intent: Intent?, flags: Int, startId: Int): Int {
        when (intent?.action) {
            ACTION_HANG_UP -> {
                hangUp()
                return START_NOT_STICKY
            }
            ACTION_TOGGLE_MUTE -> {
                serviceScope?.launch {
                    roomManager?.toggleMicrophone()
                }
                return START_STICKY
            }
            ACTION_RETURN_TO_MEETING -> {
                updateForegroundNotification()
                return START_STICKY
            }
        }

        val roomName = intent?.getStringExtra(EXTRA_ROOM_NAME) ?: run {
            stopSelf()
            return START_NOT_STICKY
        }
        val url = intent.getStringExtra(EXTRA_URL) ?: run {
            stopSelf()
            return START_NOT_STICKY
        }
        val token = intent.getStringExtra(EXTRA_TOKEN) ?: run {
            stopSelf()
            return START_NOT_STICKY
        }
        val avatarUrl = intent.getStringExtra(EXTRA_AVATAR_URL)

        if (isRunning && activeRoomName == roomName) {
            // Already in this meeting — keep the foreground call alive.
            updateForegroundNotification()
            return START_STICKY
        }

        val rm = instanceManager.roomManager.value ?: run {
            Log.e(TAG, "No RoomManager available")
            stopSelf()
            return START_NOT_STICKY
        }
        roomManager = rm
        activeRoomName = roomName
        isRunning = true
        callStartTime = SystemClock.elapsedRealtime()

        createNotificationChannel()
        CallTelecom.registerPhoneAccount(this)
        val callPlaced = CallConnectionService.placeCall(this, roomName)
        if (!callPlaced) {
            Log.e(TAG, "Telecom placeCall failed; stopping service")
            stopSelf()
            return START_NOT_STICKY
        }
        startCallForeground(roomName)

        acquireWakeLock()

        serviceScope?.cancel()
        serviceScope = CoroutineScope(Dispatchers.Main + SupervisorJob())
        serviceScope?.launch {
            rm.connectIfNeeded(url, token, roomName, avatarUrl)
        }

        rm.onDisconnected = {
            Log.d(TAG, "Room disconnected by server, stopping service")
            hangUp()
        }

        CallConnectionService.muteListener = { muted ->
            serviceScope?.launch {
                roomManager?.setMicrophoneEnabled(!muted)
                updateForegroundNotification(isMuted = muted)
            }
        }

        serviceScope?.launch {
            rm.isMicEnabled.collectLatest { micEnabled ->
                updateForegroundNotification(isMuted = !micEnabled)
                CallConnectionService.updateMuteState(!micEnabled)
            }
        }

        return START_STICKY
    }

    override fun onDestroy() {
        super.onDestroy()
        isRunning = false
        activeRoomName = null

        releaseWakeLock()

        roomManager?.onDisconnected = null
        if (userInitiatedHangUp) {
            roomManager?.disconnect()
        }
        roomManager = null
        userInitiatedHangUp = false

        CallConnectionService.muteListener = null
        CallConnectionService.endCall()

        serviceScope?.cancel()
        serviceScope = null

        Log.d(TAG, "CallService destroyed")
    }

    private fun hangUp() {
        userInitiatedHangUp = true
        stopSelf()
    }

    private fun startCallForeground(roomName: String) {
        val notification = buildNotification(roomName, isMuted = false)
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.Q) {
            startForeground(
                NOTIFICATION_ID,
                notification,
                ServiceInfo.FOREGROUND_SERVICE_TYPE_MICROPHONE or
                    ServiceInfo.FOREGROUND_SERVICE_TYPE_CAMERA or
                    ServiceInfo.FOREGROUND_SERVICE_TYPE_PHONE_CALL,
            )
        } else {
            startForeground(NOTIFICATION_ID, notification)
        }
    }

    private fun updateForegroundNotification(isMuted: Boolean = roomManager?.isMicEnabled?.value == false) {
        val roomName = activeRoomName ?: return
        val notification = buildNotification(roomName, isMuted)
        val nm = getSystemService(NotificationManager::class.java)
        if (Build.VERSION.SDK_INT < Build.VERSION_CODES.TIRAMISU ||
            checkSelfPermission(android.Manifest.permission.POST_NOTIFICATIONS) == PackageManager.PERMISSION_GRANTED
        ) {
            nm.notify(NOTIFICATION_ID, notification)
        }
    }

    private fun acquireWakeLock() {
        releaseWakeLock()
        val pm = getSystemService(PowerManager::class.java) ?: return
        wakeLock = pm.newWakeLock(PowerManager.PARTIAL_WAKE_LOCK, "bedrud:active_call").apply {
            setReferenceCounted(false)
            acquire(MAX_CALL_DURATION_MS)
        }
    }

    private fun releaseWakeLock() {
        wakeLock?.let { lock ->
            if (lock.isHeld) lock.release()
        }
        wakeLock = null
    }

    private fun createNotificationChannel() {
        val channel = NotificationChannel(
            CHANNEL_ID,
            getString(R.string.call_channel_name),
            NotificationManager.IMPORTANCE_DEFAULT,
        ).apply {
            description = getString(R.string.call_channel_description)
            setShowBadge(false)
            lockscreenVisibility = Notification.VISIBILITY_PRIVATE
            enableVibration(false)
            setSound(null, null)
        }
        val nm = getSystemService(NotificationManager::class.java)
        nm.createNotificationChannel(channel)
    }

    private fun buildNotification(roomName: String, isMuted: Boolean): Notification {
        val contentIntent = PendingIntent.getActivity(
            this,
            0,
            Intent(this, MainActivity::class.java).apply {
                action = ACTION_RETURN_TO_MEETING
                putExtra(EXTRA_ROOM_NAME, roomName)
                flags = Intent.FLAG_ACTIVITY_SINGLE_TOP or Intent.FLAG_ACTIVITY_CLEAR_TOP
            },
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )

        val muteIntent = PendingIntent.getService(
            this,
            1,
            Intent(this, CallService::class.java).apply { action = ACTION_TOGGLE_MUTE },
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
        val muteAction = NotificationCompat.Action.Builder(
            if (isMuted) android.R.drawable.ic_lock_silent_mode
            else android.R.drawable.ic_lock_silent_mode_off,
            if (isMuted) getString(R.string.call_action_unmute) else getString(R.string.call_action_mute),
            muteIntent,
        ).build()

        val hangUpIntent = PendingIntent.getService(
            this,
            2,
            Intent(this, CallService::class.java).apply { action = ACTION_HANG_UP },
            PendingIntent.FLAG_UPDATE_CURRENT or PendingIntent.FLAG_IMMUTABLE,
        )
        val hangUpAction = NotificationCompat.Action.Builder(
            android.R.drawable.ic_menu_close_clear_cancel,
            getString(R.string.call_action_hangUp),
            hangUpIntent,
        ).build()

        return NotificationCompat.Builder(this, CHANNEL_ID)
            .setContentTitle(getString(R.string.call_notification_title, roomName))
            .setContentText(getString(R.string.call_notification_text))
            .setSmallIcon(R.drawable.ic_call_notification)
            .setCategory(NotificationCompat.CATEGORY_CALL)
            .setOngoing(true)
            .setOnlyAlertOnce(true)
            .setSilent(true)
            .setUsesChronometer(true)
            .setShowWhen(true)
            .setWhen(System.currentTimeMillis() - (SystemClock.elapsedRealtime() - callStartTime))
            .setContentIntent(contentIntent)
            .addAction(muteAction)
            .addAction(hangUpAction)
            .setForegroundServiceBehavior(NotificationCompat.FOREGROUND_SERVICE_IMMEDIATE)
            .setVisibility(NotificationCompat.VISIBILITY_PRIVATE)
            .setPriority(NotificationCompat.PRIORITY_DEFAULT)
            .build()
    }

    companion object {
        private const val TAG = "CallService"
        private const val CHANNEL_ID = "bedrud_call_ongoing"
        private const val NOTIFICATION_ID = 1001
        const val EXTRA_ROOM_NAME = "room_name"
        private const val EXTRA_URL = "url"
        private const val EXTRA_TOKEN = "token"
        private const val EXTRA_AVATAR_URL = "avatar_url"
        const val ACTION_HANG_UP = "com.bedrud.app.HANG_UP"
        const val ACTION_TOGGLE_MUTE = "com.bedrud.app.TOGGLE_MUTE"
        const val ACTION_RETURN_TO_MEETING = "com.bedrud.app.RETURN_TO_MEETING"
        private const val MAX_CALL_DURATION_MS = 8 * 60 * 60 * 1000L

        var isRunning = false
            private set

        var activeRoomName: String? = null
            private set

        private var userInitiatedHangUp = false

        fun start(context: Context, roomName: String, url: String, token: String, avatarUrl: String? = null) {
            val intent = Intent(context, CallService::class.java).apply {
                putExtra(EXTRA_ROOM_NAME, roomName)
                putExtra(EXTRA_URL, url)
                putExtra(EXTRA_TOKEN, token)
                avatarUrl?.let { putExtra(EXTRA_AVATAR_URL, it) }
            }
            context.startForegroundService(intent)
        }

        fun stop(context: Context) {
            userInitiatedHangUp = true
            context.stopService(Intent(context, CallService::class.java))
        }
    }
}