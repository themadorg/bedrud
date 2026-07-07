package com.bedrud.app.ui.screens.auth

import androidx.compose.foundation.layout.Column
import androidx.compose.foundation.layout.Spacer
import androidx.compose.foundation.layout.fillMaxSize
import androidx.compose.foundation.layout.fillMaxWidth
import androidx.compose.foundation.layout.height
import androidx.compose.foundation.layout.padding
import androidx.compose.foundation.layout.size
import androidx.compose.foundation.layout.width
import androidx.compose.foundation.rememberScrollState
import androidx.compose.foundation.text.KeyboardActions
import androidx.compose.foundation.text.KeyboardOptions
import androidx.compose.foundation.verticalScroll
import androidx.compose.material.icons.Icons
import androidx.compose.material.icons.automirrored.filled.ArrowBack
import androidx.compose.material.icons.filled.Visibility
import androidx.compose.material.icons.filled.VisibilityOff
import androidx.compose.material3.Button
import androidx.compose.material3.CircularProgressIndicator
import androidx.compose.material3.ExperimentalMaterial3Api
import androidx.compose.material3.Icon
import androidx.compose.material3.IconButton
import androidx.compose.material3.MaterialTheme
import androidx.compose.material3.OutlinedTextField
import androidx.compose.material3.Scaffold
import androidx.compose.material3.SnackbarHost
import androidx.compose.material3.SnackbarHostState
import androidx.compose.material3.Text
import androidx.compose.material3.TextButton
import androidx.compose.material3.TopAppBar
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
import androidx.compose.ui.autofill.ContentType
import androidx.compose.ui.focus.FocusDirection
import androidx.compose.ui.platform.LocalFocusManager
import androidx.compose.ui.res.stringResource
import androidx.compose.ui.text.input.ImeAction
import androidx.compose.ui.text.input.KeyboardType
import androidx.compose.ui.text.input.PasswordVisualTransformation
import androidx.compose.ui.text.input.VisualTransformation
import androidx.compose.ui.text.style.TextDirection
import androidx.compose.ui.unit.dp
import com.bedrud.app.R
import com.bedrud.app.ui.components.BedrudScaffoldContentInsets
import com.bedrud.app.core.api.LoginOutcome
import com.bedrud.app.ui.components.autofillType
import com.bedrud.app.core.api.RegisterOutcome
import com.bedrud.app.core.api.parseRegisterResponse
import com.bedrud.app.core.api.performLogin
import com.bedrud.app.core.instance.InstanceManager
import com.bedrud.app.models.RegisterRequest
import kotlinx.coroutines.launch
import org.koin.compose.koinInject

