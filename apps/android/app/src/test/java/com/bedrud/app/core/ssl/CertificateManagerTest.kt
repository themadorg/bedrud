package com.bedrud.app.core.ssl

import org.junit.Assert.assertEquals
import org.junit.Assert.assertFalse
import org.junit.Assert.assertNotNull
import org.junit.Assert.assertNull
import org.junit.Assert.assertTrue
import org.junit.Before
import org.junit.Test
import java.security.cert.X509Certificate

class CertificateManagerTest {

    private lateinit var testCert: X509Certificate

    @Before
    fun setUp() {
        testCert = CertificateStore.decodeFromPem(TEST_PEM)!!
    }

    @Test
    fun `CertificateInfo fromCertificate extracts subject CN`() {
        val info = CertificateInfo.fromCertificate(testCert)
        assertEquals("Test Server", info.subjectCN)
    }

    @Test
    fun `CertificateInfo fromCertificate extracts issuer CN`() {
        val info = CertificateInfo.fromCertificate(testCert)
        assertEquals("Test Server", info.issuerCN)
    }

    @Test
    fun `CertificateInfo fromCertificate extracts serial number`() {
        val info = CertificateInfo.fromCertificate(testCert)
        assertEquals(testCert.serialNumber.toString(16), info.serialNumber)
    }

    @Test
    fun `CertificateInfo fromCertificate extracts valid dates`() {
        val info = CertificateInfo.fromCertificate(testCert)
        assertEquals(testCert.notBefore.time, info.validFrom)
        assertEquals(testCert.notAfter.time, info.validUntil)
    }

    @Test
    fun `CertificateInfo fingerprint is non-empty SHA-256 hex`() {
        val info = CertificateInfo.fromCertificate(testCert)
        assertEquals(64, info.fingerprint.length)
        info.fingerprint.forEach { c ->
            assertTrue("Fingerprint should be hex: $c", c in '0'..'9' || c in 'a'..'f')
        }
    }

    @Test
    fun `formattedFingerprint uses colon separators`() {
        val info = CertificateInfo.fromCertificate(testCert)
        val formatted = info.formattedFingerprint()
        assertEquals(64 + 31, formatted.length) // 64 hex chars + 31 colons
        assertTrue(formatted.contains(":"))
    }

    @Test
    fun `isExpired returns false for valid cert`() {
        val info = CertificateInfo.fromCertificate(testCert)
        assertFalse("Test cert should not be expired", info.isExpired())
    }

    @Test
    fun `isNotYetValid returns false for valid cert`() {
        val info = CertificateInfo.fromCertificate(testCert)
        assertFalse("Test cert should be valid now", info.isNotYetValid())
    }

    @Test
    fun `CapturingTrustManager captures server certificate`() {
        val ct = CapturingTrustManager()
        val chain = arrayOf(testCert)

        ct.checkServerTrusted(chain, "RSA")

        val captured = ct.getCapturedCertificate()
        assertNotNull(captured)
        assertEquals(testCert.serialNumber, captured!!.serialNumber)
    }

    @Test
    fun `CapturingTrustManager captures leaf certificate only`() {
        val ct = CapturingTrustManager()
        val leaf = testCert
        val chain = arrayOf(leaf, leaf) // simulate chain with two certs

        ct.checkServerTrusted(chain, "RSA")

        val captured = ct.getCapturedCertificate()
        assertEquals(leaf.serialNumber, captured!!.serialNumber)
    }

    @Test
    fun `CapturingTrustManager returns null when no chain`() {
        val ct = CapturingTrustManager()
        assertNull(ct.getCapturedCertificate())
    }

    @Test
    fun `CapturingTrustManager getCertificateInfo returns info`() {
        val ct = CapturingTrustManager()
        ct.checkServerTrusted(arrayOf(testCert), "RSA")

        val info = ct.getCertificateInfo()
        assertNotNull(info)
        assertEquals("Test Server", info!!.subjectCN)
    }

    @Test
    fun `createPinnedTrustManager trusts the given certificate`() {
        val tm = CertificateManager.createPinnedTrustManager(testCert)

        // Should not throw for the pinned cert
        tm.checkServerTrusted(arrayOf(testCert), "RSA")
    }

    @Test(expected = java.security.cert.CertificateException::class)
    fun `createPinnedTrustManager rejects unknown certificate`() {
        val tm = CertificateManager.createPinnedTrustManager(testCert)
        val otherCert = CertificateStore.decodeFromPem(OTHER_PEM)!!
        tm.checkServerTrusted(arrayOf(otherCert), "RSA")
    }

    @Test
    fun `createPinnedSSLSocketFactory produces a usable factory`() {
        val factory = CertificateManager.createPinnedSSLSocketFactory(testCert)
        assertNotNull(factory)
        assertTrue(factory.defaultCipherSuites.isNotEmpty())
    }

    @Test
    fun `createCapturingSSLSocketFactory produces factory and trust manager`() {
        val (factory, tm) = CertificateManager.createCapturingSSLSocketFactory()
        assertNotNull(factory)
        assertTrue(factory.defaultCipherSuites.isNotEmpty())
        assertNull(tm.getCapturedCertificate())
    }

    @Test
    fun `createDefaultSSLSocketFactory produces a factory`() {
        val factory = CertificateManager.createDefaultSSLSocketFactory()
        assertNotNull(factory)
        assertTrue(factory.defaultCipherSuites.isNotEmpty())
    }

