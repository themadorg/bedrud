package com.bedrud.app.ui.screens.meeting

import android.Manifest
import androidx.activity.compose.rememberLauncherForActivityResult
import androidx.activity.result.PickVisualMediaRequest
import androidx.activity.result.contract.ActivityResultContracts
import androidx.compose.animation.AnimatedVisibility
import androidx.compose.animation.slideInHorizontally
import androidx.compose.animation.slideOutHorizontally
import androidx.compose.ui.platform.LocalLayoutDirection
import androidx.compose.ui.unit.LayoutDirection
import androidx.compose.foundation.ExperimentalFoundationApi
import androidx.compose.foundation.Image
import androidx.compose.foundation.background
import androidx.compose.foundation.combinedClickable
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.aspectRatio
import androidx.compose.foundation.layout.fillMaxHeight
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.grid.GridCells
import androidx.compose.foundation.lazy.grid.LazyVerticalGrid
import androidx.compose.foundation.lazy.grid.items
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.lazy.rememberLazyListState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.Chat
import androidx.compose.material.icons.automirrored.filled.Send
import androidx.compose.material.icons.filled.Badge
import androidx.compose.material.icons.filled.CallEnd
import androidx.compose.material.icons.filled.Cameraswitch
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Image
import androidx.compose.material.icons.filled.KeyboardArrowDown
import androidx.compose.material.icons.filled.Mic
import androidx.compose.material.icons.filled.MicOff
import androidx.compose.material.icons.filled.People
import androidx.compose.material.icons.filled.ScreenShare
import androidx.compose.material.icons.filled.StopScreenShare
import androidx.compose.material.icons.filled.Videocam
import androidx.compose.material.icons.filled.VideocamOff
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.Badge
import androidx.compose.material3.BadgedBox
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.DropdownMenu
import androidx.compose.material3.DropdownMenuItem
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.FloatingActionButtonDefaults
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SmallFloatingActionButton
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.runtime.Composable
import androidx.compose.runtime.DisposableEffect
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.derivedStateOf
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.asImageBitmap
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextAlign
import androidx.compose.ui.text.style.TextDirection
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import coil.compose.AsyncImage
import com.bedrud.app.R
import com.bedrud.app.core.BidiUtils
import com.bedrud.app.core.api.RoomApi
import com.bedrud.app.core.call.CallService
import com.bedrud.app.core.instance.InstanceManager
import com.bedrud.app.core.livekit.ChatMessage
import com.bedrud.app.core.livekit.ConnectionState
import com.bedrud.app.core.pip.PipStateHolder
import com.bedrud.app.models.JoinRoomRequest
import com.bedrud.app.models.JoinRoomResponse
import io.livekit.android.compose.ui.VideoTrackView
import io.livekit.android.room.Room
import io.livekit.android.room.participant.Participant
import io.livekit.android.room.track.Track
import kotlinx.coroutines.launch
import okhttp3.MediaType.Companion.toMediaTypeOrNull
import okhttp3.MultipartBody
import okhttp3.RequestBody.Companion.toRequestBody
import org.json.JSONObject
import org.koin.compose.koinInject

