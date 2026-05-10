package com.bedrud.app.ui.screens.dashboard

import androidx.compose.animation.animateColorAsState
import androidx.compose.animation.core.tween
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.ExperimentalLayoutApi
import androidx.compose.foundation.layout.FlowRow
import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.Row
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
import androidx.compose.material.icons.automirrored.filled.Chat
import androidx.compose.material.icons.filled.Groups
import androidx.compose.material.icons.filled.Lock
import androidx.compose.material.icons.filled.Mic
import androidx.compose.material.icons.filled.Refresh
import androidx.compose.material.icons.filled.Search
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.Shield
import androidx.compose.material.icons.filled.Videocam
import androidx.compose.material3.AssistChip
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ElevatedCard
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.FilterChip
import androidx.compose.material3.FloatingActionButton
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LargeTopAppBar
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Surface
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBarDefaults
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
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.input.nestedscroll.nestedScroll
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextDirection
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.bedrud.app.R
import com.bedrud.app.core.instance.InstanceManager
import com.bedrud.app.models.CreateRoomRequest
import com.bedrud.app.models.UserRoomResponse
import kotlinx.coroutines.launch
import org.koin.compose.koinInject

// ── Filter state ─────────────────────────────────────────────────────────────

private enum class RoomFilter { ALL, ACTIVE, PRIVATE }

