package com.bedrud.app.core

import android.text.BidiFormatter
import androidx.compose.ui.text.style.TextDirection

object BidiUtils {

    private val formatter: BidiFormatter = BidiFormatter.getInstance()

    // Mirrors web/excalidraw RTL detection: first strong char in RTL script range.
    private val startsRtlPattern = Regex(
        "^[^A-Za-z\u00C0-\u00D6\u00D8-\u00F6\u00F8-\u02B8\u0300-\u0590\u0800-\u1FFF" +
            "\u2C00-\uFB1C\uFDFE-\uFE6F\uFEFD-\uFFFF]*" +
            "[\u0591-\u07FF\uFB1D-\uFDFD\uFE70-\uFEFC]"
    )

    fun startsRtl(text: String): Boolean = startsRtlPattern.containsMatchIn(text)

    fun textDirection(text: String): TextDirection =
        if (startsRtl(text)) TextDirection.Rtl else TextDirection.Ltr

    fun wrap(text: String): String = formatter.unicodeWrap(text)
}