package com.bedrud.app.ui.screens.admin

import androidx.annotation.StringRes
import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.spring
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
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AdminPanelSettings
import androidx.compose.material.icons.filled.Block
import androidx.compose.material.icons.filled.Check
import androidx.compose.material.icons.filled.ContentCopy
import androidx.compose.material.icons.filled.Delete
import androidx.compose.material.icons.filled.Group
import androidx.compose.material.icons.filled.MeetingRoom
import androidx.compose.material.icons.filled.Person
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.Token
import androidx.compose.material3.Button
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ElevatedCard
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.LargeTopAppBar
import androidx.compose.material3.ListItem
import androidx.compose.material3.ListItemDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Switch
import androidx.compose.material3.Text
import androidx.compose.material3.TopAppBarDefaults
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color
import androidx.compose.ui.graphics.TransformOrigin
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.input.nestedscroll.nestedScroll
import androidx.compose.ui.platform.LocalClipboardManager
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.AnnotatedString
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.style.TextDirection
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.unit.dp
import com.bedrud.app.R
import com.bedrud.app.core.instance.InstanceManager
import com.bedrud.app.models.AdminRoom
import com.bedrud.app.models.AdminSettings
import com.bedrud.app.models.AdminUser
import com.bedrud.app.models.CreateInviteTokenRequest
import com.bedrud.app.models.InviteToken
import kotlinx.coroutines.delay
import kotlinx.coroutines.launch
import org.koin.compose.koinInject

// ── Admin root (tab navigation) ───────────────────────────────────────────────

private enum class AdminTab(@StringRes val labelResId: Int, val icon: ImageVector) {
    OVERVIEW(R.string.admin_tab_overview, Icons.Default.AdminPanelSettings),
    USERS(R.string.admin_tab_users, Icons.Default.Group),
    ROOMS(R.string.admin_tab_rooms, Icons.Default.MeetingRoom),
    SETTINGS(R.string.admin_tab_settings, Icons.Default.Settings)
}

@Composable
fun AdminScreen(
    modifier: Modifier = Modifier,
    instanceManager: InstanceManager = koinInject()
) {
    val adminApi = instanceManager.adminApi.collectAsState().value ?: return
    val authManager = instanceManager.authManager.collectAsState().value
    val currentUser by remember(authManager) {
        authManager?.currentUser ?: kotlinx.coroutines.flow.MutableStateFlow(null)
    }.collectAsState()

    // Only show admin panel to admins
    if (currentUser?.isAdmin != true) {
        Box(modifier = Modifier.fillMaxSize(), contentAlignment = Alignment.Center) {
            Text(
                stringResource(R.string.admin_access_denied),
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )
        }
        return
    }

    var selectedTab by rememberSaveable { mutableIntStateOf(0) }
    val navScope = rememberCoroutineScope()

    Scaffold(
        modifier = modifier,
        bottomBar = {
            NavigationBar {
                AdminTab.entries.forEachIndexed { index, tab ->
                    val scale = remember { Animatable(1f) }
                    val isSelected = selectedTab == index
                    NavigationBarItem(
                        selected = isSelected,
                        onClick = {
                            selectedTab = index
                            navScope.launch {
                                scale.animateTo(1.15f, spring(dampingRatio = 0.5f, stiffness = 300f))
                                scale.animateTo(1f, spring(dampingRatio = 0.8f, stiffness = 150f))
                            }
                        },
                        icon = {
                            val baseScale = if (isSelected) 1.5f else 1f
                            Box(
                                modifier = Modifier.graphicsLayer {
                                    scaleX = scale.value * baseScale
                                    scaleY = scale.value * baseScale
                                    transformOrigin = TransformOrigin.Center
                                }
                            ) {
                                Icon(
                                    tab.icon,
                                    contentDescription = stringResource(tab.labelResId),
                                    tint = if (isSelected) MaterialTheme.colorScheme.primary
                                    else MaterialTheme.colorScheme.onSurfaceVariant,
                                    modifier = Modifier.size(24.dp)
                                )
                            }
                        },
                        label = { Text(stringResource(tab.labelResId)) }
                    )
                }
            }
        }
    ) { padding ->
        when (AdminTab.entries[selectedTab]) {
            AdminTab.OVERVIEW -> AdminOverviewContent(
                modifier = Modifier.padding(padding),
                adminApi = adminApi
            )

            AdminTab.USERS -> AdminUsersContent(
                modifier = Modifier.padding(padding),
                adminApi = adminApi
            )

            AdminTab.ROOMS -> AdminRoomsContent(
                modifier = Modifier.padding(padding),
                adminApi = adminApi
            )

            AdminTab.SETTINGS -> AdminSettingsContent(
                modifier = Modifier.padding(padding),
                adminApi = adminApi
            )
        }
    }
}

