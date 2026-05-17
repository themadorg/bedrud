package com.bedrud.app.core.instance

import android.app.Application
import com.bedrud.app.core.api.AdminApi
import com.bedrud.app.core.api.ApiClientFactory
import com.bedrud.app.core.api.AuthApi
import com.bedrud.app.core.api.AuthInterceptor
import com.bedrud.app.core.api.RoomApi
import com.bedrud.app.core.api.TokenAuthenticator
import com.bedrud.app.core.auth.AuthManager
import com.bedrud.app.core.auth.PasskeyManager
import com.bedrud.app.core.livekit.RoomManager
import com.bedrud.app.core.ssl.CertificateInfo
import com.bedrud.app.core.ssl.CertificateManager
import com.bedrud.app.core.ssl.CertificateStore
import com.bedrud.app.models.HealthResponse
import com.bedrud.app.models.Instance
import com.google.gson.GsonBuilder
import kotlinx.coroutines.flow.MutableSharedFlow
import kotlinx.coroutines.flow.MutableStateFlow
import kotlinx.coroutines.flow.SharedFlow
import kotlinx.coroutines.flow.StateFlow
import kotlinx.coroutines.flow.asSharedFlow
import kotlinx.coroutines.flow.asStateFlow
import okhttp3.OkHttpClient
import okhttp3.logging.HttpLoggingInterceptor
import retrofit2.Retrofit
import retrofit2.converter.gson.GsonConverterFactory
import java.util.concurrent.ConcurrentHashMap
import java.util.concurrent.TimeUnit
import javax.net.ssl.SSLSocketFactory
import javax.net.ssl.X509TrustManager

sealed class CheckHealthResult {
    data class Trusted(val health: HealthResponse) : CheckHealthResult()
    data class Captured(val health: HealthResponse, val certInfo: CertificateInfo, val tempCertId: String) : CheckHealthResult()
    data class Error(val message: String) : CheckHealthResult()
}

sealed class RenewalResult {
    data class Captured(val info: CertificateInfo, val tempCertId: String) : RenewalResult()
    data class Error(val message: String) : RenewalResult()
}