@Composable
fun MeetingScreen(
    roomName: String,
    onLeave: () -> Unit,
    instanceManager: InstanceManager = koinInject(),
    pipStateHolder: PipStateHolder = koinInject()
) {
    val roomApi = instanceManager.roomApi.collectAsState().value ?: return
    val roomManager = instanceManager.roomManager.collectAsState().value ?: return
    val authManager = instanceManager.authManager.collectAsState().value
    val currentUser by (authManager?.currentUser ?: kotlinx.coroutines.flow.MutableStateFlow(null)).collectAsState()
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    val snackbarHostState = remember { SnackbarHostState() }
    val isInPipMode by pipStateHolder.isInPipMode.collectAsState()

    // Track meeting state for PiP
    DisposableEffect(Unit) {
        pipStateHolder.setInMeeting(true)
        onDispose {
            pipStateHolder.setInMeeting(false)
        }
    }

    val connectionState by roomManager.connectionState.collectAsState()
    val isMicEnabled by roomManager.isMicEnabled.collectAsState()
    val isCameraEnabled by roomManager.isCameraEnabled.collectAsState()
    val isScreenShareEnabled by roomManager.isScreenShareEnabled.collectAsState()
    val error by roomManager.error.collectAsState()
    val wasKicked by roomManager.wasKicked.collectAsState()

    val participantVersion by roomManager.participantVersion.collectAsState()
    val chatMessages by roomManager.chatMessages.collectAsState()
    var showChat by remember { mutableStateOf(false) }
    var showParticipants by remember { mutableStateOf(false) }
    var chatInput by remember { mutableStateOf("") }

    // Unread chat count while panel is closed
    var lastReadCount by rememberSaveable { mutableIntStateOf(0) }
    val unreadCount = if (showChat) 0 else (chatMessages.size - lastReadCount).coerceAtLeast(0)
    LaunchedEffect(showChat) { if (showChat) lastReadCount = chatMessages.size }

    // Leave/end dialog
    var showLeaveDialog by remember { mutableStateOf(false) }

    var roomInfo by remember { mutableStateOf<JoinRoomResponse?>(null) }
    var isJoining by remember { mutableStateOf(true) }

    // Request permissions
    val permissionLauncher = rememberLauncherForActivityResult(
        ActivityResultContracts.RequestMultiplePermissions()
    ) { permissions ->
        val allGranted = permissions.values.all { it }
        if (allGranted && roomInfo != null) {
            CallService.start(context, roomName, roomInfo!!.livekitHost, roomInfo!!.token, currentUser?.avatarUrl)
        }
    }

    // Join room via API and connect to LiveKit
    LaunchedEffect(roomName) {
        try {
            val response = roomApi.joinRoom(JoinRoomRequest(roomName = roomName))
            if (response.isSuccessful) {
                roomInfo = response.body()
                val info = roomInfo!!

                // Request permissions then connect
                permissionLauncher.launch(
                    arrayOf(
                        Manifest.permission.CAMERA,
                        Manifest.permission.RECORD_AUDIO
                    )
                )
            } else {
                snackbarHostState.showSnackbar("Failed to join room")
                isJoining = false
            }
        } catch (e: Exception) {
            snackbarHostState.showSnackbar(e.message ?: "Failed to join room")
            isJoining = false
        }
    }

    // Cleanup on dispose
    DisposableEffect(Unit) {
        onDispose {
            CallService.stop(context)
        }
    }

    // Handle server-side disconnect: when connection drops after being connected, leave
    var wasConnected by remember { mutableStateOf(false) }
    LaunchedEffect(connectionState) {
        if (connectionState == ConnectionState.CONNECTED) {
            wasConnected = true
        } else if (wasConnected && connectionState == ConnectionState.DISCONNECTED) {
            onLeave()
        }
    }

    // Show error
    LaunchedEffect(error) {
        error?.let {
            snackbarHostState.showSnackbar(it)
        }
    }

    // "You were removed" overlay shown after kick
    if (wasKicked) {
        KickedScreen(onBack = {
            roomManager.disconnect()
            onLeave()
        })
        return
    }

    Scaffold(
        snackbarHost = { SnackbarHost(snackbarHostState) },
        containerColor = MaterialTheme.colorScheme.background
    ) { padding ->
        when (connectionState) {
            ConnectionState.DISCONNECTED,
            ConnectionState.CONNECTING -> {
                Box(
                    modifier = Modifier
                        .fillMaxSize()
                        .padding(padding),
                    contentAlignment = Alignment.Center
                ) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        CircularProgressIndicator()
                        Spacer(modifier = Modifier.height(16.dp))
                        Text(
                            text = if (connectionState == ConnectionState.CONNECTING)
                                stringResource(R.string.meeting_status_connecting)
                            else stringResource(R.string.meeting_status_preparing),
                            style = MaterialTheme.typography.bodyLarge,
                            color = MaterialTheme.colorScheme.onBackground
                        )
                    }
                }
            }

            ConnectionState.CONNECTED,
            ConnectionState.RECONNECTING -> {
                val room = roomManager.room
                if (room != null) {
                    // Video grid (recomposes when participantVersion changes)
                    val participants = remember(participantVersion) {
                        buildList {
                            room.localParticipant.let { add(it) }
                            addAll(room.remoteParticipants.values)
                        }
                    }

                    val isAdmin = roomInfo?.let { info ->
                        room.localParticipant.identity?.value == info.adminId
                    } ?: false
                    val roomId = roomInfo?.id ?: ""

                    if (isInPipMode) {
                        // PiP mode: show single participant filling the screen
                        val pipParticipant = participants.firstOrNull {
                            it.identity != room.localParticipant.identity
                        } ?: participants.firstOrNull()

                        Box(
                            modifier = Modifier
                                .fillMaxSize()
                                .background(MaterialTheme.colorScheme.surfaceVariant),
                            contentAlignment = Alignment.Center
                        ) {
                            if (pipParticipant != null) {
                                val cameraPublication = pipParticipant.getTrackPublication(Track.Source.CAMERA)
                                val videoTrack = cameraPublication
                                    ?.track as? io.livekit.android.room.track.VideoTrack
                                val isVideoMuted = cameraPublication?.muted == true

                                if (videoTrack != null && !isVideoMuted) {
                                    VideoTrackView(
                                        videoTrack = videoTrack,
                                        modifier = Modifier.fillMaxSize(),
                                        passedRoom = room
                                    )
                                } else {
                                    Text(
                                        text = (pipParticipant.name ?: "").take(1).uppercase(),
                                        style = MaterialTheme.typography.displayLarge,
                                        color = MaterialTheme.colorScheme.onSurfaceVariant
                                    )
                                }
                            }
                        }
                    } else {
                        // Normal mode
                        Box(
                            modifier = Modifier
                                .fillMaxSize()
                                .padding(padding)
                        ) {
                            Column(
                                modifier = Modifier.fillMaxSize()
                            ) {
                                // Meeting Header HUD
                                MeetingHeaderHUD(
                                    roomName = roomName,
                                    participantCount = participants.size,
                                    connectionState = connectionState,
                                    sessionStartedAt = roomInfo?.sessionStartedAt ?: 0L
                                )

                                val columns = when {
                                    participants.size <= 1 -> 1
                                    participants.size <= 4 -> 2
                                    else -> 3
                                }

                                LazyVerticalGrid(
                                    columns = GridCells.Fixed(columns),
                                    modifier = Modifier
                                        .weight(1f)
                                        .fillMaxWidth()
                                        .padding(horizontal = 8.dp),
                                    horizontalArrangement = Arrangement.spacedBy(8.dp),
                                    verticalArrangement = Arrangement.spacedBy(8.dp)
                                ) {
                                    items(
                                        participants,
                                        key = { it.identity?.value ?: it.hashCode() }
                                    ) { participant ->
                                        val isLocalParticipant = participant.identity == room.localParticipant.identity
                                        ParticipantTile(
                                            participant = participant,
                                            isAdmin = isAdmin,
                                            isLocalParticipant = isLocalParticipant,
                                            roomId = roomId,
                                            roomApi = roomApi,
                                            snackbarHostState = snackbarHostState,
                                            scope = scope,
                                            room = room
                                        )
                                    }
                                }

                                // Controls bar
                                Row(
                                    modifier = Modifier
                                        .fillMaxWidth()
                                        .padding(16.dp),
                                    horizontalArrangement = Arrangement.SpaceEvenly,
                                    verticalAlignment = Alignment.CenterVertically
                                ) {
                                    // Mic toggle
                                    SmallFloatingActionButton(
                                        onClick = { scope.launch { roomManager.toggleMicrophone() } },
                                        containerColor = if (isMicEnabled)
                                            MaterialTheme.colorScheme.surfaceVariant
                                        else MaterialTheme.colorScheme.error
                                    ) {
                                        Icon(
                                            if (isMicEnabled) Icons.Default.Mic
                                            else Icons.Default.MicOff,
                                            contentDescription = stringResource(R.string.meeting_contentDescription_toggleMic)
                                        )
                                    }

                                    // Camera toggle
                                    SmallFloatingActionButton(
                                        onClick = { scope.launch { roomManager.toggleCamera() } },
                                        containerColor = if (isCameraEnabled)
                                            MaterialTheme.colorScheme.surfaceVariant
                                        else MaterialTheme.colorScheme.error
                                    ) {
                                        Icon(
                                            if (isCameraEnabled) Icons.Default.Videocam
                                            else Icons.Default.VideocamOff,
                                            contentDescription = stringResource(R.string.meeting_contentDescription_toggleCamera)
                                        )
                                    }

                                    // Switch camera
                                    SmallFloatingActionButton(
                                        onClick = { roomManager.switchCamera() },
                                        containerColor = MaterialTheme.colorScheme.surfaceVariant
                                    ) {
                                        Icon(
                                            Icons.Default.Cameraswitch,
                                            contentDescription = stringResource(R.string.meeting_contentDescription_switchCamera)
                                        )
                                    }

                                    // Screen share toggle
                                    SmallFloatingActionButton(
                                        onClick = { scope.launch { roomManager.toggleScreenShare() } },
                                        containerColor = if (isScreenShareEnabled)
                                            MaterialTheme.colorScheme.primary
                                        else MaterialTheme.colorScheme.surfaceVariant
                                    ) {
                                        Icon(
                                            if (isScreenShareEnabled) Icons.Default.StopScreenShare
                                            else Icons.Default.ScreenShare,
                                            contentDescription = stringResource(R.string.meeting_contentDescription_toggleScreenShare)
                                        )
                                    }

                                    // Chat toggle with unread badge
                                    SmallFloatingActionButton(
                                        onClick = {
                                            showChat = !showChat
                                            if (showChat) showParticipants = false
                                        },
                                        containerColor = if (showChat)
                                            MaterialTheme.colorScheme.primary
                                        else MaterialTheme.colorScheme.surfaceVariant
                                    ) {
                                        BadgedBox(badge = {
                                            if (unreadCount > 0) {
                                                Badge { Text(if (unreadCount > 9) "9+" else unreadCount.toString()) }
                                            }
                                        }) {
                                            Icon(Icons.AutoMirrored.Filled.Chat, contentDescription = stringResource(R.string.meeting_contentDescription_toggleChat))
                                        }
                                    }

                                    // Participants panel toggle
                                    SmallFloatingActionButton(
                                        onClick = {
                                            showParticipants = !showParticipants
                                            if (showParticipants) showChat = false
                                        },
                                        containerColor = if (showParticipants)
                                            MaterialTheme.colorScheme.primary
                                        else MaterialTheme.colorScheme.surfaceVariant
                                    ) {
                                        Icon(Icons.Default.People, contentDescription = stringResource(R.string.meeting_contentDescription_participants))
                                    }

                                    // Leave / End call
                                    FloatingActionButton(
                                        onClick = {
                                            if (isAdmin) showLeaveDialog = true
                                            else { CallService.stop(context); onLeave() }
                                        },
                                        containerColor = MaterialTheme.colorScheme.error,
                                        contentColor = MaterialTheme.colorScheme.onError,
                                        elevation = FloatingActionButtonDefaults.elevation(defaultElevation = 0.dp)
                                    ) {
                                        Icon(Icons.Default.CallEnd, contentDescription = stringResource(R.string.meeting_contentDescription_leaveCall))
                                    }

                                    // Leave/End dialog for room creator
                                    if (showLeaveDialog) {
                                        AlertDialog(
                                            onDismissRequest = { showLeaveDialog = false },
                                            title = { Text(stringResource(R.string.meeting_dialog_leaveTitle)) },
                                            text = { Text(stringResource(R.string.meeting_dialog_leaveMessage)) },
                                            confirmButton = {
                                                TextButton(onClick = {
                                                    showLeaveDialog = false
                                                    scope.launch {
                                                        try {
                                                            roomApi.deleteRoom(roomId)
                                                        } catch (_: Exception) {}
                                                        CallService.stop(context)
                                                        onLeave()
                                                    }
                                                }) {
                                                    Text(stringResource(R.string.meeting_button_endForEveryone), color = MaterialTheme.colorScheme.error)
                                                }
                                            },
                                            dismissButton = {
                                                Row {
                                                    TextButton(onClick = {
                                                        showLeaveDialog = false
                                                        CallService.stop(context)
                                                        onLeave()
                                                    }) { Text(stringResource(R.string.meeting_button_justLeave)) }
                                                    TextButton(onClick = { showLeaveDialog = false }) { Text(stringResource(R.string.common_button_cancel)) }
                                                }
                                            }
                                        )
                                    }
                                }
                            }

                            // Chat panel - slides in from the end side
                            val layoutDirection = LocalLayoutDirection.current
                            val slideIn = slideInHorizontally(
                                initialOffsetX = { if (layoutDirection == LayoutDirection.Rtl) -it else it }
                            )
                            val slideOut = slideOutHorizontally(
                                targetOffsetX = { if (layoutDirection == LayoutDirection.Rtl) -it else it }
                            )
                            AnimatedVisibility(
                                visible = showChat,
                                enter = slideIn,
                                exit = slideOut,
                                modifier = Modifier.align(Alignment.CenterEnd)
                            ) {
                                ChatPanel(
                                    messages = chatMessages,
                                    chatInput = chatInput,
                                    onChatInputChange = { chatInput = it },
                                    onSend = {
                                        if (chatInput.isNotBlank()) {
                                            scope.launch {
                                                roomManager.sendChatMessage(chatInput.trim())
                                                chatInput = ""
                                            }
                                        }
                                    },
                                    onClose = { showChat = false },
                                    roomId = roomInfo?.id,
                                    roomApi = roomApi,
                                    onSendWithAttachment = { text, attachment ->
                                        scope.launch {
                                            roomManager.sendChatMessage(text, listOf(attachment))
                                        }
                                    },
                                )
                            }

                            // Participants panel - slides in from the end side
                            AnimatedVisibility(
                                visible = showParticipants,
                                enter = slideIn,
                                exit = slideOut,
                                modifier = Modifier.align(Alignment.CenterEnd)
                            ) {
                                ParticipantsPanel(
                                    participants = participants,
                                    localIdentity = room.localParticipant.identity?.value,
                                    isAdmin = isAdmin,
                                    roomId = roomId,
                                    roomApi = roomApi,
                                    snackbarHostState = snackbarHostState,
                                    scope = scope,
                                    onClose = { showParticipants = false }
                                )
                            }
                        }
                    }
                }
            }

            ConnectionState.FAILED -> {
                Box(
                    modifier = Modifier
                        .fillMaxSize()
                        .padding(padding),
                    contentAlignment = Alignment.Center
                ) {
                    Column(horizontalAlignment = Alignment.CenterHorizontally) {
                        Text(
                            text = stringResource(R.string.meeting_state_connectionFailed),
                            style = MaterialTheme.typography.headlineSmall,
                            color = MaterialTheme.colorScheme.error
                        )
                        Spacer(modifier = Modifier.height(8.dp))
                        Text(
                            text = error ?: stringResource(R.string.meeting_state_connectionFailedMessage),
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant,
                            textAlign = TextAlign.Center,
                            modifier = Modifier.padding(horizontal = 32.dp)
                        )
                        Spacer(modifier = Modifier.height(24.dp))
                        androidx.compose.material3.FilledTonalButton(onClick = onLeave) {
                            Text(stringResource(R.string.meeting_button_goBack))
                        }
                    }
                }
            }
        }
    }
}