// ── Overview ──────────────────────────────────────────────────────────────────

@OptIn(ExperimentalMaterial3Api::class, ExperimentalLayoutApi::class)
@Composable
private fun AdminOverviewContent(
    modifier: Modifier = Modifier,
    adminApi: com.bedrud.app.core.api.AdminApi
) {
    val scope = rememberCoroutineScope()
    val snackbarHostState = remember { SnackbarHostState() }
    var users by remember { mutableStateOf<List<AdminUser>>(emptyList()) }
    var rooms by remember { mutableStateOf<List<AdminRoom>>(emptyList()) }
    var onlineCount by remember { mutableIntStateOf(0) }
    var isLoading by remember { mutableStateOf(true) }
    val scrollBehavior = TopAppBarDefaults.exitUntilCollapsedScrollBehavior()

    suspend fun load() {
        isLoading = true
        try {
            users = adminApi.listUsers().body()?.users ?: emptyList()
            rooms = adminApi.listRooms().body()?.rooms ?: emptyList()
            onlineCount = adminApi.getOnlineCount().body()?.get("count") ?: 0
        } catch (e: Exception) {
            snackbarHostState.showSnackbar(e.message ?: "Failed to load data")
        }
        isLoading = false
    }

    LaunchedEffect(adminApi) {
        load()
        // Auto-refresh online count every 30s
        while (true) {
            delay(30_000L)
            try {
                onlineCount = adminApi.getOnlineCount().body()?.get("count") ?: onlineCount
            } catch (_: Exception) {
            }
        }
    }

    Scaffold(
        modifier = modifier,
        topBar = {
            LargeTopAppBar(
                title = { Text(stringResource(R.string.admin_title_overview)) },
                scrollBehavior = scrollBehavior
            )
        },
        snackbarHost = { SnackbarHost(snackbarHostState) }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .nestedScroll(scrollBehavior.nestedScrollConnection)
                .verticalScroll(rememberScrollState())
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            if (isLoading) {
                Box(
                    Modifier
                        .fillMaxWidth()
                        .height(200.dp), contentAlignment = Alignment.Center
                ) {
                    CircularProgressIndicator()
                }
            } else {
                // Stats grid
                FlowRow(
                    horizontalArrangement = Arrangement.spacedBy(12.dp),
                    maxItemsInEachRow = 3
                ) {
                    StatCard(R.string.admin_stat_users, users.size, Icons.Default.Person)
                    StatCard(R.string.admin_stat_activeRooms, rooms.count { it.isActive }, Icons.Default.MeetingRoom)
                    StatCard(R.string.admin_stat_onlineNow, onlineCount, Icons.Default.Group)
                }

                // Recent users
                ElevatedCard(shape = RoundedCornerShape(16.dp)) {
                    Column(modifier = Modifier.padding(16.dp)) {
                        Text(
                            stringResource(R.string.admin_section_recentSignups), style = MaterialTheme.typography.labelLarge,
                            color = MaterialTheme.colorScheme.primary
                        )
                        Spacer(modifier = Modifier.height(8.dp))
                        users.takeLast(5).reversed().forEach { user ->
                            ListItem(
                                headlineContent = {
                                    Text(
                                        user.name,
                                        maxLines = 1,
                                        overflow = TextOverflow.Ellipsis,
                                        style = MaterialTheme.typography.bodyMedium.copy(textDirection = TextDirection.Content)
                                    )
                                },
                                supportingContent = {
                                    Text(
                                        user.email, style = MaterialTheme.typography.bodySmall.copy(textDirection = TextDirection.Ltr),
                                        color = MaterialTheme.colorScheme.onSurfaceVariant
                                    )
                                },
                                leadingContent = {
                                    Icon(
                                        Icons.Default.Person,
                                        contentDescription = null
                                    )
                                },
                                colors = ListItemDefaults.colors(containerColor = Color.Transparent)
                            )
                        }
                    }
                }
            }
        }
    }
}

