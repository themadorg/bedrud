package com.bedrud.app.ui.screens.dashboard

import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.WindowInsets
import androidx.compose.foundation.layout.ime
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.lazy.LazyColumn
import androidx.compose.foundation.lazy.items
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.Add
import androidx.compose.material.icons.filled.ChevronRight
import androidx.compose.material.icons.filled.Close
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.Groups
import androidx.compose.material.icons.filled.History
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Search
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material3.AlertDialog
import androidx.compose.material3.CircularProgressIndicator
import com.bedrud.app.ui.components.BedrudOutlinedCard
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import com.bedrud.app.ui.components.BedrudCompactTopBar
import com.bedrud.app.ui.components.BedrudTabScaffoldContentInsets
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import com.bedrud.app.ui.components.BedrudButton
import com.bedrud.app.ui.components.BedrudButtonVariant
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.platform.LocalDensity
import androidx.compose.ui.draw.clip
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextDirection
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.bedrud.app.R
import com.bedrud.app.core.deeplink.BedrudURLParser
import com.bedrud.app.core.instance.InstanceManager
import com.bedrud.app.core.recent.RecentRoom
import com.bedrud.app.core.recent.RecentRoomsStore
import com.bedrud.app.core.recent.formatRecentRoomTimeAgo
import com.bedrud.app.core.recent.recentRoomsNotInApiList
import com.bedrud.app.models.CreateRoomRequest
import com.bedrud.app.models.UserRoomResponse
import kotlinx.coroutines.launch
import org.koin.compose.koinInject

// ── Filter state ─────────────────────────────────────────────────────────────

private enum class RoomFilter { ALL, ACTIVE, PRIVATE, RECENT }

private sealed interface RoomListEntry {
    data class FromApi(val room: UserRoomResponse) : RoomListEntry
    data class FromRecent(val recent: RecentRoom) : RoomListEntry
}

