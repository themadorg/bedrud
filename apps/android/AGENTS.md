# Bedrud Android

Kotlin + Jetpack Compose + Material 3. Single `:app` module. minSdk 28, compileSdk/targetSdk 36, JDK 17.

## Build & Test

```bash
./gradlew assembleDebug          # Debug APK → app/build/outputs/apk/debug/
./gradlew assembleRelease        # Release APK (needs keystore.properties)
./gradlew test                   # Unit tests only (src/test/)
```

No instrumented test directory. No `make` target for Android tests — run `./gradlew test` directly.

**Test stack:** JUnit 4, MockK, OkHttp MockWebServer, kotlinx-coroutines-test.
**Test util:** `InMemorySharedPreferences` in `testutil/` — inject into any class taking `SharedPreferences` (InstanceStore, AuthManager). Avoid Android framework dependency.

**Make aliases from repo root:** `make build-android-debug`, `make build-android`, `make install-android`, `make release-android`.

## Architecture

```
app/src/main/java/com/bedrud/app/
├── BedrudApplication.kt        Koin init + instance migration
├── MainActivity.kt             NavHost, routes, deep links, PiP
├── core/
│   ├── di/AppModule.kt         Koin module (4 singletons)
│   ├── instance/               Multi-instance: InstanceStore → InstanceManager
│   ├── auth/                   AuthManager (encrypted prefs), PasskeyManager, OAuthLoginHandler
│   ├── api/                    Retrofit interfaces: AuthApi, RoomApi, AdminApi + ApiClientFactory
│   ├── livekit/RoomManager.kt  LiveKit room lifecycle, media toggles, chat
│   ├── pip/PipState.kt         PiP state holder
│   └── call/                   CallService + CallConnectionService (telecom integration)
├── models/                     Data classes (Gson-serialized)
└── ui/
    ├── theme/                  Color, Type, Theme
    ├── components/             BedrudButton (5 variants), BedrudCard
    └── screens/                Compose screens per route
```

**Navigation routes** in `Routes` object, `MainActivity.kt`:
`ADD_INSTANCE → LOGIN → REGISTER → GUEST_LOGIN → MAIN (bottom nav) → MEETING/{roomName}`

No ViewModels. State in `MutableStateFlow` on manager classes (RoomManager, AuthManager, InstanceManager) and screen-level stores (SettingsStore). Collected in composables via `collectAsState()`.

## Multi-Instance Model

App connects to user-chosen Bedrud server instances, not fixed backend.

- `InstanceStore` — persists instance list + active ID in plain `SharedPreferences` ("bedrud_instances")
- `InstanceManager` — central wiring hub. `rebuild()` creates fresh `AuthManager`, `ApiClientFactory`, Retrofit interfaces, `PasskeyManager`, and `RoomManager` for active instance. All exposed as `StateFlow<T?>`.
- `AuthManager` — per-instance `EncryptedSharedPreferences` ("bedrud_secure_$instanceId") storing JWT tokens + user JSON.
- API base URL: `{serverURL}/api` (computed property on `Instance`).
- Health check (`GET /api/health`) runs before adding new instance.

Switching instances: `instanceManager.switchTo(id)` → sets active → rebuilds all clients → UI reacts to StateFlow changes.

## Networking

Retrofit + OkHttp + Gson (not kotlinx-serialization for HTTP). `kotlin-serialization` plugin enabled but used elsewhere.

- `AuthInterceptor` — attaches `Authorization: Bearer <token>` to every request
- `TokenAuthenticator` — handles 401 by refreshing token synchronously (creates separate Retrofit to avoid recursion), retries once, forces logout on failure
- Base URL format: `https://host/api/` (trailing slash appended by `ApiClientFactory`)

## Key Conventions

- **Buttons:** Use `BedrudButton` with `BedrudButtonVariant` enum (PRIMARY, SECONDARY, OUTLINE, GHOST, DESTRUCTIVE). 44dp height, 8dp corner radius, 24dp horizontal padding.
- **Cards:** Use `BedrudCard`. 12dp corner radius, 1dp outline border, 0dp elevation, 16dp padding.
- **Colors:** Always `MaterialTheme.colorScheme.*`. Never hardcode hex in screens/components. Theme tokens in `ui/theme/Color.kt` mapped from web CSS HSL variables.
- **Serialization:** `@SerializedName` annotations on model fields (Gson). Snake_case from server ↔ camelCase in Kotlin.
- **DI:** Koin. Single module (`appModule`). Inject with `by inject()` in Activities, `by koinViewModel()` or `koinInject()` in composables.
- **Strings:** No `strings.xml` resource layer — UI strings inline in composables.

## Release Signing

Requires `keystore.properties` at project root (gitignored):
```properties
storeFile=path/to/keystore.jks
storePassword=...
keyAlias=...
keyPassword=...
```

Without it, release builds fail. Debug builds work without it.

## ProGuard

Release builds use minification + resource shrinking. Rules in `app/proguard-rules.pro` keep LiveKit, Retrofit interfaces, `models.**`, and Credential Manager classes.

## Deep Links

- `https://bedrud.com/m/{roomName}` → join room
- `https://bedrud.com/c/{roomName}` → join room
- `bedrud://oauth` → OAuth callback (expects `?token=...`)

Parsed in `BedrudURLParser`, handled in `MainActivity.handleDeepLink()`.

## Skills Reference

| Skill                         | When to Use                                                             | Example Scenarios for Bedrud                                                                          |
|-------------------------------|-------------------------------------------------------------------------|-------------------------------------------------------------------------------------------------------|
| **android-accessibility**     | Auditing or fixing accessibility issues in Compose UI                   | Adding contentDescription to buttons, ensuring screen reader compatibility for meeting controls       |
| **android-architecture**      | Setting up project structure, modules, or dependency injection          | Adding new feature modules, restructuring to support new authentication flow, optimizing Koin modules |
| **android-gradle-logic**      | Setting up scalable Gradle build configuration                          | Adding Convention Plugins, managing Version Catalogs, optimizing build times                          |
| **android-jetpack-compose**   | Building new UI screens or managing Compose state                       | Creating new meeting screens, implementing room control UI, handling remember/mutableStateOf          |
| **android-kotlin**            | General Android Kotlin development, coroutines, testing                 | Writing coroutines for network calls, using MockK for unit tests, Hilt injection                      |
| **compose-performance-audit** | Diagnosing slow rendering, janky scrolling, or excessive recompositions | Optimizing meeting room list scrolling, reducing RoomManager state collection overhead                |
| **compose-ui**                | Writing or refactoring Composables with best practices                  | Implementing state hoisting in MeetingScreen, optimizing component recomposition, applying theming    |
| **kotlin-concurrency-expert** | Reviewing or fixing coroutine/thread-safety issues                      | Resolving race conditions in RoomManager, fixing lifecycle scope issues                               |
| **gradle-build-performance**  | Debugging slow builds or CI/CD performance                              | Analyzing build scans, identifying compilation bottlenecks in multi-instance setup                    |
| **xml-to-compose-migration**  | Converting legacy XML layouts to Compose                                | Migrating any old View-based layouts (if any remain) to Compose components                            |
| **Kotlin Error Debugging**    | Debugging complex Kotlin errors or coroutine stack traces               | Debugging StateFlow emission issues, platform type warnings, or crashes                               |
