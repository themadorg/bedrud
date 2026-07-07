package com.bedrud.app.ui.screens.settings

import android.app.Activity
import androidx.compose.foundation.layout.Arrangement
import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Row
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.shape.RoundedCornerShape
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.filled.AdminPanelSettings
import androidx.compose.material3.Button
import androidx.compose.material3.CardDefaults
import androidx.compose.material3.DropdownMenuItem
import com.bedrud.app.ui.components.BedrudOutlinedCard
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.ExposedDropdownMenuBox
import androidx.compose.material3.ExposedDropdownMenuDefaults
import androidx.compose.material3.HorizontalDivider
import androidx.compose.material3.Icon
import com.bedrud.app.ui.components.BedrudCompactTopBar
import androidx.compose.material3.ListItem
import androidx.compose.material3.ListItemDefaults
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.SegmentedButton
import androidx.compose.material3.SegmentedButtonDefaults
import androidx.compose.material3.SingleChoiceSegmentedButtonRow
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Switch
import androidx.compose.material3.Text

import androidx.compose.runtime.Composable
import androidx.compose.runtime.collectAsState
import androidx.compose.runtime.getValue
import androidx.compose.runtime.mutableStateOf
import androidx.compose.runtime.remember
import androidx.compose.runtime.rememberCoroutineScope
import androidx.compose.runtime.setValue
import androidx.compose.ui.Modifier
import androidx.compose.ui.graphics.Color

