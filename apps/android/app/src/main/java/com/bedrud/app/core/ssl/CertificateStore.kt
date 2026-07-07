package com.bedrud.app.core.ssl

import android.content.Context
import java.io.File
import java.io.FileInputStream
import java.io.FileOutputStream
import java.security.cert.CertificateFactory
import java.security.cert.X509Certificate

class CertificateStore(private val certsDir: File) {

    constructor(context: Context) : this(File(context.filesDir, "certs"))

    init {
        certsDir.mkdirs()
    }

    fun saveCertificate(instanceId: String, certificate: X509Certificate) {
        val pem = encodeToPem(certificate)
        val file = certFile(instanceId)
        FileOutputStream(file).use { it.write(pem.toByteArray(Charsets.UTF_8)) }
    }

    fun getCertificate(instanceId: String): X509Certificate? {
        val file = certFile(instanceId)
        if (!file.exists()) return null
        val pem = FileInputStream(file).use { it.readBytes().toString(Charsets.UTF_8) }
        return decodeFromPem(pem)
    }

    fun hasCertificate(instanceId: String): Boolean {
        return certFile(instanceId).exists()
    }

    fun removeCertificate(instanceId: String) {
        certFile(instanceId).delete()
    }

    fun removeAll() {
        certsDir.listFiles()?.forEach { it.delete() }
    }

    private fun certFile(instanceId: String): File {
        return File(certsDir, "${instanceId}.crt")
    }

    companion object {
        private const val PEM_HEADER = "-----BEGIN CERTIFICATE-----"
        private const val PEM_FOOTER = "-----END CERTIFICATE-----"

        fun encodeToPem(certificate: X509Certificate): String {
            val base64 = java.util.Base64.getEncoder().encodeToString(certificate.encoded)
            return buildString {
                appendLine(PEM_HEADER)
                base64.chunked(64).forEach { appendLine(it) }
                appendLine(PEM_FOOTER)
            }
        }

        fun decodeFromPem(pem: String): X509Certificate? {
            val cleaned = pem
                .replace(PEM_HEADER, "")
                .replace(PEM_FOOTER, "")
                .replace("\r", "")
                .replace("\n", "")
                .replace(" ", "")
                .trim()
            if (cleaned.isEmpty()) return null
            return try {
                val der = java.util.Base64.getDecoder().decode(cleaned)
                val factory = CertificateFactory.getInstance("X.509")
                factory.generateCertificate(der.inputStream()) as X509Certificate
            } catch (e: Exception) {
                null
            }
        }
    }
}