// ── Screen entry point ────────────────────────────────────────────────────────

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun DashboardContent(
    modifier: Modifier = Modifier,
    onJoinRoom: (String) -> Unit,
    onJoinRecent: (RecentRoom) -> Unit,
    instanceManager: InstanceManager = koinInject(),
    recentRoomsStore: RecentRoomsStore = koinInject(),
) {
    val roomApi = instanceManager.roomApi.collectAsState().value ?: return
    val recentRooms by recentRoomsStore.rooms.collectAsState()
    val activeInstanceId by instanceManager.store.activeInstanceId.collectAsState()
    val scope = rememberCoroutineScope()
    val snackbarHostState = remember { SnackbarHostState() }

    var rooms by remember { mutableStateOf<List<UserRoomResponse>>(emptyList()) }
    var isLoading by remember { mutableStateOf(true) }
    var showCreateDialog by remember { mutableStateOf(false) }
    var roomToEdit by remember { mutableStateOf<UserRoomResponse?>(null) }
    var roomToDelete by remember { mutableStateOf<UserRoomResponse?>(null) }
    var activeFilter by rememberSaveable { mutableStateOf(RoomFilter.ALL) }
    var quickJoinText by remember { mutableStateOf("") }

    fun loadRooms() {
        scope.launch {
            isLoading = true
            try {
                val response = roomApi.listRooms()
                if (response.isSuccessful) {
                    rooms = response.body() ?: emptyList()
                } else {
                    snackbarHostState.showSnackbar("Failed to load rooms")
                }
            } catch (e: Exception) {
                snackbarHostState.showSnackbar(e.message ?: "Failed to load rooms")
            } finally {
                isLoading = false
            }
        }
    }

    LaunchedEffect(Unit) { loadRooms() }

    if (showCreateDialog) {
        CreateRoomDialog(
            onDismiss = { showCreateDialog = false },
            onCreate = { name ->
                scope.launch {
                    try {
                        val response = roomApi.createRoom(CreateRoomRequest(name = name.ifBlank { null }))
                        if (response.isSuccessful) {
                            val room = response.body()!!
                            showCreateDialog = false
                            onJoinRoom(room.name)
                        } else {
                            snackbarHostState.showSnackbar("Failed to create room")
                        }
                    } catch (e: Exception) {
                        snackbarHostState.showSnackbar(e.message ?: "Failed to create room")
                    }
                }
            }
        )
    }

    roomToDelete?.let { room ->
        val title = room.name.ifEmpty { room.id }
        AlertDialog(
            onDismissRequest = { roomToDelete = null },
            title = { Text(stringResource(R.string.dashboard_dialog_deleteTitle)) },
            text = {
                Text(stringResource(R.string.dashboard_dialog_deleteMessage, title))
            },
            confirmButton = {
                TextButton(
                    onClick = {
                        val deleting = room
                        roomToDelete = null
                        scope.launch {
                            try {
                                val response = roomApi.deleteRoom(deleting.id)
                                if (response.isSuccessful) {
                                    rooms = rooms.filter { it.id != deleting.id }
                                } else {
                                    snackbarHostState.showSnackbar("Failed to delete room")
                                }
                            } catch (e: Exception) {
                                snackbarHostState.showSnackbar(e.message ?: "Failed to delete room")
                            }
                        }
                    },
                ) {
                    Text(
                        stringResource(R.string.common_button_delete),
                        color = MaterialTheme.colorScheme.error,
                    )
                }
            },
            dismissButton = {
                TextButton(onClick = { roomToDelete = null }) {
                    Text(stringResource(R.string.common_button_cancel))
                }
            },
        )
    }

    roomToEdit?.let { room ->
        RoomSettingsDialog(
            room = room,
            onDismiss = { roomToEdit = null },
            onSave = { settings ->
                scope.launch {
                    try {
                        val response = roomApi.updateRoomSettings(room.id, settings)
                        if (response.isSuccessful) {
                            roomToEdit = null
                            loadRooms()
                            snackbarHostState.showSnackbar("Settings saved")
                        } else {
                            snackbarHostState.showSnackbar("Failed to save settings")
                        }
                    } catch (e: Exception) {
                        snackbarHostState.showSnackbar(e.message ?: "Failed to save settings")
                    }
                }
            }
        )
    }

    val isKeyboardVisible = WindowInsets.ime.getBottom(LocalDensity.current) > 0

    val filteredRooms = remember(rooms, activeFilter) {
        when (activeFilter) {
            RoomFilter.ALL -> rooms
            RoomFilter.ACTIVE -> rooms.filter { it.isActive }
            RoomFilter.PRIVATE -> rooms.filter { it.isPublic == false }
            RoomFilter.RECENT -> emptyList()
        }
    }

    val allTabEntries = remember(rooms, recentRooms, activeInstanceId) {
        val recentOnly = recentRoomsNotInApiList(
            recentRooms,
            rooms.map { it.name }.toSet(),
            activeInstanceId,
        )
        rooms.map { RoomListEntry.FromApi(it) } + recentOnly.map { RoomListEntry.FromRecent(it) }
    }

    Scaffold(
        modifier = modifier,
        contentWindowInsets = BedrudTabScaffoldContentInsets,
        topBar = {
            BedrudCompactTopBar(
                title = stringResource(R.string.dashboard_title_rooms),
                actions = {
                    IconButton(onClick = { loadRooms() }) {
                        Icon(
                            Icons.Default.Refresh,
                            contentDescription = stringResource(R.string.dashboard_contentDescription_refresh),
                        )
                    }
                },
            )
        },
        floatingActionButton = {
            FloatingActionButton(
                onClick = { showCreateDialog = true },
                containerColor = MaterialTheme.colorScheme.primaryContainer,
                contentColor = MaterialTheme.colorScheme.onPrimaryContainer
            ) { Icon(Icons.Default.Add, contentDescription = stringResource(R.string.dashboard_contentDescription_createRoom)) }
        },
        snackbarHost = { SnackbarHost(snackbarHostState) }
    ) { innerPadding ->
        if (isLoading && rooms.isEmpty() && recentRooms.isEmpty()) {
            Box(
                modifier = Modifier.fillMaxSize().padding(innerPadding),
                contentAlignment = Alignment.Center
            ) { CircularProgressIndicator() }
        } else {
            LazyColumn(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(innerPadding),
                contentPadding = PaddingValues(
                    bottom = if (isKeyboardVisible) 16.dp else 88.dp,
                )
            ) {
                // ── Quick join bar ────────────────────────────────
                item {
                    QuickJoinBar(
                        value = quickJoinText,
                        onValueChange = { quickJoinText = it },
                        onJoin = {
                            val roomName = BedrudURLParser.parseJoinInput(quickJoinText)
                            if (!roomName.isNullOrBlank()) {
                                quickJoinText = ""
                                onJoinRoom(roomName)
                            }
                        },
                        modifier = Modifier.padding(horizontal = 16.dp, vertical = 4.dp)
                    )
                }

                // ── Filter tabs ───────────────────────────────────
                item {
                    FilterRow(
                        activeFilter = activeFilter,
                        onFilterChange = { activeFilter = it },
                        modifier = Modifier.padding(horizontal = 16.dp, vertical = 4.dp)
                    )
                }

                // ── Room list ─────────────────────────────────────
                if (activeFilter == RoomFilter.RECENT) {
                    if (recentRooms.isEmpty()) {
                        item {
                            RecentEmptyState()
                        }
                    } else {
                        items(
                            recentRooms,
                            key = { "${it.instanceId}:${it.roomName}" },
                        ) { recent ->
                            RecentRoomCard(
                                recent = recent,
                                isCurrentServer = recent.instanceId == activeInstanceId,
                                onJoin = { onJoinRecent(recent) },
                                onRemove = {
                                    recentRoomsStore.remove(recent.roomName, recent.instanceId)
                                },
                                modifier = Modifier.padding(horizontal = 16.dp, vertical = 4.dp),
                            )
                        }
                    }
                } else if (activeFilter == RoomFilter.ALL) {
                    if (allTabEntries.isEmpty() && !isLoading) {
                        item {
                            EmptyState(
                                hasFilter = false,
                                onCreateRoom = { showCreateDialog = true },
                            )
                        }
                    } else {
                        items(
                            allTabEntries,
                            key = { entry ->
                                when (entry) {
                                    is RoomListEntry.FromApi -> "api:${entry.room.id}"
                                    is RoomListEntry.FromRecent ->
                                        "recent:${entry.recent.instanceId}:${entry.recent.roomName}"
                                }
                            },
                        ) { entry ->
                            when (entry) {
                                is RoomListEntry.FromApi -> RoomCard(
                                    room = entry.room,
                                    onJoin = { onJoinRoom(entry.room.name) },
                                    onDelete = { roomToDelete = entry.room },
                                    onSettings = if (entry.room.relationship == "owner") {
                                        { roomToEdit = entry.room }
                                    } else null,
                                    modifier = Modifier.padding(horizontal = 16.dp, vertical = 4.dp),
                                )
                                is RoomListEntry.FromRecent -> RecentRoomCard(
                                    recent = entry.recent,
                                    isCurrentServer = entry.recent.instanceId == activeInstanceId,
                                    onJoin = { onJoinRecent(entry.recent) },
                                    onRemove = {
                                        recentRoomsStore.remove(
                                            entry.recent.roomName,
                                            entry.recent.instanceId,
                                        )
                                    },
                                    modifier = Modifier.padding(horizontal = 16.dp, vertical = 4.dp),
                                )
                            }
                        }
                    }
                } else if (filteredRooms.isEmpty() && !isLoading) {
                    item {
                        EmptyState(
                            hasFilter = true,
                            onCreateRoom = { showCreateDialog = true }
                        )
                    }
                } else {
                    items(filteredRooms, key = { it.id }) { room ->
                        RoomCard(
                            room = room,
                            onJoin = { onJoinRoom(room.name) },
                            onDelete = { roomToDelete = room },
                            onSettings = if (room.relationship == "owner") {
                                { roomToEdit = room }
                            } else null,
                            modifier = Modifier.padding(horizontal = 16.dp, vertical = 4.dp)
                        )
                    }
                }
            }
        }
    }
}

