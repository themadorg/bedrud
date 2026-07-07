package com.bedrud.app.ui.screens.profile

import androidx.compose.foundation.background
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Box
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.CircleShape
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.Logout
import androidx.compose.material.icons.filled.SwapHoriz
import androidx.compose.material3.ButtonDefaults

import com.bedrud.app.ui.components.BedrudOutlinedCard
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.FilledTonalButton
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import com.bedrud.app.ui.components.BedrudCompactTopBar
import androidx.compose.material3.ListItem
import androidx.compose.material3.ListItemDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton

import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.setValue
import androidx.compose.ui.Alignment
import androidx.compose.ui.Modifier
import androidx.compose.ui.draw.clip
import androidx.compose.ui.graphics.Color

import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.text.style.TextDirection
import androidx.compose.ui.text.style.TextOverflow
import androidx.compose.ui.layout.ContentScale
import androidx.compose.ui.unit.dp
import coil.compose.AsyncImage
import com.bedrud.app.R
import com.bedrud.app.core.instance.InstanceManager
import com.bedrud.app.ui.screens.instance.InstanceSwitcherSheet
import org.koin.compose.koinInject

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun ProfileContent(
    modifier: Modifier = Modifier,
    onLogout: () -> Unit,
    onNavigateToAddInstance: () -> Unit,
    instanceManager: InstanceManager = koinInject()
) {
    val authManager = instanceManager.authManager.collectAsState().value
    val user by (authManager?.currentUser ?: kotlinx.coroutines.flow.MutableStateFlow(null)).collectAsState()
    val activeInstance = instanceManager.store.activeInstance
    var showInstanceSwitcher by remember { mutableStateOf(false) }

    if (showInstanceSwitcher) {
        InstanceSwitcherSheet(
            instanceManager = instanceManager,
            onDismiss = { showInstanceSwitcher = false },
            onAddInstance = {
                showInstanceSwitcher = false
                onNavigateToAddInstance()
            }
        )
    }

    Column(modifier = modifier.fillMaxSize()) {
        BedrudCompactTopBar(title = stringResource(R.string.profile_title))

        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 16.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            Spacer(modifier = Modifier.height(0.dp))

            // User section
            BedrudOutlinedCard(shape = RoundedCornerShape(16.dp)) {
                Row(
                    modifier = Modifier
                        .fillMaxWidth()
                        .padding(16.dp),
                    verticalAlignment = Alignment.CenterVertically
                ) {
                    if (!user?.avatarUrl.isNullOrBlank()) {
                        AsyncImage(
                            model = user?.avatarUrl,
                            contentDescription = stringResource(R.string.profile_contentDescription_profilePicture),
                            modifier = Modifier
                                .size(56.dp)
                                .clip(CircleShape),
                            contentScale = ContentScale.Crop
                        )
                    } else {
                        Box(
                            modifier = Modifier
                                .size(56.dp)
                                .clip(CircleShape)
                                .background(MaterialTheme.colorScheme.primary),
                            contentAlignment = Alignment.Center
                        ) {
                            Text(
                                (user?.name?.take(1) ?: "?").uppercase(),
                                style = MaterialTheme.typography.headlineSmall,
                                color = MaterialTheme.colorScheme.onPrimary
                            )
                        }
                    }
                    Spacer(modifier = Modifier.width(16.dp))
                    Column(modifier = Modifier.weight(1f)) {
                        Row(
                            verticalAlignment = Alignment.CenterVertically,
                            horizontalArrangement = Arrangement.spacedBy(8.dp)
                        ) {
                            Text(
                                user?.name ?: stringResource(R.string.profile_fallback_name),
                                style = MaterialTheme.typography.titleLarge.copy(textDirection = TextDirection.Content)
                            )
                            if (user?.isAdmin == true) {
                                Text(
                                    stringResource(R.string.profile_badge_admin),
                                    style = MaterialTheme.typography.labelSmall.copy(
                                        fontWeight = FontWeight.Bold
                                    ),
                                    color = MaterialTheme.colorScheme.onPrimary,
                                    modifier = Modifier
                                        .background(
                                            MaterialTheme.colorScheme.primary,
                                            RoundedCornerShape(4.dp)
                                        )
                                        .padding(horizontal = 6.dp, vertical = 2.dp)
                                )
                            }
                        }
                        Text(
                            user?.email ?: "",
                            style = MaterialTheme.typography.bodyMedium.copy(textDirection = TextDirection.Ltr),
                            color = MaterialTheme.colorScheme.onSurfaceVariant
                        )
                    }
                }
            }

            // Server section
            BedrudOutlinedCard(shape = RoundedCornerShape(16.dp)) {
                Column {
                    Text(
                        stringResource(R.string.profile_section_server),
                        style = MaterialTheme.typography.labelLarge,
                        color = MaterialTheme.colorScheme.primary,
                        modifier = Modifier.padding(start = 16.dp, top = 16.dp, end = 16.dp, bottom = 8.dp)
                    )

                    if (activeInstance != null) {
                        ListItem(
                            headlineContent = {
                                Text(activeInstance.displayName, style = MaterialTheme.typography.bodyLarge.copy(textDirection = TextDirection.Content))
                            },
                            supportingContent = {
                                Text(
                                    activeInstance.serverURL,
                                    style = MaterialTheme.typography.bodySmall.copy(textDirection = TextDirection.Ltr),
                                    color = MaterialTheme.colorScheme.onSurfaceVariant,
                                    maxLines = 1,
                                    overflow = TextOverflow.Ellipsis
                                )
                            },
                            leadingContent = {
                                Box(
                                    modifier = Modifier
                                        .size(36.dp)
                                        .clip(CircleShape)
                                        .background(parseProfileColor(activeInstance.iconColorHex)),
                                    contentAlignment = Alignment.Center
                                ) {
                                    Text(
                                        activeInstance.displayName.take(1).uppercase(),
                                        style = MaterialTheme.typography.labelMedium,
                                        color = Color.White
                                    )
                                }
                            },
                            trailingContent = {
                                FilledTonalButton(onClick = { showInstanceSwitcher = true }) {
                                    Icon(
                                        Icons.Default.SwapHoriz,
                                        contentDescription = null,
                                        modifier = Modifier.size(18.dp)
                                    )
                                    Spacer(modifier = Modifier.width(4.dp))
                                    Text(stringResource(R.string.profile_button_switch))
                                }
                            },
                            colors = ListItemDefaults.colors(containerColor = Color.Transparent)
                        )
                    }
                }
            }

            // Account section
            BedrudOutlinedCard(shape = RoundedCornerShape(16.dp)) {
                Column {
                    Text(
                        stringResource(R.string.profile_section_account),
                        style = MaterialTheme.typography.labelLarge,
                        color = MaterialTheme.colorScheme.primary,
                        modifier = Modifier.padding(start = 16.dp, top = 16.dp, end = 16.dp, bottom = 8.dp)
                    )

                    if (user != null) {
                        ListItem(
                            headlineContent = { Text(stringResource(R.string.profile_label_userId)) },
                            trailingContent = {
                                Text(
                                    user!!.id.take(8) + "...",
                                    style = MaterialTheme.typography.bodyMedium,
                                    color = MaterialTheme.colorScheme.onSurfaceVariant
                                )
                            },
                            colors = ListItemDefaults.colors(containerColor = Color.Transparent)
                        )

                        if (user?.provider != null) {
                            HorizontalDivider(
                                modifier = Modifier.padding(horizontal = 16.dp),
                                color = MaterialTheme.colorScheme.outlineVariant
                            )
                            ListItem(
                                headlineContent = { Text(stringResource(R.string.profile_label_provider)) },
                                trailingContent = {
                                    Text(
                                        user!!.provider!!.replaceFirstChar { it.uppercase() },
                                        style = MaterialTheme.typography.bodyMedium,
                                        color = MaterialTheme.colorScheme.onSurfaceVariant
                                    )
                                },
                                colors = ListItemDefaults.colors(containerColor = Color.Transparent)
                            )
                        }
                    }
                }
            }

            // Sign Out
            TextButton(
                onClick = onLogout,
                colors = ButtonDefaults.textButtonColors(
                    contentColor = MaterialTheme.colorScheme.error
                ),
                modifier = Modifier.fillMaxWidth()
            ) {
                Icon(
                    Icons.AutoMirrored.Filled.Logout,
                    contentDescription = null,
                    modifier = Modifier.size(18.dp)
                )
                Spacer(modifier = Modifier.width(8.dp))
                Text(stringResource(R.string.profile_button_signOut), style = MaterialTheme.typography.labelLarge)
            }

            Spacer(modifier = Modifier.height(16.dp))
        }
    }
}

private fun parseProfileColor(hex: String): Color {
    val cleaned = hex.trimStart('#')
    if (cleaned.length != 6) return Color(0xFF3B82F6)
    return try {
        Color(android.graphics.Color.parseColor("#$cleaned"))
    } catch (_: Exception) {
        Color(0xFF3B82F6)
    }
}