class InstanceManager(
    private val application: Application,
    val store: InstanceStore,
    private val certificateStore: CertificateStore
) {
    private val _authManager = MutableStateFlow<AuthManager?>(null)
    val authManager: StateFlow<AuthManager?> = _authManager.asStateFlow()

    private val _authApi = MutableStateFlow<AuthApi?>(null)
    val authApi: StateFlow<AuthApi?> = _authApi.asStateFlow()

    private val _roomApi = MutableStateFlow<RoomApi?>(null)
    val roomApi: StateFlow<RoomApi?> = _roomApi.asStateFlow()

    private val _passkeyManager = MutableStateFlow<PasskeyManager?>(null)
    val passkeyManager: StateFlow<PasskeyManager?> = _passkeyManager.asStateFlow()

    private val _roomManager = MutableStateFlow<RoomManager?>(null)
    val roomManager: StateFlow<RoomManager?> = _roomManager.asStateFlow()

    private val _adminApi = MutableStateFlow<AdminApi?>(null)
    val adminApi: StateFlow<AdminApi?> = _adminApi.asStateFlow()

    private val pendingCerts = ConcurrentHashMap<String, java.security.cert.X509Certificate>()

    private val _certificateNeedRenewal = MutableSharedFlow<Unit>(extraBufferCapacity = 1)
    val certificateNeedRenewal: SharedFlow<Unit> = _certificateNeedRenewal.asSharedFlow()

    init {
        rebuild()
    }

    fun rebuild() {
        val instance = store.activeInstance ?: run {
            _authManager.value = null
            _authApi.value = null
            _roomApi.value = null
            _passkeyManager.value = null
            _roomManager.value = null
            _adminApi.value = null
            return
        }

        val baseURL = instance.apiBaseURL
        val am = AuthManager(application, instance.id)
        val factory = ApiClientFactory(baseURL)

        var sslSocketFactory: SSLSocketFactory? = null
        var x509TrustManager: X509TrustManager? = null
        certificateStore.getCertificate(instance.id)?.let { cert ->
            if (CertificateInfo.fromCertificate(cert).isExpired()) {
                _certificateNeedRenewal.tryEmit(Unit)
            }
            sslSocketFactory = CertificateManager.createPinnedSSLSocketFactory(cert)
            x509TrustManager = CertificateManager.createPinnedTrustManager(cert)
        }

        val interceptor = AuthInterceptor(am)
        val authenticator = TokenAuthenticator(
            am, baseURL,
            { _authApi.value ?: error("AuthApi not yet initialized — token refresh attempted before setup completed") },
            sslSocketFactory,
            x509TrustManager
        )
        val okHttp = factory.createOkHttpClient(interceptor, authenticator, sslSocketFactory, x509TrustManager)
        val retrofit = factory.createRetrofit(okHttp)

        val auth: AuthApi = factory.createApi(retrofit)
        val room: RoomApi = factory.createApi(retrofit)
        val admin: AdminApi = factory.createApi(retrofit)
        val pk = PasskeyManager(application, auth, am)
        val rm = RoomManager(application)

        _authManager.value = am
        _authApi.value = auth
        _roomApi.value = room
        _adminApi.value = admin
        _passkeyManager.value = pk
        _roomManager.value = rm
    }

    fun switchTo(instanceId: String) {
        store.setActive(instanceId)
        rebuild()
    }

    fun removeInstance(id: String) {
        if (store.activeInstanceId.value == id) {
            _authManager.value?.logout()
        }
        store.removeInstance(id)
        certificateStore.removeCertificate(id)
        rebuild()
    }

    suspend fun checkHealth(serverURL: String): HealthResponse {
        val baseURL = if (serverURL.endsWith("/")) "${serverURL}api" else "$serverURL/api"
        val plainClient = OkHttpClient.Builder()
            .connectTimeout(10, TimeUnit.SECONDS)
            .readTimeout(10, TimeUnit.SECONDS)
            .build()
        val gson = GsonBuilder().setLenient().create()
        val retrofit = Retrofit.Builder()
            .baseUrl(baseURL.trimEnd('/') + "/")
            .client(plainClient)
            .addConverterFactory(GsonConverterFactory.create(gson))
            .build()

        val api = retrofit.create(HealthApi::class.java)
        val response = api.health()
        if (response.isSuccessful) {
            return response.body() ?: HealthResponse()
        } else {
            throw Exception("Server returned ${response.code()}")
        }
    }

    suspend fun checkHealthWithCapture(serverURL: String): CheckHealthResult {
        val baseURL = if (serverURL.endsWith("/")) "${serverURL}api" else "$serverURL/api"
        val (capturingSslFactory, capturingTM) = CertificateManager.createCapturingSSLSocketFactory()

        val client = OkHttpClient.Builder()
            .connectTimeout(10, TimeUnit.SECONDS)
            .readTimeout(10, TimeUnit.SECONDS)
            .sslSocketFactory(capturingSslFactory, capturingTM)
            .hostnameVerifier(okhttp3.internal.tls.OkHostnameVerifier)
            .build()

        val gson = GsonBuilder().setLenient().create()
        val retrofit = Retrofit.Builder()
            .baseUrl(baseURL.trimEnd('/') + "/")
            .client(client)
            .addConverterFactory(GsonConverterFactory.create(gson))
            .build()

        val api = retrofit.create(HealthApi::class.java)
        val response = try {
            api.health()
        } catch (e: Exception) {
            return CheckHealthResult.Error("Could not reach server: ${e.message}")
        }

        if (!response.isSuccessful) {
            return CheckHealthResult.Error("Server returned ${response.code()}")
        }

        val health = response.body() ?: HealthResponse()
        val capturedCert = capturingTM.getCapturedCertificate()

        if (capturedCert != null) {
            val certInfo = CertificateInfo.fromCertificate(capturedCert)
            val tempId = java.util.UUID.randomUUID().toString()
            pendingCerts[tempId] = capturedCert
            return CheckHealthResult.Captured(health, certInfo, tempId)
        }

        return CheckHealthResult.Trusted(health)
    }

    fun discardPendingCertificate(tempCertId: String) {
        pendingCerts.remove(tempCertId)
    }

    fun onSslError() {
        _certificateNeedRenewal.tryEmit(Unit)
    }

    suspend fun beginCertificateRenewal(): RenewalResult {
        val instance = store.activeInstance ?: return RenewalResult.Error("No active instance")
        val serverURL = instance.serverURL
        val baseURL = if (serverURL.endsWith("/")) "${serverURL}api" else "$serverURL/api"
        val (capturingSslFactory, capturingTM) = CertificateManager.createCapturingSSLSocketFactory()

        val client = OkHttpClient.Builder()
            .connectTimeout(10, TimeUnit.SECONDS)
            .readTimeout(10, TimeUnit.SECONDS)
            .sslSocketFactory(capturingSslFactory, capturingTM)
            .hostnameVerifier(okhttp3.internal.tls.OkHostnameVerifier)
            .build()

        val gson = GsonBuilder().setLenient().create()
        val retrofit = Retrofit.Builder()
            .baseUrl(baseURL.trimEnd('/') + "/")
            .client(client)
            .addConverterFactory(GsonConverterFactory.create(gson))
            .build()

        val api = retrofit.create(HealthApi::class.java)
        try {
            api.health()
        } catch (_: Exception) {
        }

        val capturedCert = capturingTM.getCapturedCertificate()
            ?: return RenewalResult.Error("Could not capture server certificate")

        val certInfo = CertificateInfo.fromCertificate(capturedCert)
        val tempId = java.util.UUID.randomUUID().toString()
        pendingCerts[tempId] = capturedCert
        return RenewalResult.Captured(certInfo, tempId)
    }

    fun confirmRenewal(tempCertId: String) {
        val instanceId = store.activeInstanceId.value ?: return
        pendingCerts.remove(tempCertId)?.let { cert ->
            certificateStore.saveCertificate(instanceId, cert)
        }
        rebuild()
    }

    fun cancelRenewal(tempCertId: String) {
        pendingCerts.remove(tempCertId)
    }

    suspend fun addInstance(serverURL: String, displayName: String, trustCertId: String? = null) {
        if (trustCertId == null) {
            checkHealth(serverURL)
        }
        val instance = Instance(
            serverURL = serverURL,
            displayName = displayName
        )

        trustCertId?.let { id ->
            pendingCerts.remove(id)?.let { cert ->
                certificateStore.saveCertificate(instance.id, cert)
            }
        }

        store.addInstance(instance)
        store.setActive(instance.id)
        rebuild()
    }
}

interface HealthApi {
    @retrofit2.http.GET("health")
    suspend fun health(): retrofit2.Response<HealthResponse>
}
