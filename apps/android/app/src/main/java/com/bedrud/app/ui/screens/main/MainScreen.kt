package com.bedrud.app.ui.screens.main

import androidx.compose.foundation.layout.PaddingValues
import androidx.compose.foundation.layout.padding
import com.bedrud.app.ui.components.BedrudMainScaffoldContentInsets
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AdminPanelSettings
import androidx.compose.material.icons.filled.Person
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.VideoCall
import androidx.compose.material.icons.outlined.AdminPanelSettings
import androidx.compose.material.icons.outlined.Person
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material.icons.outlined.VideoCall
import androidx.compose.material3.Scaffold
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.res.stringResource
import com.bedrud.app.R
import com.bedrud.app.core.instance.InstanceManager
import com.bedrud.app.core.recent.RecentRoomsStore
import com.bedrud.app.ui.components.BottomNavTab
import com.bedrud.app.ui.components.BedrudBottomNavigationBar
import com.bedrud.app.ui.screens.admin.AdminScreen
import com.bedrud.app.ui.screens.dashboard.DashboardContent
import com.bedrud.app.ui.screens.profile.ProfileContent
import com.bedrud.app.ui.screens.settings.SettingsContent
import com.bedrud.app.ui.screens.settings.SettingsStore
import org.koin.compose.koinInject

@Composable
fun MainScreen(
    onJoinRoom: (String) -> Unit,
    onLogout: () -> Unit,
    onNavigateToAddInstance: () -> Unit,
    instanceManager: InstanceManager = koinInject(),
    settingsStore: SettingsStore = koinInject(),
    recentRoomsStore: RecentRoomsStore = koinInject(),
) {
    fun recordAndJoin(roomName: String, instanceId: String, instanceName: String) {
        recentRoomsStore.add(roomName, instanceId, instanceName)
        onJoinRoom(roomName)
    }

    fun joinFromDashboard(roomName: String) {
        val instance = instanceManager.store.activeInstance
        if (instance != null) {
            recordAndJoin(roomName, instance.id, instance.displayName)
        } else {
            onJoinRoom(roomName)
        }
    }
    val authManager by instanceManager.authManager.collectAsState()
    val currentUser by remember(authManager) {
        authManager?.currentUser ?: kotlinx.coroutines.flow.MutableStateFlow(null)
    }.collectAsState()
    val isAdmin = currentUser?.isAdmin == true

    val tabs = buildList {
        add(
            BottomNavTab(
                label = stringResource(R.string.main_tab_rooms),
                icon = Icons.Outlined.VideoCall,
                selectedIcon = Icons.Filled.VideoCall
            )
        )
        add(
            BottomNavTab(
                label = stringResource(R.string.main_tab_profile),
                icon = Icons.Outlined.Person,
                selectedIcon = Icons.Filled.Person
            )
        )
        add(
            BottomNavTab(
                label = stringResource(R.string.main_tab_settings),
                icon = Icons.Outlined.Settings,
                selectedIcon = Icons.Filled.Settings
            )
        )
        if (isAdmin) {
            add(
                BottomNavTab(
                    label = stringResource(R.string.main_tab_admin),
                    icon = Icons.Outlined.AdminPanelSettings,
                    selectedIcon = Icons.Filled.AdminPanelSettings
                )
            )
        }
    }

    var selectedTab by remember { mutableIntStateOf(settingsStore.getLastTab()) }
    if (selectedTab >= tabs.size) {
        selectedTab = 0
    }

    LaunchedEffect(selectedTab) {
        settingsStore.setLastTab(selectedTab)
    }

    Scaffold(
        contentWindowInsets = BedrudMainScaffoldContentInsets,
        bottomBar = {
            BedrudBottomNavigationBar(
                tabs = tabs,
                selectedIndex = selectedTab,
                onTabSelected = { selectedTab = it }
            )
        }
    ) { padding ->
        // Only reserve space for the bottom nav — each tab's top bar handles the status bar once.
        val contentPadding = PaddingValues(bottom = padding.calculateBottomPadding())
        when (selectedTab) {
            0 -> DashboardContent(
                modifier = Modifier.padding(contentPadding),
                onJoinRoom = ::joinFromDashboard,
                onJoinRecent = { recent ->
                    if (recent.instanceId != instanceManager.store.activeInstanceId.value) {
                        instanceManager.switchTo(recent.instanceId)
                    }
                    recordAndJoin(recent.roomName, recent.instanceId, recent.instanceName)
                },
                instanceManager = instanceManager,
            )
            1 -> ProfileContent(
                modifier = Modifier.padding(contentPadding),
                onLogout = onLogout,
                onNavigateToAddInstance = onNavigateToAddInstance,
                instanceManager = instanceManager
            )
            2 -> SettingsContent(
                modifier = Modifier.padding(contentPadding)
            )
            3 -> if (isAdmin) {
                AdminScreen(modifier = Modifier.padding(contentPadding))
            }
        }
    }
}