@OptIn(ExperimentalFoundationApi::class)
@Composable
private fun ParticipantTile(
    participant: Participant,
    isAdmin: Boolean = false,
    isLocalParticipant: Boolean = false,
    roomId: String = "",
    roomApi: RoomApi? = null,
    snackbarHostState: SnackbarHostState? = null,
    scope: kotlinx.coroutines.CoroutineScope? = null,
    room: Room? = null
) {
    val cameraPublication = participant.getTrackPublication(Track.Source.CAMERA)
    val videoTrack = cameraPublication
        ?.track as? io.livekit.android.room.track.VideoTrack
    val isVideoMuted = cameraPublication?.muted == true

    val identity = participant.identity?.value ?: "Unknown"
    val name = participant.name?.ifBlank { identity } ?: identity

    // Parse avatar URL from participant metadata
    val avatarUrl = remember(participant.metadata) {
        participant.metadata?.let { meta ->
            try {
                val obj = JSONObject(meta)
                if (obj.has("avatarUrl")) obj.getString("avatarUrl") else null
            } catch (_: Exception) { null }
        }
    }

    var showMenu by remember { mutableStateOf(false) }
    var showKickConfirm by remember { mutableStateOf(false) }
    val showAdminMenu = isAdmin && !isLocalParticipant && roomApi != null

    if (showKickConfirm) {
        AlertDialog(
            onDismissRequest = { showKickConfirm = false },
            title = { Text(text = stringResource(R.string.meeting_dialog_kickTitle)) },
            text = { Text(text = stringResource(R.string.meeting_dialog_kickMessage)) },
            confirmButton = {
                TextButton(onClick = {
                    showKickConfirm = false
                    scope?.launch {
                        try {
                            roomApi?.kickParticipant(roomId, identity)
                        } catch (e: Exception) {
                            snackbarHostState?.showSnackbar(e.message ?: "Failed to kick participant")
                        }
                    }
                }) {
                    Text(stringResource(R.string.meeting_action_kick), color = MaterialTheme.colorScheme.error)
                }
            },
            dismissButton = {
                TextButton(onClick = { showKickConfirm = false }) {
                    Text(stringResource(R.string.common_button_cancel))
                }
            }
        )
    }

    Box(
        modifier = Modifier
            .fillMaxWidth()
            .aspectRatio(16f / 9f)
            .clip(RoundedCornerShape(12.dp))
            .background(MaterialTheme.colorScheme.surfaceVariant)
            .then(
                if (showAdminMenu) {
                    Modifier.combinedClickable(
                        onClick = {},
                        onLongClick = { showMenu = true }
                    )
                } else Modifier
            ),
        contentAlignment = Alignment.Center
    ) {
        if (videoTrack != null && !isVideoMuted) {
            VideoTrackView(
                videoTrack = videoTrack,
                modifier = Modifier.fillMaxSize(),
                passedRoom = room
            )
        } else if (!avatarUrl.isNullOrBlank()) {
            AsyncImage(
                model = avatarUrl,
                contentDescription = stringResource(R.string.meeting_contentDescription_participantAvatar),
                modifier = Modifier
                    .size(56.dp)
                    .clip(CircleShape),
                contentScale = androidx.compose.ui.layout.ContentScale.Crop
            )
        } else {
            // Initials placeholder
            Box(
                modifier = Modifier
                    .size(56.dp)
                    .clip(CircleShape)
                    .background(MaterialTheme.colorScheme.primary),
                contentAlignment = Alignment.Center
            ) {
                Text(
                    text = name.take(1).uppercase(),
                    style = MaterialTheme.typography.headlineMedium,
                    color = MaterialTheme.colorScheme.onPrimary
                )
            }
        }

        // Name label
        Box(
            modifier = Modifier
                .align(Alignment.BottomStart)
                .padding(8.dp)
                .background(
                    MaterialTheme.colorScheme.surface.copy(alpha = 0.7f),
                    RoundedCornerShape(4.dp)
                )
                .padding(horizontal = 8.dp, vertical = 4.dp)
        ) {
            Text(
                text = name,
                style = MaterialTheme.typography.labelSmall.copy(textDirection = TextDirection.Content),
                color = MaterialTheme.colorScheme.onSurface,
                maxLines = 1,
                overflow = TextOverflow.Ellipsis
            )
        }

        // Admin dropdown menu
        if (showAdminMenu) {
            DropdownMenu(
                expanded = showMenu,
                onDismissRequest = { showMenu = false }
            ) {
                DropdownMenuItem(
                    text = { Text(stringResource(R.string.meeting_action_mute)) },
                    onClick = {
                        showMenu = false
                        scope?.launch {
                            try {
                                roomApi?.muteParticipant(roomId, identity)
                            } catch (e: Exception) {
                                snackbarHostState?.showSnackbar(e.message ?: "Failed to mute")
                            }
                        }
                    }
                )
                DropdownMenuItem(
                    text = { Text(stringResource(R.string.meeting_action_disableVideo)) },
                    onClick = {
                        showMenu = false
                        scope?.launch {
                            try {
                                roomApi?.disableParticipantVideo(roomId, identity)
                            } catch (e: Exception) {
                                snackbarHostState?.showSnackbar(e.message ?: "Failed to disable video")
                            }
                        }
                    }
                )
                DropdownMenuItem(
                    text = { Text(stringResource(R.string.meeting_action_bringToStage)) },
                    onClick = {
                        showMenu = false
                        scope?.launch {
                            try {
                                roomApi?.bringToStage(roomId, identity)
                            } catch (e: Exception) {
                                snackbarHostState?.showSnackbar(e.message ?: "Failed")
                            }
                        }
                    }
                )
                DropdownMenuItem(
                    text = { Text(stringResource(R.string.meeting_action_removeFromStage)) },
                    onClick = {
                        showMenu = false
                        scope?.launch {
                            try {
                                roomApi?.removeFromStage(roomId, identity)
                            } catch (e: Exception) {
                                snackbarHostState?.showSnackbar(e.message ?: "Failed")
                            }
                        }
                    }
                )
                DropdownMenuItem(
                    text = { Text(stringResource(R.string.meeting_action_kick), color = MaterialTheme.colorScheme.error) },
                    onClick = {
                        showMenu = false
                        showKickConfirm = true
                    }
                )
                DropdownMenuItem(
                    text = { Text(stringResource(R.string.meeting_action_ban), color = MaterialTheme.colorScheme.error) },
                    onClick = {
                        showMenu = false
                        scope?.launch {
                            try {
                                roomApi?.banParticipant(roomId, identity)
                            } catch (e: Exception) {
                                snackbarHostState?.showSnackbar(e.message ?: "Failed to ban participant")
                            }
                        }
                    }
                )
            }
        }
    }
}

