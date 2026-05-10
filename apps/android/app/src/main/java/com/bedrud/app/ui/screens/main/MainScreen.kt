package com.bedrud.app.ui.screens.main

import androidx.compose.animation.core.Animatable
import androidx.compose.animation.core.spring
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AdminPanelSettings
import androidx.compose.material.icons.filled.Person
import androidx.compose.material.icons.filled.Settings
import androidx.compose.material.icons.filled.VideoCall
import androidx.compose.material.icons.outlined.AdminPanelSettings
import androidx.compose.material.icons.outlined.Person
import androidx.compose.material.icons.outlined.Settings
import androidx.compose.material.icons.outlined.VideoCall
import androidx.compose.material3.Icon
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.NavigationBar
import androidx.compose.material3.NavigationBarItem
import androidx.compose.material3.Scaffold
import androidx.compose.material3.Text
import androidx.compose.runtime.Composable
import androidx.compose.runtime.LaunchedEffect
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.remember
import androidx.compose.runtime.mutableIntStateOf
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.saveable.rememberSaveable
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.TransformOrigin
import androidx.compose.ui.graphics.graphicsLayer
import androidx.compose.ui.graphics.vector.ImageVector
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.unit.dp
import kotlinx.coroutines.launch
import com.bedrud.app.R
import com.bedrud.app.core.instance.InstanceManager
import com.bedrud.app.ui.screens.admin.AdminScreen
import com.bedrud.app.ui.screens.dashboard.DashboardContent
import com.bedrud.app.ui.screens.profile.ProfileContent
import com.bedrud.app.ui.screens.settings.SettingsContent
import com.bedrud.app.ui.screens.settings.SettingsStore
import org.koin.compose.koinInject

private data class TabItem(
    val label: String,
    val selectedIcon: ImageVector,
    val unselectedIcon: ImageVector
)

@Composable
fun MainScreen(
    onJoinRoom: (String) -> Unit,
    onLogout: () -> Unit,
    onNavigateToAddInstance: () -> Unit,
    instanceManager: InstanceManager = koinInject(),
    settingsStore: SettingsStore = koinInject()
) {
    val authManager by instanceManager.authManager.collectAsState()
    val currentUser by remember(authManager) {
        authManager?.currentUser ?: kotlinx.coroutines.flow.MutableStateFlow(null)
    }.collectAsState()
    val isAdmin = currentUser?.isAdmin == true

    val tabs = buildList {
        add(TabItem(stringResource(R.string.main_tab_rooms), Icons.Filled.VideoCall, Icons.Outlined.VideoCall))
        add(TabItem(stringResource(R.string.main_tab_profile), Icons.Filled.Person, Icons.Outlined.Person))
        add(TabItem(stringResource(R.string.main_tab_settings), Icons.Filled.Settings, Icons.Outlined.Settings))
        if (isAdmin) add(TabItem(stringResource(R.string.main_tab_admin), Icons.Filled.AdminPanelSettings, Icons.Outlined.AdminPanelSettings))
    }

    var selectedTab by remember { mutableIntStateOf(settingsStore.getLastTab()) }
    if (selectedTab >= tabs.size) { selectedTab = 0 }

    val navScope = rememberCoroutineScope()

    LaunchedEffect(selectedTab) {
        settingsStore.setLastTab(selectedTab)
    }

    Scaffold(
        bottomBar = {
            NavigationBar {
                tabs.forEachIndexed { index, tab ->
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
                                    if (isSelected) tab.selectedIcon else tab.unselectedIcon,
                                    contentDescription = tab.label,
                                    tint = if (isSelected) MaterialTheme.colorScheme.primary
                                    else MaterialTheme.colorScheme.onSurfaceVariant,
                                    modifier = Modifier.size(24.dp)
                                )
                            }
                        },
                        label = { Text(tab.label) }
                    )
                }
            }
        }
    ) { padding ->
        when (selectedTab) {
            0 -> DashboardContent(
                modifier = Modifier.padding(padding),
                onJoinRoom = onJoinRoom,
                instanceManager = instanceManager
            )
            1 -> ProfileContent(
                modifier = Modifier.padding(padding),
                onLogout = onLogout,
                onNavigateToAddInstance = onNavigateToAddInstance,
                instanceManager = instanceManager
            )
            2 -> SettingsContent(
                modifier = Modifier.padding(padding)
            )
            3 -> if (isAdmin) AdminScreen(modifier = Modifier.padding(padding))
        }
    }
}