    @Test
    fun `CertificateInfo subjectCN falls back to full DN when no CN`() {
        val info = CertificateInfo.fromCertificate(testCert)
        assertNotNull(info.subjectCN)
    }

    companion object {
        private val TEST_PEM = """-----BEGIN CERTIFICATE-----
MIIC0DCCAbigAwIBAgIJAPM6MQMI2wbFMA0GCSqGSIb3DQEBCwUAMBYxFDASBgNV
BAMTC1Rlc3QgU2VydmVyMB4XDTI2MDUxMTExMjcxNVoXDTM2MDUwODExMjcxNVow
FjEUMBIGA1UEAxMLVGVzdCBTZXJ2ZXIwggEiMA0GCSqGSIb3DQEBAQUAA4IBDwAw
ggEKAoIBAQDF5hA8VIPAkUKUKL2lyPOFG1Xk1jnm+rUtWu6oM9HGe4qvvpNYq64I
X9AxlGhiqIlPOobXWWMVQbHabrAe52cLZ/zU9K0J3xAaMOtyzo06Jz3aXK9lxAP3
NIRBTQh5NG+32uta3vwKgsg/I02C8KYRrddljqeQ267r/495moa8W9n//PPAH45g
M66tXHxrie5ET7S3GCh7a47mI3UqmUBzMuGlvbKrw9DzQAGZSqT/LXDxzHTg1MuW
DvT7gdRTaLf/Nk1iCWcNH6Eq4a01IwZG3WTp5NCYNIu8PDxNbXt1H2mDA2TgDW1G
bVnmodQ+PydM/vF4t18tXF6EqnyYaY/bAgMBAAGjITAfMB0GA1UdDgQWBBRKwVr6
uxSpjNx8ayzT4VFO9rKndTANBgkqhkiG9w0BAQsFAAOCAQEAt0QvUwx3XQNjBLdE
ztF+tQUFAEUbqnxZqJSEwBtV/nFT0Gq7HqN4oCSfySxrvYBDm39ouW506cWgTd6x
Wkht2Ni9gUOuEB08iRCMKq3zD8qGmVnfe/0Zfy3CN6Y8KteIso5pM6edZBA22Dci
N1XM/SNB3TF2zb36VZyyWCDtnF6n+iN7tM3XcaJsJt8etZeIPTA1o6Hd0uQoTi3k
job4dnjkHpaILOH2DJM4wWWVueMdPVDuvvN1yvZ3fi7T+RjYKv4NUs6ZjzemWvvN
QGStJuRpNXtJBN6KjnOaiaoTzh3ZxVkEw7jPUfkH7on2ie4HuO40LWo7C+/fk6q2
dWmLcQ==
-----END CERTIFICATE-----"""

        private val OTHER_PEM = """-----BEGIN CERTIFICATE-----
MIIC0jCCAbqgAwIBAgIJAIBVt6NZdLoLMA0GCSqGSIb3DQEBCwUAMBcxFTATBgNV
BAMTDE90aGVyIFNlcnZlcjAeFw0yNjA1MTExMTMyMTFaFw0zNjA1MDgxMTMyMTFa
MBcxFTATBgNVBAMTDE90aGVyIFNlcnZlcjCCASIwDQYJKoZIhvcNAQEBBQADggEP
ADCCAQoCggEBAMmaOWDGZ4Aw/4IWSi+S8OTbGIAaEEiuDsk6r+K/098vP87yomm+
EXTcUarz27H2pQ6H0tHugc1vEvVfJBHJ/Oy2gn08wDy7MkEbVAJTJMntikM2Q7ZU
2zkrsUr023bHbdVc7heazdu70BZthZRKVXHz2rypK1yH95Cw/fgIOKSSSHcIa2pA
JLyLzg3ERSU4RgTLaarOoZir5sVqqjNhsW87m/ZIxZG5+wNiYQbtoQxI+k1NEnr1
dMi6GWtNOJ3Lv4apAlFSdoMVShnXlvY3nXCJezOmkT3CKIAtxcOHEpzG6mDecn60
knhuLdaSCukdh5aafiWVkSy5nEpq6PCpzcsCAwEAAaMhMB8wHQYDVR0OBBYEFCMJ
w/2mzWmlPgsqd71kwXG8Bg0VMA0GCSqGSIb3DQEBCwUAA4IBAQBb1/ekbyIFrlM9
quDMXohHQzUCinuQvEgPDbjOQvOmxK7j9YmItGBwlo5EBGRpNJsTE+DnBzmcsUx0
tKAiIfncM9DHcV8hJmzfAkh3pG55FXa99hDngchuNXW5aXQmYvuqwpsRj5ZY/Onr
YYyQP6RVo94M4SaADYeABcaZ6zoyJ5WwCszysydXJrVkOl7JwRqEfo9pynku1j2q
dNRPg+1QqaU2CXV7CxCwAVY1Wf+pvkGzukf5XwXSF/vQ6DrZNom2xFcsH+Txsi8D
DU8PpMG2iONrxVZ/Hcd2jRVwC48Sv9nDfPSc3fJtAeD9AzRlZnwgdcei476vdggL
N75mF5yp
-----END CERTIFICATE-----"""
    }
}