// ── Quick join bar ────────────────────────────────────────────────────────────

@Composable
private fun QuickJoinBar(
    value: String,
    onValueChange: (String) -> Unit,
    onJoin: () -> Unit,
    modifier: Modifier = Modifier
) {
    Row(modifier = modifier, verticalAlignment = Alignment.CenterVertically) {
        OutlinedTextField(
            value = value,
            onValueChange = onValueChange,
            placeholder = { Text(stringResource(R.string.dashboard_placeholder_search)) },
            leadingIcon = { Icon(Icons.Default.Search, contentDescription = null, modifier = Modifier.size(18.dp)) },
            singleLine = true,
            shape = RoundedCornerShape(12.dp),
            modifier = Modifier.weight(1f)
        )
        Spacer(modifier = Modifier.width(8.dp))
        FilledTonalButton(
            onClick = onJoin,
            enabled = value.isNotBlank()
        ) { Text(stringResource(R.string.common_button_join)) }
    }
}

// ── Filter tabs ───────────────────────────────────────────────────────────────

@Composable
private fun FilterRow(
    activeFilter: RoomFilter,
    onFilterChange: (RoomFilter) -> Unit,
    modifier: Modifier = Modifier
) {
    Row(modifier = modifier, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        RoomFilter.entries.forEach { filter ->
            FilterChip(
                selected = activeFilter == filter,
                onClick = { onFilterChange(filter) },
                label = {
                    Text(
                        when (filter) {
                            RoomFilter.ALL -> stringResource(R.string.dashboard_filter_all)
                            RoomFilter.ACTIVE -> stringResource(R.string.dashboard_filter_active)
                            RoomFilter.PRIVATE -> stringResource(R.string.dashboard_filter_private)
                            RoomFilter.RECENT -> stringResource(R.string.dashboard_filter_recent)
                        }
                    )
                }
            )
        }
    }
}

