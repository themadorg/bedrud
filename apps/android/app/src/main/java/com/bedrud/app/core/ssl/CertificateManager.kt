package com.bedrud.app.core.ssl

import java.security.KeyStore
import java.security.SecureRandom
import java.security.cert.X509Certificate
import java.text.SimpleDateFormat
import java.util.Date
import java.util.Locale
import javax.net.ssl.SSLContext
import javax.net.ssl.SSLSocketFactory
import javax.net.ssl.TrustManager
import javax.net.ssl.TrustManagerFactory
import javax.net.ssl.X509TrustManager

data class CertificateInfo(
    val fingerprint: String,
    val subjectCN: String,
    val issuerCN: String,
    val serialNumber: String,
    val validFrom: Long,
    val validUntil: Long
) {
    fun formattedFingerprint(): String {
        return fingerprint.chunked(2).joinToString(":")
    }

    fun validFromDate(): String = DATE_FORMAT.format(Date(validFrom))
    fun validUntilDate(): String = DATE_FORMAT.format(Date(validUntil))
    fun isExpired(): Boolean = System.currentTimeMillis() > validUntil
    fun isNotYetValid(): Boolean = System.currentTimeMillis() < validFrom

    companion object {
        private val DATE_FORMAT = SimpleDateFormat("MMM dd, yyyy", Locale.US)

        fun fromCertificate(cert: X509Certificate): CertificateInfo {
            return CertificateInfo(
                fingerprint = sha256Fingerprint(cert),
                subjectCN = extractCN(cert.subjectX500Principal.name),
                issuerCN = extractCN(cert.issuerX500Principal.name),
                serialNumber = cert.serialNumber.toString(16),
                validFrom = cert.notBefore.time,
                validUntil = cert.notAfter.time
            )
        }

        private fun sha256Fingerprint(cert: X509Certificate): String {
            val digest = java.security.MessageDigest.getInstance("SHA-256")
            val hash = digest.digest(cert.encoded)
            return hash.joinToString("") { "%02x".format(it) }
        }

        private fun extractCN(dn: String): String {
            val parts = dn.split(",")
            for (part in parts) {
                val trimmed = part.trim()
                if (trimmed.startsWith("CN=", ignoreCase = true)) {
                    return trimmed.removePrefix("CN=").removePrefix("cn=")
                }
            }
            return dn
        }
    }
}

class CapturingTrustManager : X509TrustManager {

    private var captured: X509Certificate? = null

    fun getCapturedCertificate(): X509Certificate? = captured

    fun getCertificateInfo(): CertificateInfo? {
        return captured?.let { CertificateInfo.fromCertificate(it) }
    }

    override fun checkClientTrusted(chain: Array<X509Certificate>, authType: String) {
    }

    override fun checkServerTrusted(chain: Array<X509Certificate>, authType: String) {
        if (chain.isNotEmpty()) {
            captured = chain[0]
        }
    }

    override fun getAcceptedIssuers(): Array<X509Certificate> = emptyArray()
}

object CertificateManager {

    fun createPinnedTrustManager(certificate: X509Certificate): X509TrustManager {
        val keyStore = KeyStore.getInstance(KeyStore.getDefaultType()).apply {
            load(null, null)
            setCertificateEntry("server", certificate)
        }

        val tmf = TrustManagerFactory.getInstance(TrustManagerFactory.getDefaultAlgorithm())
        tmf.init(keyStore)

        val trustManager = tmf.trustManagers.firstOrNull { it is X509TrustManager }
            ?: throw IllegalStateException("No X509TrustManager found from TrustManagerFactory")

        return trustManager as X509TrustManager
    }

    fun createPinnedSSLSocketFactory(certificate: X509Certificate): SSLSocketFactory {
        val trustManager = createPinnedTrustManager(certificate)
        val sslContext = SSLContext.getInstance("TLS")
        sslContext.init(null, arrayOf<TrustManager>(trustManager), SecureRandom())
        return sslContext.socketFactory
    }

    fun createCapturingSSLSocketFactory(): Pair<SSLSocketFactory, CapturingTrustManager> {
        val capturingTM = CapturingTrustManager()
        val sslContext = SSLContext.getInstance("TLS")
        sslContext.init(null, arrayOf<TrustManager>(capturingTM), SecureRandom())
        return Pair(sslContext.socketFactory, capturingTM)
    }

    fun createDefaultSSLSocketFactory(): SSLSocketFactory {
        return SSLContext.getInstance("TLS").apply {
            init(null, null, SecureRandom())
        }.socketFactory
    }
}