// ── Participants Panel ─────────────────────────────────────────────────────────

@Composable
private fun ParticipantsPanel(
    participants: List<Participant>,
    localIdentity: String?,
    isAdmin: Boolean,
    roomId: String,
    roomApi: RoomApi?,
    snackbarHostState: SnackbarHostState,
    scope: kotlinx.coroutines.CoroutineScope,
    onClose: () -> Unit
) {
    Column(
        modifier = Modifier
            .width(300.dp)
            .fillMaxHeight()
            .background(MaterialTheme.colorScheme.surface)
    ) {
        // Header
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 12.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            Text(
                stringResource(R.string.meeting_panel_participants),
                style = MaterialTheme.typography.titleMedium
            )
            IconButton(onClick = onClose) {
                Icon(Icons.Default.Close, contentDescription = stringResource(R.string.meeting_contentDescription_close))
            }
        }

        androidx.compose.material3.HorizontalDivider()

        LazyColumn(
            modifier = Modifier.weight(1f),
            contentPadding = androidx.compose.foundation.layout.PaddingValues(8.dp),
            verticalArrangement = Arrangement.spacedBy(4.dp)
        ) {
            items(participants) { participant ->
                val identity = participant.identity?.value ?: ""
                val name = participant.name?.ifBlank { identity } ?: identity
                val isLocal = identity == localIdentity

                ParticipantListRow(
                    name = name,
                    identity = identity,
                    isLocal = isLocal,
                    isAdmin = isAdmin,
                    roomId = roomId,
                    roomApi = roomApi,
                    snackbarHostState = snackbarHostState,
                    scope = scope
                )
            }
        }
    }
}