import androidx.compose.ui.platform.LocalContext
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.tooling.preview.Preview
import androidx.compose.ui.unit.dp
import com.bedrud.app.R
import com.bedrud.app.core.instance.InstanceManager
import com.bedrud.app.models.ChangePasswordRequest
import kotlinx.coroutines.launch
import org.koin.compose.koinInject

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun SettingsContent(
    modifier: Modifier = Modifier,
    settingsStore: SettingsStore = koinInject(),
    instanceManager: InstanceManager = koinInject()
) {
    val appearance by settingsStore.appearance.collectAsState()
    val notificationsEnabled by settingsStore.notificationsEnabled.collectAsState()
    val language by settingsStore.language.collectAsState()
    val context = LocalContext.current
    val scope = rememberCoroutineScope()
    val snackbarHostState = remember { SnackbarHostState() }
    val authApi = instanceManager.authApi.collectAsState().value
    val authManager = instanceManager.authManager.collectAsState().value
    val currentUser by (authManager?.currentUser ?: kotlinx.coroutines.flow.MutableStateFlow(null)).collectAsState()

    var currentPassword by remember { mutableStateOf("") }
    var newPassword by remember { mutableStateOf("") }
    var confirmPassword by remember { mutableStateOf("") }

    Column(modifier = modifier.fillMaxSize()) {
        BedrudCompactTopBar(title = stringResource(R.string.settings_title))

        SnackbarHost(snackbarHostState)

        Column(
            modifier = Modifier
                .fillMaxSize()
                .verticalScroll(rememberScrollState())
                .padding(horizontal = 16.dp),
            verticalArrangement = Arrangement.spacedBy(16.dp)
        ) {
            Spacer(modifier = Modifier.height(0.dp))

            // Appearance
            BedrudOutlinedCard(shape = RoundedCornerShape(16.dp)) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text(
                        stringResource(R.string.settings_section_appearance),
                        style = MaterialTheme.typography.labelLarge,
                        color = MaterialTheme.colorScheme.primary
                    )
                    Spacer(modifier = Modifier.height(12.dp))
                    Text(
                        stringResource(R.string.settings_label_theme),
                        style = MaterialTheme.typography.bodyLarge
                    )
                    Spacer(modifier = Modifier.height(8.dp))
                    SingleChoiceSegmentedButtonRow(modifier = Modifier.fillMaxWidth()) {
                        AppAppearance.entries.forEachIndexed { index, option ->
                            SegmentedButton(
                                selected = appearance == option,
                                onClick = { settingsStore.setAppearance(option) },
                                shape = SegmentedButtonDefaults.itemShape(
                                    index = index,
                                    count = AppAppearance.entries.size
                                )
                            ) {
                                Text(stringResource(option.stringResId))
                            }
                        }
                    }
                    Spacer(modifier = Modifier.height(12.dp))
                    Text(
                        stringResource(R.string.settings_label_language),
                        style = MaterialTheme.typography.bodyLarge
                    )
                    Spacer(modifier = Modifier.height(8.dp))
                    var languageExpanded by remember { mutableStateOf(false) }
                    ExposedDropdownMenuBox(
                        expanded = languageExpanded,
                        onExpandedChange = { languageExpanded = it }
                    ) {
                        OutlinedTextField(
                            value = language.label,
                            onValueChange = {},
                            readOnly = true,
                            trailingIcon = { ExposedDropdownMenuDefaults.TrailingIcon(expanded = languageExpanded) },
                            modifier = Modifier.menuAnchor().fillMaxWidth(),
                            shape = RoundedCornerShape(12.dp)
                        )
                        ExposedDropdownMenu(
                            expanded = languageExpanded,
                            onDismissRequest = { languageExpanded = false }
                        ) {
                            AppLanguage.entries.forEach { lang ->
                                DropdownMenuItem(
                                    text = { Text(lang.label) },
                                    onClick = {
                                        settingsStore.setLanguage(lang)
                                        languageExpanded = false
                                        (context as? Activity)?.recreate()
                                    }
                                )
                            }
                        }
                    }
                }
            }

            // Notifications
            BedrudOutlinedCard(shape = RoundedCornerShape(16.dp)) {
                Column {
                    Text(
                        stringResource(R.string.settings_section_notifications),
                        style = MaterialTheme.typography.labelLarge,
                        color = MaterialTheme.colorScheme.primary,
                        modifier = Modifier.padding(start = 16.dp, top = 16.dp, end = 16.dp, bottom = 8.dp)
                    )
                    ListItem(
                        headlineContent = { Text(stringResource(R.string.settings_label_enableNotifications)) },
                        trailingContent = {
                            Switch(
                                checked = notificationsEnabled,
                                onCheckedChange = { settingsStore.setNotificationsEnabled(it) }
                            )
                        },
                        colors = ListItemDefaults.colors(containerColor = Color.Transparent)
                    )
                }
            }

            // Account Info
            if (currentUser != null) {
                BedrudOutlinedCard(shape = RoundedCornerShape(16.dp)) {
                    Column {
                        Row(
                            modifier = Modifier.padding(start = 16.dp, top = 16.dp, end = 16.dp, bottom = 8.dp),
                            horizontalArrangement = Arrangement.spacedBy(8.dp)
                        ) {
                            Text(stringResource(R.string.settings_section_account), style = MaterialTheme.typography.labelLarge,
                                color = MaterialTheme.colorScheme.primary)
                            if (currentUser?.isAdmin == true) {
                                Icon(Icons.Default.AdminPanelSettings, contentDescription = null,
                                    tint = MaterialTheme.colorScheme.primary)
                            }
                        }
                        ListItem(
                            headlineContent = { Text(stringResource(R.string.settings_label_accountId)) },
                            trailingContent = {
                                Text(currentUser?.id?.take(8) ?: "", style = MaterialTheme.typography.bodySmall.copy(fontFamily = FontFamily.Monospace),
                                    color = MaterialTheme.colorScheme.onSurfaceVariant)
                            },
                            colors = ListItemDefaults.colors(containerColor = Color.Transparent)
                        )
                        HorizontalDivider(modifier = Modifier.padding(horizontal = 16.dp),
                            color = MaterialTheme.colorScheme.outlineVariant)
                        ListItem(
                            headlineContent = { Text(stringResource(R.string.settings_label_signInMethod)) },
                            trailingContent = {
                                Text(currentUser?.provider?.replaceFirstChar { it.uppercase() } ?: stringResource(R.string.settings_provider_email),
                                    color = MaterialTheme.colorScheme.onSurfaceVariant)
                            },
                            colors = ListItemDefaults.colors(containerColor = Color.Transparent)
                        )
                        HorizontalDivider(modifier = Modifier.padding(horizontal = 16.dp),
                            color = MaterialTheme.colorScheme.outlineVariant)
                        ListItem(
                            headlineContent = { Text(stringResource(R.string.settings_label_role)) },
                            trailingContent = {
                                Text(if (currentUser?.isAdmin == true) stringResource(R.string.settings_role_admin) else stringResource(R.string.settings_role_user),
                                    color = MaterialTheme.colorScheme.onSurfaceVariant)
                            },
                            colors = ListItemDefaults.colors(containerColor = Color.Transparent)
                        )
                    }
                }
            }

            // Change Password
            val isLocalAccount = currentUser?.provider.let { it == null || it == "local" || it == "passkey" }
            BedrudOutlinedCard(shape = RoundedCornerShape(16.dp)) {
                Column(modifier = Modifier.padding(16.dp)) {
                    Text(stringResource(R.string.settings_section_security), style = MaterialTheme.typography.labelLarge,
                        color = MaterialTheme.colorScheme.primary)
                    Spacer(modifier = Modifier.height(12.dp))

                    if (!isLocalAccount) {
                        Text(stringResource(R.string.settings_password_unavailable, currentUser?.provider?.replaceFirstChar { it.uppercase() } ?: ""),
                            style = MaterialTheme.typography.bodyMedium,
                            color = MaterialTheme.colorScheme.onSurfaceVariant)
                    } else {
                        OutlinedTextField(
                            value = currentPassword,
                            onValueChange = { currentPassword = it },
                            label = { Text(stringResource(R.string.settings_label_currentPassword)) },
                            visualTransformation = PasswordVisualTransformation(),
                            singleLine = true,
                            shape = RoundedCornerShape(12.dp),
                            modifier = Modifier.fillMaxWidth()
                        )
                        Spacer(modifier = Modifier.height(8.dp))
                        OutlinedTextField(
                            value = newPassword,
                            onValueChange = { newPassword = it },
                            label = { Text(stringResource(R.string.settings_label_newPassword)) },
                            visualTransformation = PasswordVisualTransformation(),
                            singleLine = true,
                            shape = RoundedCornerShape(12.dp),
                            modifier = Modifier.fillMaxWidth()
                        )
                        Spacer(modifier = Modifier.height(8.dp))
                        OutlinedTextField(
                            value = confirmPassword,
                            onValueChange = { confirmPassword = it },
                            label = { Text(stringResource(R.string.settings_label_confirmNewPassword)) },
                            visualTransformation = PasswordVisualTransformation(),
                            singleLine = true,
                            shape = RoundedCornerShape(12.dp),
                            modifier = Modifier.fillMaxWidth()
                        )
                        Spacer(modifier = Modifier.height(12.dp))
                        Button(
                            onClick = {
                                when {
                                    newPassword.length < 8 -> scope.launch {
                                        snackbarHostState.showSnackbar("Password must be at least 8 characters")
                                    }
                                    newPassword != confirmPassword -> scope.launch {
                                        snackbarHostState.showSnackbar("Passwords do not match")
                                    }
                                    else -> scope.launch {
                                        try {
                                            val response = authApi?.changePassword(
                                                ChangePasswordRequest(currentPassword, newPassword)
                                            )
                                            if (response?.isSuccessful == true) {
                                                currentPassword = ""
                                                newPassword = ""
                                                confirmPassword = ""
                                                snackbarHostState.showSnackbar("Password changed successfully")
                                            } else {
                                                snackbarHostState.showSnackbar("Failed to change password")
                                            }
                                        } catch (e: Exception) {
                                            snackbarHostState.showSnackbar(e.message ?: "Failed to change password")
                                        }
                                    }
                                }
                            },
                            enabled = currentPassword.isNotBlank() && newPassword.isNotBlank() && confirmPassword.isNotBlank(),
                            modifier = Modifier.fillMaxWidth()
                        ) { Text(stringResource(R.string.settings_button_changePassword)) }
                    }
                }
            }

            // About
            BedrudOutlinedCard(shape = RoundedCornerShape(16.dp)) {
                Column {
                    Text(
                        stringResource(R.string.settings_section_about),
                        style = MaterialTheme.typography.labelLarge,
                        color = MaterialTheme.colorScheme.primary,
                        modifier = Modifier.padding(start = 16.dp, top = 16.dp, end = 16.dp, bottom = 8.dp)
                    )

                    val packageInfo = try {
                        context.packageManager.getPackageInfo(context.packageName, 0)
                    } catch (_: Exception) {
                        null
                    }

                    ListItem(
                        headlineContent = { Text(stringResource(R.string.settings_label_version)) },
                        trailingContent = {
                            Text(
                                packageInfo?.versionName ?: "1.0.0",
                                style = MaterialTheme.typography.bodyMedium,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        },
                        colors = ListItemDefaults.colors(containerColor = Color.Transparent)
                    )

                    HorizontalDivider(
                        modifier = Modifier.padding(horizontal = 16.dp),
                        color = MaterialTheme.colorScheme.outlineVariant
                    )

                    ListItem(
                        headlineContent = { Text(stringResource(R.string.settings_label_build)) },
                        trailingContent = {
                            Text(
                                packageInfo?.longVersionCode?.toString() ?: "1",
                                style = MaterialTheme.typography.bodyMedium,
                                color = MaterialTheme.colorScheme.onSurfaceVariant
                            )
                        },
                        colors = ListItemDefaults.colors(containerColor = Color.Transparent)
                    )
                }
            }

            Spacer(modifier = Modifier.height(16.dp))
        }
    }
}

