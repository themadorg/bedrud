package com.bedrud.app.core.deeplink

import java.net.URI

data class BedrudMeetingURL(
    val serverBaseURL: String,
    val roomName: String
)

object BedrudURLParser {
    /**
     * Parses a Bedrud meeting URL.
     *
     * Accepted formats:
     * - `https://server.com/c/room-name`
     * - `https://server.com/m/room-name`
     * - `server.com/c/room-name` (scheme defaults to https)
     * - `https://server.com:8080/c/room-name` (port numbers)
     */
    fun parse(rawURL: String): BedrudMeetingURL? {
        var urlString = rawURL.trim()

        // Add scheme if missing
        if (!urlString.contains("://")) {
            urlString = "https://$urlString"
        }

        val uri = try {
            URI(urlString)
        } catch (_: Exception) {
            return null
        }

        val host = uri.host
        if (host.isNullOrEmpty()) return null

        val pathComponents = (uri.path ?: "")
            .split("/")
            .filter { it.isNotEmpty() }

        // Expect /c/<room> or /m/<room>
        if (pathComponents.size < 2) return null
        if (pathComponents[0] !in listOf("c", "m")) return null

        val roomName = pathComponents[1]
        if (roomName.isEmpty()) return null

        // Build base URL: scheme + host + optional port
        val scheme = uri.scheme ?: "https"
        var baseURL = "$scheme://$host"
        if (uri.port > 0) {
            baseURL += ":${uri.port}"
        }

        return BedrudMeetingURL(serverBaseURL = baseURL, roomName = roomName)
    }

    /**
     * Resolves pasted join input to a room slug.
     * Accepts full meeting URLs (any host), `/m/` or `/c/` paths, or a plain room name.
     */
    fun parseJoinInput(rawInput: String): String? {
        val trimmed = rawInput.trim()
        if (trimmed.isEmpty()) return null

        parse(trimmed)?.roomName?.let { return it }

        if (!trimmed.contains("://") && !trimmed.contains("/")) {
            return trimmed
        }

        val path = trimmed.substringBefore('?').substringBefore('#')
        for (prefix in listOf("/m/", "/c/")) {
            val index = path.indexOf(prefix)
            if (index >= 0) {
                val slug = path.substring(index + prefix.length).trim('/').substringBefore('/')
                if (slug.isNotEmpty()) return slug
            }
        }

        return null
    }

    /** Builds a shareable meeting join URL: `{server}/m/{roomName}`. */
    fun buildMeetingLink(serverBaseURL: String, roomName: String): String {
        val base = serverBaseURL.trim().trimEnd('/')
        val slug = roomName.trim().trim('/')
        return "$base/m/$slug"
    }
}
