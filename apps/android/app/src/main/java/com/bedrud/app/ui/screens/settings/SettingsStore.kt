package com.bedrud.app.ui.screens.settings

import android.content.Context
import android.content.SharedPreferences
import android.text.TextUtils
import android.view.View
import androidx.annotation.StringRes
import com.bedrud.app.R
import java.util.Locale
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asStateFlow

enum class AppAppearance(val label: String, @param:StringRes val stringResId: Int) {
    SYSTEM("System", R.string.settings_theme_system),
    LIGHT("Light", R.string.settings_theme_light),
    DARK("Dark", R.string.settings_theme_dark);
}

enum class AppLanguage(val localeTag: String, val label: String, val isRtl: Boolean = false) {
    SYSTEM("", "System"),
    ENGLISH("en", "English"),
    PERSIAN("fa", "فارسی", isRtl = true),
    ARABIC("ar", "العربية", isRtl = true),
    FRENCH("fr", "Français"),
    GERMAN("de", "Deutsch"),
    JAPANESE("ja", "日本語"),
    RUSSIAN("ru", "Русский"),
    SPANISH("es", "Español"),
    TURKISH("tr", "Türkçe"),
    CHINESE("zh", "中文");

    fun resolveIsRtl(): Boolean {
        if (this != SYSTEM) return isRtl
        return TextUtils.getLayoutDirectionFromLocale(Locale.getDefault()) == View.LAYOUT_DIRECTION_RTL
    }
}

class SettingsStore(context: Context) {

    private val prefs: SharedPreferences =
        context.getSharedPreferences("bedrud_settings", Context.MODE_PRIVATE)

    private val _appearance = MutableStateFlow(loadAppearance())
    val appearance: StateFlow<AppAppearance> = _appearance.asStateFlow()

    private val _notificationsEnabled = MutableStateFlow(
        prefs.getBoolean(KEY_NOTIFICATIONS, true)
    )
    val notificationsEnabled: StateFlow<Boolean> = _notificationsEnabled.asStateFlow()

    private val _language = MutableStateFlow(loadLanguage())
    val language: StateFlow<AppLanguage> = _language.asStateFlow()

    fun setAppearance(value: AppAppearance) {
        prefs.edit().putString(KEY_APPEARANCE, value.name).apply()
        _appearance.value = value
    }

    fun setNotificationsEnabled(value: Boolean) {
        prefs.edit().putBoolean(KEY_NOTIFICATIONS, value).apply()
        _notificationsEnabled.value = value
    }

    fun setLanguage(value: AppLanguage) {
        prefs.edit().putString(KEY_LANGUAGE, value.name).apply()
        _language.value = value
    }

    fun getLanguageTag(): String {
        val raw = prefs.getString(KEY_LANGUAGE, AppLanguage.SYSTEM.name)
        return try {
            AppLanguage.valueOf(raw ?: AppLanguage.SYSTEM.name).localeTag
        } catch (_: Exception) {
            AppLanguage.SYSTEM.localeTag
        }
    }

    fun getLastTab(): Int = prefs.getInt(KEY_LAST_TAB, 0)

    fun setLastTab(index: Int) {
        prefs.edit().putInt(KEY_LAST_TAB, index).apply()
    }

    private fun loadAppearance(): AppAppearance {
        val raw = prefs.getString(KEY_APPEARANCE, AppAppearance.SYSTEM.name)
        return try {
            AppAppearance.valueOf(raw ?: AppAppearance.SYSTEM.name)
        } catch (_: Exception) {
            AppAppearance.SYSTEM
        }
    }

    private fun loadLanguage(): AppLanguage {
        val raw = prefs.getString(KEY_LANGUAGE, AppLanguage.SYSTEM.name)
        return try {
            AppLanguage.valueOf(raw ?: AppLanguage.SYSTEM.name)
        } catch (_: Exception) {
            AppLanguage.SYSTEM
        }
    }

    companion object {
        private const val KEY_APPEARANCE = "bedrud_appearance"
        private const val KEY_NOTIFICATIONS = "bedrud_notifications_enabled"
        private const val KEY_LANGUAGE = "bedrud_language"
        private const val KEY_LAST_TAB = "bedrud_last_tab"
    }
}