@OptIn(ExperimentalMaterial3Api::class)
@Composable
fun RegisterScreen(
    onRegisterSuccess: () -> Unit,
    onNavigateToLogin: () -> Unit,
    instanceManager: InstanceManager = koinInject()
) {
    val authApi = instanceManager.authApi.collectAsState().value ?: return
    val authManager = instanceManager.authManager.collectAsState().value ?: return
    val scope = rememberCoroutineScope()
    val focusManager = LocalFocusManager.current
    val snackbarHostState = remember { SnackbarHostState() }

    var name by rememberSaveable { mutableStateOf("") }
    var email by rememberSaveable { mutableStateOf("") }
    var password by rememberSaveable { mutableStateOf("") }
    var confirmPassword by rememberSaveable { mutableStateOf("") }
    var passwordVisible by rememberSaveable { mutableStateOf(false) }
    var isLoading by remember { mutableStateOf(false) }
    var errorMessage by remember { mutableStateOf<String?>(null) }

    val passwordMismatchError = stringResource(R.string.auth_error_passwordMismatch)
    val formValid = name.isNotBlank() &&
        email.isNotBlank() &&
        password.isNotBlank() &&
        confirmPassword.isNotBlank() &&
        password == confirmPassword

    LaunchedEffect(errorMessage) {
        errorMessage?.let {
            snackbarHostState.showSnackbar(it)
            errorMessage = null
        }
    }

    Scaffold(
        contentWindowInsets = BedrudScaffoldContentInsets,
        topBar = {
            TopAppBar(
                title = { Text(stringResource(R.string.auth_title_createAccount)) },
                navigationIcon = {
                    IconButton(onClick = onNavigateToLogin, enabled = !isLoading) {
                        Icon(
                            Icons.AutoMirrored.Filled.ArrowBack,
                            contentDescription = stringResource(R.string.common_action_back)
                        )
                    }
                }
            )
        },
        snackbarHost = { SnackbarHost(snackbarHostState) }
    ) { padding ->
        Column(
            modifier = Modifier
                .fillMaxSize()
                .padding(padding)
                .padding(horizontal = 24.dp)
                .verticalScroll(rememberScrollState()),
            horizontalAlignment = Alignment.CenterHorizontally
        ) {
            Spacer(modifier = Modifier.height(24.dp))

            Text(
                text = "Join Bedrud",
                style = MaterialTheme.typography.headlineMedium
            )

            Spacer(modifier = Modifier.height(8.dp))

            Text(
                text = "Create your account to get started",
                style = MaterialTheme.typography.bodyMedium,
                color = MaterialTheme.colorScheme.onSurfaceVariant
            )

            Spacer(modifier = Modifier.height(32.dp))

            OutlinedTextField(
                value = name,
                onValueChange = { name = it },
                label = { Text(stringResource(R.string.auth_label_fullName)) },
                enabled = !isLoading,
                keyboardOptions = KeyboardOptions(
                    keyboardType = KeyboardType.Text,
                    imeAction = ImeAction.Next
                ),
                keyboardActions = KeyboardActions(
                    onNext = { focusManager.moveFocus(FocusDirection.Down) }
                ),
                singleLine = true,
                modifier = Modifier
                    .fillMaxWidth()
                    .autofillType(ContentType.PersonFullName),
                textStyle = MaterialTheme.typography.bodyMedium.copy(textDirection = TextDirection.Content)
            )

            Spacer(modifier = Modifier.height(12.dp))

            OutlinedTextField(
                value = email,
                onValueChange = { email = it },
                label = { Text(stringResource(R.string.auth_label_email)) },
                enabled = !isLoading,
                keyboardOptions = KeyboardOptions(
                    keyboardType = KeyboardType.Email,
                    imeAction = ImeAction.Next
                ),
                keyboardActions = KeyboardActions(
                    onNext = { focusManager.moveFocus(FocusDirection.Down) }
                ),
                singleLine = true,
                modifier = Modifier
                    .fillMaxWidth()
                    .autofillType(ContentType.EmailAddress),
                textStyle = MaterialTheme.typography.bodyMedium.copy(textDirection = TextDirection.Ltr)
            )

            Spacer(modifier = Modifier.height(12.dp))

            OutlinedTextField(
                value = password,
                onValueChange = { password = it },
                label = { Text(stringResource(R.string.auth_label_password)) },
                enabled = !isLoading,
                trailingIcon = {
                    IconButton(onClick = { passwordVisible = !passwordVisible }) {
                        Icon(
                            if (passwordVisible) Icons.Default.VisibilityOff
                            else Icons.Default.Visibility,
                            contentDescription = if (passwordVisible) {
                                stringResource(R.string.auth_password_toggle_hide)
                            } else {
                                stringResource(R.string.auth_password_toggle_show)
                            }
                        )
                    }
                },
                visualTransformation = if (passwordVisible) {
                    VisualTransformation.None
                } else {
                    PasswordVisualTransformation()
                },
                keyboardOptions = KeyboardOptions(
                    keyboardType = KeyboardType.Password,
                    imeAction = ImeAction.Next
                ),
                keyboardActions = KeyboardActions(
                    onNext = { focusManager.moveFocus(FocusDirection.Down) }
                ),
                singleLine = true,
                modifier = Modifier
                    .fillMaxWidth()
                    .autofillType(ContentType.NewPassword),
                textStyle = MaterialTheme.typography.bodyMedium.copy(textDirection = TextDirection.Ltr)
            )

            Spacer(modifier = Modifier.height(12.dp))

            OutlinedTextField(
                value = confirmPassword,
                onValueChange = { confirmPassword = it },
                label = { Text(stringResource(R.string.auth_label_confirmPassword)) },
                enabled = !isLoading,
                visualTransformation = if (passwordVisible) {
                    VisualTransformation.None
                } else {
                    PasswordVisualTransformation()
                },
                keyboardOptions = KeyboardOptions(
                    keyboardType = KeyboardType.Password,
                    imeAction = ImeAction.Done
                ),
                keyboardActions = KeyboardActions(
                    onDone = { focusManager.clearFocus() }
                ),
                singleLine = true,
                isError = confirmPassword.isNotEmpty() && password != confirmPassword,
                supportingText = if (confirmPassword.isNotEmpty() && password != confirmPassword) {
                    { Text(stringResource(R.string.auth_error_passwordMismatch)) }
                } else {
                    null
                },
                modifier = Modifier
                    .fillMaxWidth()
                    .autofillType(ContentType.NewPassword)
            )

            Spacer(modifier = Modifier.height(24.dp))

            Button(
                onClick = {
                    if (isLoading) return@Button
                    if (password != confirmPassword) {
                        errorMessage = passwordMismatchError
                        return@Button
                    }
                    scope.launch {
                        isLoading = true
                        try {
                            val trimmedEmail = email.trim()
                            val trimmedName = name.trim()

                            val registerResponse = authApi.register(
                                RegisterRequest(
                                    email = trimmedEmail,
                                    password = password,
                                    name = trimmedName
                                )
                            )
                            when (val registerOutcome = parseRegisterResponse(registerResponse)) {
                                is RegisterOutcome.Failed -> {
                                    errorMessage = registerOutcome.message
                                    return@launch
                                }
                                is RegisterOutcome.VerificationRequired -> {
                                    errorMessage = registerOutcome.message
                                    onNavigateToLogin()
                                    return@launch
                                }
                                is RegisterOutcome.AccountCreated -> {
                                    when (
                                        val loginOutcome = performLogin(
                                            authApi = authApi,
                                            authManager = authManager,
                                            email = trimmedEmail,
                                            password = password
                                        )
                                    ) {
                                        is LoginOutcome.Success -> onRegisterSuccess()
                                        is LoginOutcome.VerificationRequired -> {
                                            errorMessage = loginOutcome.message
                                            onNavigateToLogin()
                                        }
                                        is LoginOutcome.Failed -> {
                                            errorMessage = loginOutcome.message
                                        }
                                    }
                                }
                            }
                        } catch (e: Exception) {
                            errorMessage = e.message ?: "An error occurred"
                        } finally {
                            isLoading = false
                        }
                    }
                },
                enabled = formValid,
                modifier = Modifier
                    .fillMaxWidth()
                    .height(52.dp)
            ) {
                if (isLoading) {
                    CircularProgressIndicator(
                        modifier = Modifier.size(20.dp),
                        strokeWidth = 2.dp,
                        color = MaterialTheme.colorScheme.onPrimary
                    )
                    Spacer(modifier = Modifier.width(8.dp))
                }
                Text(stringResource(R.string.auth_title_createAccount))
            }

            Spacer(modifier = Modifier.height(16.dp))

            TextButton(onClick = onNavigateToLogin, enabled = !isLoading) {
                Text(stringResource(R.string.auth_link_alreadyHaveAccount))
            }

            Spacer(modifier = Modifier.height(48.dp))
        }
    }
}