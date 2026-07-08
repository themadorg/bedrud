package com.bedrud.app.ui.theme

import androidx.compose.material3.Typography
import androidx.compose.ui.text.ExperimentalTextApi
import androidx.compose.ui.text.TextStyle
import androidx.compose.ui.text.font.Font
import androidx.compose.ui.text.font.FontFamily
import androidx.compose.ui.text.font.FontVariation
import androidx.compose.ui.text.font.FontWeight
import androidx.compose.ui.unit.sp
import com.bedrud.app.R

private fun buildVazirmatnFamily(): FontFamily {
    @OptIn(ExperimentalTextApi::class)
    return FontFamily(
        Font(R.font.vazirmatn, weight = FontWeight.Normal, variationSettings = FontVariation.Settings(FontVariation.Setting("wght", 400f))),
        Font(R.font.vazirmatn, weight = FontWeight.Medium, variationSettings = FontVariation.Settings(FontVariation.Setting("wght", 500f))),
        Font(R.font.vazirmatn, weight = FontWeight.SemiBold, variationSettings = FontVariation.Settings(FontVariation.Setting("wght", 600f))),
        Font(R.font.vazirmatn, weight = FontWeight.Bold, variationSettings = FontVariation.Settings(FontVariation.Setting("wght", 700f)))
    )
}

val VazirmatnFontFamily = buildVazirmatnFamily()

private fun buildShabnamFamily(): FontFamily {
    return FontFamily(
        Font(R.font.shabnam, weight = FontWeight.Normal),
        Font(R.font.shabnam, weight = FontWeight.Medium),
        Font(R.font.shabnam, weight = FontWeight.SemiBold),
        Font(R.font.shabnam, weight = FontWeight.Bold)
    )
}

val ShabnamFontFamily = buildShabnamFamily()
val BedrudTypography = Typography(
    displayLarge = TextStyle(
        fontFamily = FontFamily.SansSerif,
        fontWeight = FontWeight.Bold,
        fontSize = 57.sp,
        lineHeight = 64.sp,
        letterSpacing = (-0.25).sp
    ),
    displayMedium = TextStyle(
        fontFamily = FontFamily.SansSerif,
        fontWeight = FontWeight.Bold,
        fontSize = 45.sp,
        lineHeight = 52.sp,
        letterSpacing = 0.sp
    ),
    displaySmall = TextStyle(
        fontFamily = FontFamily.SansSerif,
        fontWeight = FontWeight.Bold,
        fontSize = 36.sp,
        lineHeight = 44.sp,
        letterSpacing = 0.sp
    ),
    headlineLarge = TextStyle(
        fontFamily = FontFamily.SansSerif,
        fontWeight = FontWeight.SemiBold,
        fontSize = 32.sp,
        lineHeight = 40.sp,
        letterSpacing = 0.sp
    ),
    headlineMedium = TextStyle(
        fontFamily = FontFamily.SansSerif,
        fontWeight = FontWeight.SemiBold,
        fontSize = 28.sp,
        lineHeight = 36.sp,
        letterSpacing = 0.sp
    ),
    headlineSmall = TextStyle(
        fontFamily = FontFamily.SansSerif,
        fontWeight = FontWeight.SemiBold,
        fontSize = 24.sp,
        lineHeight = 32.sp,
        letterSpacing = 0.sp
    ),
    titleLarge = TextStyle(
        fontFamily = FontFamily.SansSerif,
        fontWeight = FontWeight.SemiBold,
        fontSize = 22.sp,
        lineHeight = 28.sp,
        letterSpacing = 0.sp
    ),
    titleMedium = TextStyle(
        fontFamily = FontFamily.SansSerif,
        fontWeight = FontWeight.Medium,
        fontSize = 16.sp,
        lineHeight = 24.sp,
        letterSpacing = 0.15.sp
    ),
    titleSmall = TextStyle(
        fontFamily = FontFamily.SansSerif,
        fontWeight = FontWeight.Medium,
        fontSize = 14.sp,
        lineHeight = 20.sp,
        letterSpacing = 0.1.sp
    ),
    bodyLarge = TextStyle(
        fontFamily = FontFamily.SansSerif,
        fontWeight = FontWeight.Normal,
        fontSize = 16.sp,
        lineHeight = 24.sp,
        letterSpacing = 0.5.sp
    ),
    bodyMedium = TextStyle(
        fontFamily = FontFamily.SansSerif,
        fontWeight = FontWeight.Normal,
        fontSize = 14.sp,
        lineHeight = 20.sp,
        letterSpacing = 0.25.sp
    ),
    bodySmall = TextStyle(
        fontFamily = FontFamily.SansSerif,
        fontWeight = FontWeight.Normal,
        fontSize = 12.sp,
        lineHeight = 16.sp,
        letterSpacing = 0.4.sp
    ),
    labelLarge = TextStyle(
        fontFamily = FontFamily.SansSerif,
        fontWeight = FontWeight.Medium,
        fontSize = 14.sp,
        lineHeight = 20.sp,
        letterSpacing = 0.1.sp
    ),
    labelMedium = TextStyle(
        fontFamily = FontFamily.SansSerif,
        fontWeight = FontWeight.Medium,
        fontSize = 12.sp,
        lineHeight = 16.sp,
        letterSpacing = 0.5.sp
    ),
    labelSmall = TextStyle(
        fontFamily = FontFamily.SansSerif,
        fontWeight = FontWeight.Medium,
        fontSize = 11.sp,
        lineHeight = 16.sp,
        letterSpacing = 0.5.sp
    )
)

private fun rtlTypography(fontFamily: FontFamily) = Typography(
    displayLarge = BedrudTypography.displayLarge.copy(fontFamily = fontFamily),
    displayMedium = BedrudTypography.displayMedium.copy(fontFamily = fontFamily),
    displaySmall = BedrudTypography.displaySmall.copy(fontFamily = fontFamily),
    headlineLarge = BedrudTypography.headlineLarge.copy(fontFamily = fontFamily),
    headlineMedium = BedrudTypography.headlineMedium.copy(fontFamily = fontFamily),
    headlineSmall = BedrudTypography.headlineSmall.copy(fontFamily = fontFamily),
    titleLarge = BedrudTypography.titleLarge.copy(fontFamily = fontFamily),
    titleMedium = BedrudTypography.titleMedium.copy(fontFamily = fontFamily),
    titleSmall = BedrudTypography.titleSmall.copy(fontFamily = fontFamily),
    bodyLarge = BedrudTypography.bodyLarge.copy(fontFamily = fontFamily),
    bodyMedium = BedrudTypography.bodyMedium.copy(fontFamily = fontFamily),
    bodySmall = BedrudTypography.bodySmall.copy(fontFamily = fontFamily),
    labelLarge = BedrudTypography.labelLarge.copy(fontFamily = fontFamily),
    labelMedium = BedrudTypography.labelMedium.copy(fontFamily = fontFamily),
    labelSmall = BedrudTypography.labelSmall.copy(fontFamily = fontFamily)
)

val ShabnamTypography = rtlTypography(ShabnamFontFamily)
val VazirmatnTypography = rtlTypography(VazirmatnFontFamily)
