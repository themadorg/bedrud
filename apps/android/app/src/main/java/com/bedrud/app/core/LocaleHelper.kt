package com.bedrud.app.core

import android.content.Context
import android.content.res.Configuration
import android.os.Build
import java.util.Locale

fun Context.createLocaleContext(localeTag: String): Context {
    val locale = if (localeTag.isNotEmpty()) {
        Locale.forLanguageTag(localeTag)
    } else {
        Locale.getDefault()
    }
    Locale.setDefault(locale)
    val config = Configuration(this.resources.configuration).apply {
        if (Build.VERSION.SDK_INT >= Build.VERSION_CODES.TIRAMISU) {
            setLocales(android.os.LocaleList(locale))
        } else {
            @Suppress("DEPRECATION")
            setLocale(locale)
        }
    }
    return createConfigurationContext(config)
}