@Composable
private fun ParticipantListRow(
    name: String,
    identity: String,
    isLocal: Boolean,
    isAdmin: Boolean,
    roomId: String,
    roomApi: RoomApi?,
    snackbarHostState: SnackbarHostState,
    scope: kotlinx.coroutines.CoroutineScope
) {
    var showMenu by remember { mutableStateOf(false) }
    val avatarColor = remember(name) {
        val colors = listOf(0xFF6366F1, 0xFF8B5CF6, 0xFF06B6D4, 0xFF10B981, 0xFFF59E0B, 0xFFEF4444)
        colors[Math.abs(name.hashCode()) % colors.size]
    }

    Row(
        modifier = Modifier
            .fillMaxWidth()
            .clip(RoundedCornerShape(8.dp))
            .background(MaterialTheme.colorScheme.surfaceVariant.copy(alpha = 0.5f))
            .padding(horizontal = 8.dp, vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        // Gradient avatar
        Box(
            modifier = Modifier
                .size(36.dp)
                .clip(CircleShape)
                .background(androidx.compose.ui.graphics.Color(avatarColor)),
            contentAlignment = Alignment.Center
        ) {
            Text(
                name.take(1).uppercase(),
                style = MaterialTheme.typography.labelMedium,
                color = androidx.compose.ui.graphics.Color.White
            )
        }

        Column(modifier = Modifier.weight(1f)) {
            Row(horizontalArrangement = Arrangement.spacedBy(4.dp), verticalAlignment = Alignment.CenterVertically) {
                Text(
                    text = name,
                    style = MaterialTheme.typography.bodyMedium,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis
                )
                if (isLocal) {
                    Text(
                        text = stringResource(R.string.meeting_label_you),
                        style = MaterialTheme.typography.labelSmall,
                        color = MaterialTheme.colorScheme.primary
                    )
                }
            }
        }

        // Three-dot menu for admins (on remote participants only)
        if (isAdmin && !isLocal && roomApi != null) {
            Box {
                IconButton(
                    onClick = { showMenu = true },
                    modifier = Modifier.size(28.dp)
                ) {
                    Icon(
                        Icons.Default.Badge,
                        contentDescription = stringResource(R.string.meeting_contentDescription_moreOptions),
                        modifier = Modifier.size(16.dp),
                        tint = MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
                DropdownMenu(expanded = showMenu, onDismissRequest = { showMenu = false }) {
                    DropdownMenuItem(
                        text = { Text(stringResource(R.string.meeting_action_mute)) },
                        onClick = {
                            showMenu = false
                            scope.launch {
                                try { roomApi.muteParticipant(roomId, identity) }
                                catch (e: Exception) { snackbarHostState.showSnackbar(e.message ?: "Failed") }
                            }
                        }
                    )
                    DropdownMenuItem(
                        text = { Text(stringResource(R.string.meeting_action_kick), color = MaterialTheme.colorScheme.error) },
                        onClick = {
                            showMenu = false
                            scope.launch {
                                try { roomApi.kickParticipant(roomId, identity) }
                                catch (e: Exception) { snackbarHostState.showSnackbar(e.message ?: "Failed") }
                            }
                        }
                    )
                    DropdownMenuItem(
                        text = { Text(stringResource(R.string.meeting_action_ban), color = MaterialTheme.colorScheme.error) },
                        onClick = {
                            showMenu = false
                            scope.launch {
                                try { roomApi.banParticipant(roomId, identity) }
                                catch (e: Exception) { snackbarHostState.showSnackbar(e.message ?: "Failed") }
                            }
                        }
                    )
                }
            }
        }
    }
}

// ── Kicked screen ─────────────────────────────────────────────────────────────

@Composable
private fun KickedScreen(onBack: () -> Unit) {
    Box(
        modifier = Modifier.fillMaxSize(),
        contentAlignment = Alignment.Center
    ) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Icon(
                Icons.Default.Badge,
                contentDescription = null,
                modifier = Modifier.size(72.dp),
                tint = MaterialTheme.colorScheme.error
            )
            Spacer(modifier = Modifier.height(16.dp))
            Text(
                text = stringResource(R.string.meeting_state_kickedTitle),
                style = MaterialTheme.typography.headlineSmall,
                color = MaterialTheme.colorScheme.onBackground
            )
            Spacer(modifier = Modifier.height(8.dp))
            Text(
                text = stringResource(R.string.meeting_state_kickedMessage),
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                textAlign = TextAlign.Center,
                modifier = Modifier.padding(horizontal = 32.dp)
            )
            Spacer(modifier = Modifier.height(24.dp))
            androidx.compose.material3.FilledTonalButton(onClick = onBack) {
                Text(stringResource(R.string.meeting_button_backToDashboard))
            }
        }
    }
}

// ── Meeting header HUD ────────────────────────────────────────────────────────

@Composable
private fun MeetingHeaderHUD(
    roomName: String,
    participantCount: Int,
    connectionState: ConnectionState,
    sessionStartedAt: Long
) {
    var connectedAtMs by remember { mutableStateOf<Long?>(null) }
    var elapsedText by remember { mutableStateOf("0:00") }
    LaunchedEffect(connectionState) {
        if (connectionState == ConnectionState.CONNECTED && connectedAtMs == null) {
            connectedAtMs = System.currentTimeMillis()
        }
    }
    val startMs: Long? = if (sessionStartedAt > 0L) sessionStartedAt else connectedAtMs
    LaunchedEffect(startMs) {
        while (startMs != null) {
            val secs = ((System.currentTimeMillis() - startMs) / 1000L).coerceAtLeast(0L)
            val h = secs / 3600
            val m = (secs % 3600) / 60
            val s = secs % 60
            elapsedText = if (h > 0) "%d:%02d:%02d".format(h, m, s) else "%d:%02d".format(m, s)
            kotlinx.coroutines.delay(1000L)
        }
    }

    Row(
        modifier = Modifier
            .fillMaxWidth()
            .padding(horizontal = 12.dp, vertical = 6.dp),
        verticalAlignment = Alignment.CenterVertically,
        horizontalArrangement = Arrangement.spacedBy(8.dp)
    ) {
        // Room name (monospace)
        Text(
            text = roomName,
            style = MaterialTheme.typography.bodySmall.copy(
                fontFamily = FontFamily.Monospace,
                textDirection = TextDirection.Ltr
            ),
            color = MaterialTheme.colorScheme.onBackground,
            maxLines = 1,
            overflow = TextOverflow.Ellipsis,
            modifier = Modifier.weight(1f)
        )

        // Participant count
        Row(verticalAlignment = Alignment.CenterVertically) {
            Icon(Icons.Default.People, contentDescription = null,
                modifier = Modifier.size(14.dp),
                tint = MaterialTheme.colorScheme.onSurfaceVariant)
            Spacer(modifier = Modifier.width(2.dp))
            Text(participantCount.toString(), style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant)
        }

        // Elapsed call duration
        Text(
            elapsedText,
            style = MaterialTheme.typography.labelSmall.copy(fontFamily = FontFamily.Monospace),
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )

        // Connection state dot
        Box(
            modifier = Modifier
                .size(8.dp)
                .clip(CircleShape)
                .background(
                    when (connectionState) {
                        ConnectionState.CONNECTED -> MaterialTheme.colorScheme.primary
                        ConnectionState.RECONNECTING -> MaterialTheme.colorScheme.tertiary
                        else -> MaterialTheme.colorScheme.error
                    }
                )
        )
    }
}

@Composable
private fun ChatPanel(
    messages: List<ChatMessage>,
    chatInput: String,
    onChatInputChange: (String) -> Unit,
    onSend: () -> Unit,
    onClose: () -> Unit,
    roomId: String? = null,
    roomApi: RoomApi? = null,
    onSendWithAttachment: ((String, com.bedrud.app.core.livekit.ChatAttachment) -> Unit)? = null,
) {
    val listState = rememberLazyListState()
    val scope = rememberCoroutineScope()
    val context = LocalContext.current

    // Detect whether user has scrolled away from the bottom
    val isAtBottom by remember {
        derivedStateOf {
            val lastVisible = listState.layoutInfo.visibleItemsInfo.lastOrNull()?.index ?: 0
            lastVisible >= messages.size - 2
        }
    }

    // Auto-scroll to bottom only when following
    LaunchedEffect(messages.size) {
        if (messages.isNotEmpty() && isAtBottom) {
            listState.animateScrollToItem(messages.size - 1)
        }
    }

    var isUploading by remember { mutableStateOf(false) }
    var uploadError by remember { mutableStateOf<String?>(null) }

    val imagePicker = rememberLauncherForActivityResult(
        ActivityResultContracts.PickVisualMedia()
    ) { uri ->
        if (uri == null || roomId == null || roomApi == null) return@rememberLauncherForActivityResult
        scope.launch {
            isUploading = true
            uploadError = null
            try {
                val stream = context.contentResolver.openInputStream(uri)
                    ?: throw Exception("Cannot open image")
                val bytes = stream.readBytes()
                stream.close()
                val mimeType = context.contentResolver.getType(uri) ?: "image/jpeg"
                val ext = when (mimeType) {
                    "image/png" -> "png"
                    "image/gif" -> "gif"
                    "image/webp" -> "webp"
                    else -> "jpg"
                }
                val requestBody = bytes.toRequestBody(mimeType.toMediaTypeOrNull())
                val part = MultipartBody.Part.createFormData("file", "upload.$ext", requestBody)
                val response = roomApi.uploadChatImage(roomId, part)
                if (response.isSuccessful) {
                    val result = response.body()!!
                    val attachment = com.bedrud.app.core.livekit.ChatAttachment(
                        kind = "image",
                        url = result.url,
                        mime = result.mime,
                        w = result.width,
                        h = result.height,
                        size = result.size,
                    )
                    onSendWithAttachment?.invoke(chatInput.trim(), attachment)
                    onChatInputChange("")
                } else {
                    uploadError = "Upload failed (${response.code()})"
                }
            } catch (e: Exception) {
                uploadError = e.message ?: "Upload failed"
            } finally {
                isUploading = false
            }
        }
    }

    Column(
        modifier = Modifier
            .width(300.dp)
            .fillMaxHeight()
            .background(MaterialTheme.colorScheme.surface)
    ) {
        // Chat header
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(horizontal = 12.dp, vertical = 8.dp),
            verticalAlignment = Alignment.CenterVertically,
            horizontalArrangement = Arrangement.SpaceBetween
        ) {
            Text(
                text = stringResource(R.string.meeting_panel_chat),
                style = MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onSurface
            )
            IconButton(onClick = onClose) {
                Icon(Icons.Default.Close, contentDescription = stringResource(R.string.meeting_contentDescription_closeChat))
            }
        }

        // Messages list + scroll-to-bottom button
        Box(
            modifier = Modifier
                .weight(1f)
                .fillMaxWidth()
        ) {
            LazyColumn(
                state = listState,
                modifier = Modifier
                    .fillMaxSize()
                    .padding(horizontal = 12.dp),
                verticalArrangement = Arrangement.spacedBy(8.dp)
            ) {
                items(messages) { message ->
                    ChatBubble(message = message)
                }
            }

            // Scroll-to-bottom FAB when user scrolled up
            if (!isAtBottom) {
                SmallFloatingActionButton(
                    onClick = {
                        scope.launch {
                            if (messages.isNotEmpty()) listState.animateScrollToItem(messages.size - 1)
                        }
                    },
                    modifier = Modifier
                        .align(Alignment.BottomEnd)
                        .padding(8.dp),
                    containerColor = MaterialTheme.colorScheme.primaryContainer,
                ) {
                    Icon(
                        Icons.Default.KeyboardArrowDown,
                        contentDescription = stringResource(R.string.meeting_contentDescription_scrollToBottom),
                        tint = MaterialTheme.colorScheme.onPrimaryContainer,
                    )
                }
            }
        }

        // Upload status
        if (isUploading) {
            Row(
                modifier = Modifier.padding(horizontal = 12.dp, vertical = 4.dp),
                verticalAlignment = Alignment.CenterVertically,
                horizontalArrangement = Arrangement.spacedBy(6.dp)
            ) {
                CircularProgressIndicator(modifier = Modifier.size(14.dp), strokeWidth = 2.dp)
                Text(text = stringResource(R.string.meeting_chat_uploading), style = MaterialTheme.typography.labelSmall)
            }
        }
        uploadError?.let { err ->
            Text(
                text = err,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.error,
                modifier = Modifier.padding(horizontal = 12.dp, vertical = 2.dp)
            )
        }

        // Input row
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(8.dp),
            verticalAlignment = Alignment.CenterVertically
        ) {
            // Image picker button
            if (roomApi != null && roomId != null) {
                IconButton(
                    onClick = {
                        imagePicker.launch(
                            PickVisualMediaRequest(ActivityResultContracts.PickVisualMedia.ImageOnly)
                        )
                    },
                    enabled = !isUploading,
                ) {
                    Icon(
                        Icons.Default.Image,
                        contentDescription = stringResource(R.string.meeting_contentDescription_attachImage),
                        tint = if (!isUploading) MaterialTheme.colorScheme.onSurfaceVariant
                               else MaterialTheme.colorScheme.outline,
                    )
                }
            }
            OutlinedTextField(
                value = chatInput,
                onValueChange = onChatInputChange,
                placeholder = { Text(stringResource(R.string.meeting_chat_placeholder)) },
                singleLine = true,
                modifier = Modifier.weight(1f),
                shape = RoundedCornerShape(24.dp),
                textStyle = MaterialTheme.typography.bodyMedium.copy(textDirection = TextDirection.Content)
            )
            Spacer(modifier = Modifier.width(4.dp))
            IconButton(
                onClick = onSend,
                enabled = chatInput.isNotBlank() && !isUploading
            ) {
                Icon(
                    Icons.AutoMirrored.Filled.Send,
                    contentDescription = stringResource(R.string.meeting_contentDescription_send),
                    tint = if (chatInput.isNotBlank() && !isUploading)
                        MaterialTheme.colorScheme.primary
                    else MaterialTheme.colorScheme.onSurfaceVariant
                )
            }
        }
    }
}