@Composable
private fun StatCard(@StringRes labelResId: Int, value: Int, icon: ImageVector) {
    ElevatedCard(shape = RoundedCornerShape(12.dp), modifier = Modifier.width(100.dp)) {
        Column(
            modifier = Modifier.padding(12.dp),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            Icon(
                icon, contentDescription = null, modifier = Modifier.size(24.dp),
                tint = MaterialTheme.colorScheme.primary
            )
            Spacer(modifier = Modifier.height(6.dp))
            Text(value.toString(), style = MaterialTheme.typography.titleLarge)
            Text(
                stringResource(labelResId), style = MaterialTheme.typography.labelSmall,
                color = MaterialTheme.colorScheme.onSurfaceVariant,
                maxLines = 1, overflow = TextOverflow.Ellipsis
            )
        }
    }
}

// ── Users ─────────────────────────────────────────────────────────────────────

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun AdminUsersContent(
    modifier: Modifier = Modifier,
    adminApi: com.bedrud.app.core.api.AdminApi
) {
    val scope = rememberCoroutineScope()
    val snackbarHostState = remember { SnackbarHostState() }
    var users by remember { mutableStateOf<List<AdminUser>>(emptyList()) }
    var isLoading by remember { mutableStateOf(true) }
    var search by remember { mutableStateOf("") }
    val scrollBehavior = TopAppBarDefaults.exitUntilCollapsedScrollBehavior()

    LaunchedEffect(Unit) {
        isLoading = true
        try {
            users = adminApi.listUsers().body()?.users ?: emptyList()
        } catch (e: Exception) {
            snackbarHostState.showSnackbar(e.message ?: "Failed to load users")
        }
        isLoading = false
    }

    val filtered = remember(users, search) {
        if (search.isBlank()) users
        else users.filter { it.name.contains(search, true) || it.email.contains(search, true) }
    }

    Scaffold(
        modifier = modifier,
        topBar = { LargeTopAppBar(title = { Text(stringResource(R.string.admin_users)) }, scrollBehavior = scrollBehavior) },
        snackbarHost = { SnackbarHost(snackbarHostState) }
    ) { padding ->
        LazyColumn(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .nestedScroll(scrollBehavior.nestedScrollConnection),
            contentPadding = PaddingValues(bottom = 16.dp)
        ) {
            item {
                OutlinedTextField(
                    value = search, onValueChange = { search = it },
                    placeholder = { Text(stringResource(R.string.admin_placeholder_searchUsers)) },
                    singleLine = true, shape = RoundedCornerShape(12.dp),
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(horizontal = 16.dp, vertical = 8.dp)
                )
            }
            if (isLoading) {
                item {
                    Box(
                        Modifier
                            .fillMaxWidth()
                            .height(200.dp),
                        contentAlignment = Alignment.Center
                    ) {
                        CircularProgressIndicator()
                    }
                }
            } else {
                items(filtered, key = { it.id }) { user ->
                    ListItem(
                        headlineContent = {
                            Row(
                                verticalAlignment = Alignment.CenterVertically,
                                horizontalArrangement = Arrangement.spacedBy(6.dp)
                            ) {
                                Text(user.name, maxLines = 1, overflow = TextOverflow.Ellipsis, style = MaterialTheme.typography.bodyMedium.copy(textDirection = TextDirection.Content))
                                if (user.isAdmin) {
                                    Icon(
                                        Icons.Default.AdminPanelSettings,
                                        contentDescription = stringResource(R.string.admin_contentDescription_admin),
                                        modifier = Modifier.size(14.dp),
                                        tint = MaterialTheme.colorScheme.primary
                                    )
                                }
                            }
                        },
                        supportingContent = {
                            Text(
                                user.email,
                                style = MaterialTheme.typography.bodySmall.copy(textDirection = TextDirection.Ltr)
                            )
                        },
                        leadingContent = {
                            Icon(
                                Icons.Default.Person, contentDescription = null,
                                tint = if (user.isActive) MaterialTheme.colorScheme.primary
                                else MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        },
                        trailingContent = {
                            Row {
                                IconButton(onClick = {
                                    scope.launch {
                                        try {
                                            adminApi.setUserStatus(
                                                user.id,
                                                mapOf("active" to !user.isActive)
                                            )
                                            users =
                                                users.map { if (it.id == user.id) it.copy(isActive = !user.isActive) else it }
                                        } catch (e: Exception) {
                                            snackbarHostState.showSnackbar(e.message ?: "Failed")
                                        }
                                    }
                                }) {
                                    Icon(
                                        if (user.isActive) Icons.Default.Block else Icons.Default.Check,
                                        contentDescription = if (user.isActive) stringResource(R.string.admin_contentDescription_ban) else stringResource(R.string.admin_contentDescription_unban),
                                        tint = if (user.isActive) MaterialTheme.colorScheme.error
                                        else MaterialTheme.colorScheme.primary
                                    )
                                }
                            }
                        }
                    )
                    HorizontalDivider()
                }
            }
        }
    }
}

// ── Rooms ─────────────────────────────────────────────────────────────────────

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun AdminRoomsContent(
    modifier: Modifier = Modifier,
    adminApi: com.bedrud.app.core.api.AdminApi
) {
    val scope = rememberCoroutineScope()
    val snackbarHostState = remember { SnackbarHostState() }
    var rooms by remember { mutableStateOf<List<AdminRoom>>(emptyList()) }
    var isLoading by remember { mutableStateOf(true) }
    val scrollBehavior = TopAppBarDefaults.exitUntilCollapsedScrollBehavior()

    LaunchedEffect(Unit) {
        isLoading = true
        try {
            rooms = adminApi.listRooms().body()?.rooms ?: emptyList()
        } catch (e: Exception) {
            snackbarHostState.showSnackbar(e.message ?: "Failed to load rooms")
        }
        isLoading = false
    }

    Scaffold(
        modifier = modifier,
        topBar = { LargeTopAppBar(title = { Text(stringResource(R.string.admin_tab_rooms)) }, scrollBehavior = scrollBehavior) },
        snackbarHost = { SnackbarHost(snackbarHostState) }
    ) { padding ->
        LazyColumn(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .nestedScroll(scrollBehavior.nestedScrollConnection),
            contentPadding = PaddingValues(bottom = 16.dp)
        ) {
            if (isLoading) {
                item {
                    Box(
                        Modifier
                            .fillMaxWidth()
                            .height(200.dp),
                        contentAlignment = Alignment.Center
                    ) {
                        CircularProgressIndicator()
                    }
                }
            } else {
                items(rooms, key = { it.id }) { room ->
                    ListItem(
                        headlineContent = {
                            Text(
                                room.name.ifBlank { room.id.take(8) },
                                style = MaterialTheme.typography.bodyLarge.copy(fontFamily = FontFamily.Monospace),
                                maxLines = 1, overflow = TextOverflow.Ellipsis
                            )
                        },
                        supportingContent = {
                            Row(horizontalArrangement = Arrangement.spacedBy(8.dp)) {
                                Text(
                                    if (room.isActive) stringResource(R.string.admin_room_status_live) else stringResource(R.string.admin_room_status_idle),
                                    color = if (room.isActive) MaterialTheme.colorScheme.primary
                                    else MaterialTheme.colorScheme.onSurfaceVariant,
                                    style = MaterialTheme.typography.bodySmall
                                )
                                Text(
                                    stringResource(R.string.admin_room_maxParticipants, room.maxParticipants),
                                    style = MaterialTheme.typography.bodySmall
                                )
                            }
                        },
                        leadingContent = {
                            Icon(
                                Icons.Default.MeetingRoom,
                                contentDescription = null
                            )
                        },
                        trailingContent = {
                            IconButton(onClick = {
                                scope.launch {
                                    try {
                                        adminApi.deleteRoom(room.id)
                                        rooms = rooms.filter { it.id != room.id }
                                    } catch (e: Exception) {
                                        snackbarHostState.showSnackbar(
                                            e.message ?: "Failed to delete"
                                        )
                                    }
                                }
                            }) {
                                Icon(
                                            Icons.Default.Delete, contentDescription = stringResource(R.string.common_button_delete),
                                    tint = MaterialTheme.colorScheme.error
                                )
                            }
                        }
                    )
                    HorizontalDivider()
                }
            }
        }
    }
}

// ── Settings ──────────────────────────────────────────────────────────────────

@OptIn(ExperimentalMaterial3Api::class)
@Composable
private fun AdminSettingsContent(
    modifier: Modifier = Modifier,
    adminApi: com.bedrud.app.core.api.AdminApi
) {
    val scope = rememberCoroutineScope()
    val snackbarHostState = remember { SnackbarHostState() }
    val clipboard = LocalClipboardManager.current
    var settings by remember { mutableStateOf<AdminSettings?>(null) }
    var tokens by remember { mutableStateOf<List<InviteToken>>(emptyList()) }
    var isLoading by remember { mutableStateOf(true) }
    var tokenEmail by remember { mutableStateOf("") }
    var newToken by remember { mutableStateOf<InviteToken?>(null) }
    val scrollBehavior = TopAppBarDefaults.exitUntilCollapsedScrollBehavior()

    LaunchedEffect(Unit) {
        isLoading = true
        try {
            settings = adminApi.getSettings().body()
            tokens = adminApi.listInviteTokens().body()?.tokens ?: emptyList()
        } catch (e: Exception) {
            snackbarHostState.showSnackbar(e.message ?: "Failed to load settings")
        }
        isLoading = false
    }

    Scaffold(
        modifier = modifier,
        topBar = {
            LargeTopAppBar(
                title = { Text(stringResource(R.string.admin_title_systemSettings)) },
                scrollBehavior = scrollBehavior
            )
        },
        snackbarHost = { SnackbarHost(snackbarHostState) }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .nestedScroll(scrollBehavior.nestedScrollConnection)
                .verticalScroll(rememberScrollState())
                .padding(16.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            settings?.let { s ->
                // Registration settings
                ElevatedCard(shape = RoundedCornerShape(16.dp)) {
                    Column {
                        Text(
                            stringResource(R.string.admin_section_registration), style = MaterialTheme.typography.labelLarge,
                            color = MaterialTheme.colorScheme.primary,
                            modifier = Modifier.padding(
                                start = 16.dp,
                                top = 16.dp,
                                end = 16.dp,
                                bottom = 8.dp
                            )
                        )
                        ListItem(
                            headlineContent = { Text(stringResource(R.string.admin_setting_allowRegistrations)) },
                            trailingContent = {
                                Switch(
                                    checked = s.registrationEnabled,
                                    onCheckedChange = { newVal ->
                                        scope.launch {
                                            val updated = s.copy(registrationEnabled = newVal)
                                            try {
                                                adminApi.updateSettings(updated); settings = updated
                                            } catch (e: Exception) {
                                                snackbarHostState.showSnackbar(
                                                    e.message ?: "Failed"
                                                )
                                            }
                                        }
                                    })
                            },
                            colors = ListItemDefaults.colors(containerColor = Color.Transparent)
                        )
                        HorizontalDivider(modifier = Modifier.padding(horizontal = 16.dp))
                        ListItem(
                            headlineContent = { Text(stringResource(R.string.admin_setting_requireInviteToken)) },
                            trailingContent = {
                                Switch(
                                    checked = s.tokenRegistrationOnly,
                                    onCheckedChange = { newVal ->
                                        scope.launch {
                                            val updated = s.copy(tokenRegistrationOnly = newVal)
                                            try {
                                                adminApi.updateSettings(updated); settings = updated
                                            } catch (e: Exception) {
                                                snackbarHostState.showSnackbar(
                                                    e.message ?: "Failed"
                                                )
                                            }
                                        }
                                    })
                            },
                            colors = ListItemDefaults.colors(containerColor = Color.Transparent)
                        )
                    }
                }
            }

            // Invite tokens
            ElevatedCard(shape = RoundedCornerShape(16.dp)) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text(
                        text = stringResource(R.string.admin_section_inviteTokens),
                        style = MaterialTheme.typography.labelLarge,
                        color = MaterialTheme.colorScheme.primary
                    )
                    Spacer(modifier = Modifier.height(8.dp))

                    // New token generated highlight
                    newToken?.let { tok ->
                        ElevatedCard(
                            shape = RoundedCornerShape(8.dp),
                            colors = CardDefaults.elevatedCardColors(containerColor = MaterialTheme.colorScheme.primaryContainer)
                        ) {
                            Row(
                                modifier = Modifier
                                    .fillMaxWidth()
                                    .padding(12.dp),
                                verticalAlignment = Alignment.CenterVertically
                            ) {
                                Text(
                                    tok.token, modifier = Modifier.weight(1f),
                                    style = MaterialTheme.typography.bodySmall.copy(fontFamily = FontFamily.Monospace),
                                    maxLines = 1, overflow = TextOverflow.Ellipsis
                                )
                                IconButton(onClick = { clipboard.setText(AnnotatedString(tok.token)) }) {
                                    Icon(Icons.Default.ContentCopy, contentDescription = stringResource(R.string.common_action_copy))
                                }
                            }
                        }
                        Spacer(modifier = Modifier.height(8.dp))
                    }

                    Row(verticalAlignment = Alignment.CenterVertically) {
                        OutlinedTextField(
                            value = tokenEmail, onValueChange = { tokenEmail = it },
                            placeholder = { Text(stringResource(R.string.admin_placeholder_email)) },
                            singleLine = true, shape = RoundedCornerShape(12.dp),
                            modifier = Modifier.weight(1f),
                            textStyle = MaterialTheme.typography.bodyMedium.copy(textDirection = TextDirection.Ltr)
                        )
                        Spacer(modifier = Modifier.width(8.dp))
                        Button(onClick = {
                            scope.launch {
                                try {
                                    val body = CreateInviteTokenRequest(
                                        email = tokenEmail.takeIf { it.isNotBlank() },
                                        expiresInHours = 168
                                    )
                                    val created = adminApi.createInviteToken(body).body()
                                    if (created != null) {
                                        tokens = tokens + created
                                        newToken = created
                                        tokenEmail = ""
                                    }
                                } catch (e: Exception) {
                                    snackbarHostState.showSnackbar(e.message ?: "Failed")
                                }
                            }
                        }) { Text(stringResource(R.string.common_button_generate)) }
                    }

                    Spacer(modifier = Modifier.height(8.dp))

                    tokens.forEach { tok ->
                        ListItem(
                            headlineContent = {
                                Text(
                                    tok.token.take(16) + "…",
                                    style = MaterialTheme.typography.bodySmall.copy(fontFamily = FontFamily.Monospace)
                                )
                            },
                            supportingContent = {
                                Text(
                                    if (tok.used) stringResource(R.string.admin_token_status_used) else tok.email ?: stringResource(R.string.admin_token_status_noEmail),
                                    style = MaterialTheme.typography.bodySmall.copy(textDirection = TextDirection.Ltr),
                                    color = if (tok.used) MaterialTheme.colorScheme.error
                                    else MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            },
                            leadingContent = {
                                Icon(
                                    Icons.Default.Token,
                                    contentDescription = null
                                )
                            },
                            trailingContent = {
                                Row {
                                    IconButton(onClick = { clipboard.setText(AnnotatedString(tok.token)) }) {
                                        Icon(
                                            Icons.Default.ContentCopy, contentDescription = stringResource(R.string.common_action_copy),
                                            modifier = Modifier.size(18.dp)
                                        )
                                    }
                                    IconButton(onClick = {
                                        scope.launch {
                                            try {
                                                adminApi.deleteInviteToken(tok.id)
                                                tokens = tokens.filter { it.id != tok.id }
                                            } catch (e: Exception) {
                                                snackbarHostState.showSnackbar(
                                                    e.message ?: "Failed"
                                                )
                                            }
                                        }
                                    }) {
                                        Icon(
                                            Icons.Default.Delete, contentDescription = stringResource(R.string.common_button_delete),
                                            tint = MaterialTheme.colorScheme.error,
                                            modifier = Modifier.size(18.dp)
                                        )
                                    }
                                }
                            },
                            colors = ListItemDefaults.colors(containerColor = Color.Transparent)
                        )
                        HorizontalDivider()
                    }
                }
            }
        }
    }
}