// ── Room card ─────────────────────────────────────────────────────────────────

@Composable
private fun RoomCard(
    room: UserRoomResponse,
    onJoin: () -> Unit,
    onDelete: () -> Unit,
    onSettings: (() -> Unit)? = null,
    modifier: Modifier = Modifier
) {
    val title = room.name.ifEmpty {
        val parts = room.id.split("-")
        if (parts.size >= 2) "${parts[0]}-${parts[1]}" else room.id
    }

    val activeTint by animateColorAsState(
        targetValue = if (room.isActive) MaterialTheme.colorScheme.primary
        else MaterialTheme.colorScheme.onSurfaceVariant,
        animationSpec = tween(400),
        label = "activeTint"
    )

    val statusText = if (room.isActive) {
        stringResource(R.string.dashboard_status_live)
    } else {
        stringResource(R.string.dashboard_status_idle)
    }
    val metaText = if (room.isPublic == false) {
        "$statusText · ${stringResource(R.string.dashboard_feature_private)}"
    } else {
        statusText
    }

    BedrudOutlinedCard(
        onClick = onJoin,
        modifier = modifier.fillMaxWidth(),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(start = 14.dp, end = 4.dp, top = 10.dp, bottom = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = title,
                    style = MaterialTheme.typography.bodyLarge.copy(
                        fontFamily = FontFamily.Monospace,
                        textDirection = TextDirection.Ltr,
                    ),
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    text = metaText,
                    style = MaterialTheme.typography.labelSmall,
                    color = activeTint,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }

            if (onSettings != null) {
                IconButton(
                    onClick = onSettings,
                    modifier = Modifier.size(36.dp),
                ) {
                    Icon(
                        Icons.Default.Settings,
                        contentDescription = stringResource(R.string.dashboard_contentDescription_settings),
                        modifier = Modifier.size(18.dp),
                        tint = MaterialTheme.colorScheme.onSurfaceVariant,
                    )
                }
            }

            IconButton(
                onClick = onDelete,
                modifier = Modifier.size(36.dp),
            ) {
                Icon(
                    Icons.Default.Delete,
                    contentDescription = stringResource(R.string.common_button_delete),
                    modifier = Modifier.size(18.dp),
                    tint = MaterialTheme.colorScheme.error,
                )
            }

            Icon(
                Icons.Default.ChevronRight,
                contentDescription = null,
                modifier = Modifier
                    .padding(end = 6.dp)
                    .size(20.dp),
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

// ── Recent room card ──────────────────────────────────────────────────────────

@Composable
private fun RecentRoomCard(
    recent: RecentRoom,
    isCurrentServer: Boolean,
    onJoin: () -> Unit,
    onRemove: () -> Unit,
    modifier: Modifier = Modifier,
) {
    val metaText = if (isCurrentServer) {
        formatRecentRoomTimeAgo(recent.joinedAt)
    } else {
        "${formatRecentRoomTimeAgo(recent.joinedAt)} · ${
            stringResource(R.string.dashboard_recent_onServer, recent.instanceName)
        }"
    }

    BedrudOutlinedCard(
        onClick = onJoin,
        modifier = modifier.fillMaxWidth(),
    ) {
        Row(
            modifier = Modifier
                .fillMaxWidth()
                .padding(start = 14.dp, end = 4.dp, top = 10.dp, bottom = 10.dp),
            verticalAlignment = Alignment.CenterVertically,
        ) {
            Icon(
                Icons.Default.History,
                contentDescription = null,
                modifier = Modifier.size(18.dp),
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Spacer(modifier = Modifier.width(10.dp))
            Column(modifier = Modifier.weight(1f)) {
                Text(
                    text = recent.roomName,
                    style = MaterialTheme.typography.bodyLarge.copy(
                        fontFamily = FontFamily.Monospace,
                        textDirection = TextDirection.Ltr,
                    ),
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
                Text(
                    text = metaText,
                    style = MaterialTheme.typography.labelSmall,
                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                    maxLines = 1,
                    overflow = TextOverflow.Ellipsis,
                )
            }

            IconButton(
                onClick = onRemove,
                modifier = Modifier.size(36.dp),
            ) {
                Icon(
                    Icons.Default.Close,
                    contentDescription = stringResource(R.string.dashboard_contentDescription_removeRecent),
                    modifier = Modifier.size(18.dp),
                    tint = MaterialTheme.colorScheme.onSurfaceVariant,
                )
            }

            Icon(
                Icons.Default.ChevronRight,
                contentDescription = null,
                modifier = Modifier
                    .padding(end = 6.dp)
                    .size(20.dp),
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

@Composable
private fun RecentEmptyState() {
    Box(
        modifier = Modifier.fillMaxWidth().padding(vertical = 64.dp),
        contentAlignment = Alignment.Center,
    ) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Icon(
                Icons.Default.History,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.size(64.dp),
            )
            Spacer(modifier = Modifier.height(16.dp))
            Text(
                text = stringResource(R.string.dashboard_empty_noRecent),
                style = MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
            Spacer(modifier = Modifier.height(4.dp))
            Text(
                text = stringResource(R.string.dashboard_empty_noRecentHint),
                style = MaterialTheme.typography.bodySmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
            )
        }
    }
}

// ── Empty state ───────────────────────────────────────────────────────────────

@Composable
private fun EmptyState(hasFilter: Boolean, onCreateRoom: () -> Unit) {
    Box(
        modifier = Modifier.fillMaxWidth().padding(vertical = 64.dp),
        contentAlignment = Alignment.Center
    ) {
        Column(horizontalAlignment = Alignment.CenterHorizontally) {
            Icon(
                Icons.Default.Groups,
                contentDescription = null,
                tint = MaterialTheme.colorScheme.onSurfaceVariant,
                modifier = Modifier.size(64.dp)
            )
            Spacer(modifier = Modifier.height(16.dp))
            Text(
                text = if (hasFilter) stringResource(R.string.dashboard_empty_noMatch) else stringResource(R.string.dashboard_empty_noRooms),
                style = MaterialTheme.typography.titleMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            if (!hasFilter) {
                Spacer(modifier = Modifier.height(4.dp))
                BedrudButton(
                    text = stringResource(R.string.dashboard_button_createFirstRoom),
                    onClick = onCreateRoom,
                    variant = BedrudButtonVariant.OUTLINE
                )
            }
        }
    }
}

// ── Create room dialog ────────────────────────────────────────────────────────

@Composable
private fun CreateRoomDialog(onDismiss: () -> Unit, onCreate: (String) -> Unit) {
    var roomName by remember { mutableStateOf("") }

    androidx.compose.material3.AlertDialog(
        onDismissRequest = onDismiss,
        title = { Text(stringResource(R.string.dashboard_dialog_createTitle)) },
        text = {
            Column {
                Text(stringResource(R.string.dashboard_dialog_createDescription),
                    style = MaterialTheme.typography.bodyMedium)
                Spacer(modifier = Modifier.height(16.dp))
                OutlinedTextField(
                    value = roomName,
                    onValueChange = { roomName = it },
                    label = { Text(stringResource(R.string.dashboard_label_roomName)) },
                    singleLine = true,
                    shape = RoundedCornerShape(12.dp),
                    modifier = Modifier.fillMaxWidth(),
                    textStyle = MaterialTheme.typography.bodyMedium.copy(textDirection = TextDirection.Ltr)
                )
            }
        },
        confirmButton = { TextButton(onClick = { onCreate(roomName) }) { Text(stringResource(
            R.string.common_button_create)) } },
        dismissButton = { TextButton(onClick = onDismiss) { Text(stringResource(R.string.common_button_cancel)) } }
    )
}