@Composable
private fun ChatBubble(message: ChatMessage) {
    Column(
        modifier = Modifier.fillMaxWidth(),
        horizontalAlignment = if (message.isLocal) Alignment.End else Alignment.Start
    ) {
        Text(
            text = message.senderName,
            style = MaterialTheme.typography.labelSmall.copy(textDirection = TextDirection.Content),
            color = MaterialTheme.colorScheme.onSurfaceVariant
        )
        Spacer(modifier = Modifier.height(2.dp))
        Column(
            horizontalAlignment = if (message.isLocal) Alignment.End else Alignment.Start
        ) {
            // Image attachments
            message.attachments.filter { it.kind == "image" }.forEach { att ->
                val isDataUri = att.url.startsWith("data:")
                if (isDataUri) {
                    // Decode base64 data URI to bitmap in-memory
                    val bitmap = remember(att.url) {
                        runCatching {
                            val b64 = att.url.substringAfter(",")
                            val bytes = android.util.Base64.decode(b64, android.util.Base64.DEFAULT)
                            android.graphics.BitmapFactory.decodeByteArray(bytes, 0, bytes.size)
                                ?.asImageBitmap()
                        }.getOrNull()
                    }
                    if (bitmap != null) {
                        Image(
                            bitmap = bitmap,
                            contentDescription = stringResource(R.string.meeting_chat_sharedImage),
                            modifier = Modifier
                                .fillMaxWidth(0.8f)
                                .clip(RoundedCornerShape(10.dp)),
                            contentScale = ContentScale.FillWidth,
                        )
                    }
                } else {
                    AsyncImage(
                        model = att.url,
                        contentDescription = stringResource(R.string.meeting_chat_sharedImage),
                        modifier = Modifier
                            .fillMaxWidth(0.8f)
                            .clip(RoundedCornerShape(10.dp)),
                        contentScale = ContentScale.FillWidth,
                    )
                }
                Spacer(modifier = Modifier.height(4.dp))
            }
            // Text content
            if (message.text.isNotEmpty()) {
                Box(
                    modifier = Modifier
                        .background(
                            if (message.isLocal) MaterialTheme.colorScheme.primaryContainer
                            else MaterialTheme.colorScheme.surfaceVariant,
                            RoundedCornerShape(12.dp)
                        )
                        .padding(horizontal = 12.dp, vertical = 8.dp)
                ) {
                    Text(
                        text = BidiUtils.wrap(message.text),
                        style = MaterialTheme.typography.bodyMedium.copy(textDirection = TextDirection.Content),
                        color = if (message.isLocal) MaterialTheme.colorScheme.onPrimaryContainer
                        else MaterialTheme.colorScheme.onSurfaceVariant
                    )
                }
            }
        }
    }
}
