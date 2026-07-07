package com.bedrud.app.core.ssl

import org.junit.Assert.*
import org.junit.Before
import org.junit.Test
import java.io.File
import java.security.cert.X509Certificate

class CertificateStoreTest {

    private lateinit var certDir: File
    private lateinit var store: CertificateStore
    private lateinit var testCert: X509Certificate

    @Before
    fun setUp() {
        certDir = createTempDir()
        store = CertificateStore(certDir)
        testCert = CertificateStore.decodeFromPem(TEST_PEM)!!
    }

    @Test
    fun `encodeToPem wraps certificate in PEM headers`() {
        val pem = CertificateStore.encodeToPem(testCert)
        assertTrue(pem, pem.startsWith("-----BEGIN CERTIFICATE-----"))
        assertTrue(pem, pem.trimEnd().endsWith("-----END CERTIFICATE-----"))
    }

    @Test
    fun `decodeFromPem recovers original certificate`() {
        val pem = CertificateStore.encodeToPem(testCert)
        val decoded = CertificateStore.decodeFromPem(pem)
        assertNotNull(decoded)
        assertEquals("CN=Test Server", decoded!!.subjectX500Principal.name)
    }

    @Test
    fun `decodeFromPem returns null for garbage input`() {
        assertNull(CertificateStore.decodeFromPem("not a certificate"))
        assertNull(CertificateStore.decodeFromPem(""))
    }

    @Test
    fun `decodeFromPem returns null for invalid base64 content`() {
        assertNull(CertificateStore.decodeFromPem(
            "-----BEGIN CERTIFICATE-----\n!!!not-base64!!!\n-----END CERTIFICATE-----"
        ))
    }

    @Test
    fun `save and load certificate round-trips`() {
        store.saveCertificate("inst-1", testCert)

        assertTrue(store.hasCertificate("inst-1"))

        val loaded = store.getCertificate("inst-1")
        assertNotNull(loaded)
        assertEquals("CN=Test Server", loaded!!.subjectX500Principal.name)
        assertEquals(testCert.serialNumber, loaded.serialNumber)
    }

    @Test
    fun `getCertificate returns null for missing instance`() {
        assertNull(store.getCertificate("nonexistent"))
    }

    @Test
    fun `hasCertificate returns false for missing instance`() {
        assertFalse(store.hasCertificate("nonexistent"))
    }

    @Test
    fun `removeCertificate deletes stored file`() {
        store.saveCertificate("inst-1", testCert)
        assertTrue(store.hasCertificate("inst-1"))

        store.removeCertificate("inst-1")
        assertFalse(store.hasCertificate("inst-1"))
        assertFalse(File(certDir, "inst-1.crt").exists())
    }

    @Test
    fun `removeAll clears all certificates`() {
        store.saveCertificate("i1", testCert)
        store.saveCertificate("i2", testCert)

        store.removeAll()
        assertFalse(store.hasCertificate("i1"))
        assertFalse(store.hasCertificate("i2"))
        assertEquals(0, certDir.listFiles()?.size ?: 0)
    }

    @Test
    fun `multiple instances have independent storage`() {
        store.saveCertificate("i1", testCert)
        store.saveCertificate("i2", testCert)

        store.removeCertificate("i1")
        assertFalse(store.hasCertificate("i1"))
        assertTrue(store.hasCertificate("i2"))
    }

    @Test
    fun `init creates certs directory when missing`() {
        certDir.deleteRecursively()
        assertFalse(certDir.exists())

        CertificateStore(certDir)
        assertTrue(certDir.exists())
    }

    companion object {
        private fun createTempDir(): File {
            val dir = File(System.getProperty("java.io.tmpdir"), "bedrud-cert-test-${System.nanoTime()}")
            dir.mkdirs()
            dir.deleteOnExit()
            return dir
        }

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
    }
}