// ── Screen entry point ────────────────────────────────────────────────────────

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun DashboardContent(
    modifier: Modifier = Modifier,
    onJoinRoom: (String) -> Unit,
    instanceManager: InstanceManager = koinInject()
) {
    val roomApi = instanceManager.roomApi.collectAsState().value ?: return
    val scope = rememberCoroutineScope()
    val snackbarHostState = remember { SnackbarHostState() }

    var rooms by remember { mutableStateOf<List<UserRoomResponse>>(emptyList()) }
    var isLoading by remember { mutableStateOf(true) }
    var showCreateDialog by remember { mutableStateOf(false) }
    var roomToEdit by remember { mutableStateOf<UserRoomResponse?>(null) }
    var activeFilter by rememberSaveable { mutableStateOf(RoomFilter.ALL) }
    var quickJoinText by remember { mutableStateOf("") }

    val scrollBehavior = TopAppBarDefaults.exitUntilCollapsedScrollBehavior()

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

    val filteredRooms = remember(rooms, activeFilter) {
        when (activeFilter) {
            RoomFilter.ALL -> rooms
            RoomFilter.ACTIVE -> rooms.filter { it.isActive }
            RoomFilter.PRIVATE -> rooms.filter { it.isPublic == false }
        }
    }

    Scaffold(
        modifier = modifier,
        topBar = {
            LargeTopAppBar(
                title = { Text(stringResource(R.string.dashboard_title_rooms)) },
                actions = {
                    IconButton(onClick = { loadRooms() }) {
                        Icon(Icons.Default.Refresh, contentDescription = stringResource(R.string.dashboard_contentDescription_refresh))
                    }
                },
                scrollBehavior = scrollBehavior
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
        if (isLoading && rooms.isEmpty()) {
            Box(
                modifier = Modifier.fillMaxSize().padding(innerPadding),
                contentAlignment = Alignment.Center
            ) { CircularProgressIndicator() }
        } else {
            LazyColumn(
                modifier = Modifier
                    .fillMaxSize()
                    .padding(innerPadding)
                    .nestedScroll(scrollBehavior.nestedScrollConnection),
                contentPadding = PaddingValues(bottom = 88.dp)
            ) {
                // ── Stats row ─────────────────────────────────────
                item {
                    StatsRow(rooms = rooms, modifier = Modifier.padding(horizontal = 16.dp, vertical = 8.dp))
                }

                // ── Quick join bar ────────────────────────────────
                item {
                    QuickJoinBar(
                        value = quickJoinText,
                        onValueChange = { quickJoinText = it },
                        onJoin = {
                            val slug = quickJoinText.trim()
                                .removePrefix("https://bedrud.com/m/")
                                .removePrefix("http://bedrud.com/m/")
                                .removePrefix("https://bedrud.com/c/")
                                .trim('/')
                            if (slug.isNotBlank()) {
                                quickJoinText = ""
                                onJoinRoom(slug)
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
                if (filteredRooms.isEmpty() && !isLoading) {
                    item {
                        EmptyState(
                            hasFilter = activeFilter != RoomFilter.ALL,
                            onCreateRoom = { showCreateDialog = true }
                        )
                    }
                } else {
                    items(filteredRooms, key = { it.id }) { room ->
                        RoomCard(
                            room = room,
                            onJoin = { onJoinRoom(room.name) },
                            onDelete = {
                                scope.launch {
                                    try {
                                        val response = roomApi.deleteRoom(room.id)
                                        if (response.isSuccessful) {
                                            rooms = rooms.filter { it.id != room.id }
                                        } else {
                                            snackbarHostState.showSnackbar("Failed to delete room")
                                        }
                                    } catch (e: Exception) {
                                        snackbarHostState.showSnackbar(e.message ?: "Failed to delete room")
                                    }
                                }
                            },
                            onSettings = if (room.relationship == "owner") {
                                { roomToEdit = room }
                            } else null,
                            modifier = Modifier.padding(horizontal = 16.dp, vertical = 6.dp)
                        )
                    }
                }
            }
        }
    }
}

// ── Stats row ─────────────────────────────────────────────────────────────────

@Composable
private fun StatsRow(rooms: List<UserRoomResponse>, modifier: Modifier = Modifier) {
    Row(modifier = modifier, horizontalArrangement = Arrangement.spacedBy(8.dp)) {
        StatChip(label = stringResource(R.string.dashboard_stat_total), count = rooms.size, modifier = Modifier.weight(1f))
        StatChip(label = stringResource(R.string.dashboard_stat_live), count = rooms.count { it.isActive }, modifier = Modifier.weight(1f))
        StatChip(label = stringResource(R.string.dashboard_stat_private), count = rooms.count { it.isPublic == false }, modifier = Modifier.weight(1f))
    }
}

@Composable
private fun StatChip(label: String, count: Int, modifier: Modifier = Modifier) {
    Surface(
        modifier = modifier.clip(RoundedCornerShape(12.dp)),
        color = MaterialTheme.colorScheme.surfaceVariant,
        shape = RoundedCornerShape(12.dp)
    ) {
        Column(
            modifier = Modifier.padding(vertical = 10.dp),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            Text(
                text = count.toString(),
                style = MaterialTheme.typography.titleLarge,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
            Text(
                text = label,
                style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
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
                        }
                    )
                }
            )
        }
    }
}

// ── Room card ─────────────────────────────────────────────────────────────────

@OptIn(ExperimentalLayoutApi::class)
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

    ElevatedCard(
        onClick = onJoin,
        shape = RoundedCornerShape(16.dp),
        colors = CardDefaults.elevatedCardColors(containerColor = MaterialTheme.colorScheme.surface),
        modifier = modifier.fillMaxWidth()
    ) {
        Column(modifier = Modifier.padding(16.dp)) {
            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.SpaceBetween,
                verticalAlignment = Alignment.CenterVertically
            ) {
                Column(modifier = Modifier.weight(1f)) {
                    Text(
                        text = title,
                        style = MaterialTheme.typography.titleMedium.copy(
                            fontFamily = FontFamily.Monospace,
                            textDirection = TextDirection.Ltr
                        ),
                        maxLines = 1,
                        overflow = TextOverflow.Ellipsis
                    )
                    Spacer(modifier = Modifier.height(2.dp))
                    Text(
                        text = if (room.isActive) stringResource(R.string.dashboard_status_live) else stringResource(R.string.dashboard_status_idle),
                        style = MaterialTheme.typography.bodySmall,
                        color = activeTint
                    )
                }
                Row {
                    if (onSettings != null) {
                        IconButton(onClick = onSettings, modifier = Modifier.size(32.dp)) {
                            Icon(Icons.Default.Settings, contentDescription = stringResource(R.string.dashboard_contentDescription_settings),
                                modifier = Modifier.size(18.dp),
                                tint = MaterialTheme.colorScheme.onSurfaceVariant)
                        }
                    }
                }
            }

            Spacer(modifier = Modifier.height(10.dp))

            // Feature badge pills
            FlowRow(horizontalArrangement = Arrangement.spacedBy(6.dp)) {
                if (room.settings.allowChat)  FeaturePill(Icons.AutoMirrored.Filled.Chat, stringResource(R.string.dashboard_feature_chat))
                if (room.settings.allowVideo) FeaturePill(Icons.Default.Videocam, stringResource(R.string.dashboard_feature_video))
                if (room.settings.allowAudio) FeaturePill(Icons.Default.Mic, stringResource(R.string.dashboard_feature_audio))
                if (room.settings.e2ee)       FeaturePill(Icons.Default.Shield, stringResource(R.string.dashboard_feature_e2ee))
                if (room.isPublic == false)   FeaturePill(Icons.Default.Lock, stringResource(R.string.dashboard_feature_private))
            }

            Spacer(modifier = Modifier.height(10.dp))

            Row(
                modifier = Modifier.fillMaxWidth(),
                horizontalArrangement = Arrangement.End,
                verticalAlignment = Alignment.CenterVertically
            ) {
                TextButton(onClick = onDelete) {
                    Text(stringResource(R.string.common_button_delete), color = MaterialTheme.colorScheme.error)
                }
                Spacer(modifier = Modifier.width(8.dp))
                FilledTonalButton(onClick = onJoin) { Text(stringResource(R.string.common_button_join)) }
            }
        }
    }
}

@Composable
private fun FeaturePill(icon: ImageVector, label: String) {
    AssistChip(
        onClick = {},
        label = { Text(label, style = MaterialTheme.typography.labelSmall) },
        leadingIcon = { Icon(icon, contentDescription = null, modifier = Modifier.size(14.dp)) }
    )
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
