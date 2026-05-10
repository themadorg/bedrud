package com.bedrud.app.core

import android.text.BidiFormatter

object BidiUtils {

    private val formatter: BidiFormatter = BidiFormatter.getInstance()

    fun wrap(text: String): String = formatter.unicodeWrap(text)
}
