package com.netodrive.app.api

import com.squareup.moshi.Json
import com.squareup.moshi.Moshi
import com.squareup.moshi.kotlin.reflect.KotlinJsonAdapterFactory
import okhttp3.MediaType.Companion.toMediaType
import okhttp3.OkHttpClient
import okhttp3.Request
import okhttp3.RequestBody.Companion.asRequestBody
import okhttp3.RequestBody.Companion.toRequestBody
import java.io.File
import java.io.IOException
import java.util.concurrent.TimeUnit

data class LoginResponse(
    val token: String,
    @Json(name = "user_id") val userId: Long,
    val username: String,
)

data class FileMeta(
    val id: Long,
    val path: String,
    val name: String,
    @Json(name = "is_dir") val isDir: Boolean = false,
    val size: Long = 0,
    val hash: String = "",
    val mime: String = "application/octet-stream",
    @Json(name = "gallery_key") val galleryKey: String? = null,
    val width: Int = 0,
    val height: Int = 0,
)

data class FilesResponse(val path: String = "", val files: List<FileMeta> = emptyList())

data class GalleryResponse(val items: List<FileMeta> = emptyList())

class NetoDriveApi(
    private val baseUrl: String,
    private var token: String,
    private val deviceId: String,
) {
    private val client = OkHttpClient.Builder()
        .connectTimeout(30, TimeUnit.SECONDS)
        .readTimeout(0, TimeUnit.MILLISECONDS)
        .writeTimeout(0, TimeUnit.MILLISECONDS)
        .build()

    private val moshi = Moshi.Builder().add(KotlinJsonAdapterFactory()).build()

    fun login(username: String, password: String): LoginResponse {
        val body = """{"username":"$username","password":"$password"}"""
            .toRequestBody("application/json".toMediaType())
        val req = Request.Builder()
            .url("${baseUrl.trimEnd('/')}/api/auth/login")
            .post(body)
            .build()
        client.newCall(req).execute().use { res ->
            if (!res.isSuccessful) throw IOException("login failed: ${res.code}")
            val json = res.body?.string() ?: throw IOException("empty body")
            val parsed = moshi.adapter(LoginResponse::class.java).fromJson(json)
                ?: throw IOException("bad login json")
            token = parsed.token
            return parsed
        }
    }

    fun listFiles(path: String): List<FileMeta> {
        val q = java.net.URLEncoder.encode(path, "UTF-8")
        val req = authBuilder("/api/files?path=$q").get().build()
        client.newCall(req).execute().use { res ->
            if (!res.isSuccessful) throw IOException("list failed: ${res.code}")
            val json = res.body?.string() ?: "{}"
            return moshi.adapter(FilesResponse::class.java).fromJson(json)?.files.orEmpty()
        }
    }

    fun gallery(limit: Int = 100, offset: Int = 0): List<FileMeta> {
        val req = authBuilder("/api/gallery?limit=$limit&offset=$offset").get().build()
        client.newCall(req).execute().use { res ->
            if (!res.isSuccessful) throw IOException("gallery failed: ${res.code}")
            val json = res.body?.string() ?: "{}"
            return moshi.adapter(GalleryResponse::class.java).fromJson(json)?.items.orEmpty()
        }
    }

    fun uploadGalleryItem(
        localFile: File,
        album: String,
        remoteName: String,
        galleryKey: String,
        mime: String,
        width: Int,
        height: Int,
        takenAtIso: String?,
    ): FileMeta {
        val safeAlbum = album.trim().ifBlank { "Camera" }
        val body = localFile.asRequestBody(mime.toMediaType())
        val builder = authBuilder("/api/gallery/sync")
            .put(body)
            .header("X-Device-Id", deviceId)
            .header("X-Gallery-Key", galleryKey)
            .header("X-Gallery-Album", safeAlbum)
            .header("X-File-Path", "Galeria/$safeAlbum/$remoteName")
            .header("X-File-Mime", mime)
            .header("X-Width", width.toString())
            .header("X-Height", height.toString())
            .header("X-File-Mtime", java.time.Instant.ofEpochMilli(localFile.lastModified()).toString())
        if (!takenAtIso.isNullOrBlank()) {
            builder.header("X-Taken-At", takenAtIso)
        }
        client.newCall(builder.build()).execute().use { res ->
            if (!res.isSuccessful) throw IOException("upload failed: ${res.code} ${res.body?.string()}")
            val json = res.body?.string() ?: throw IOException("empty upload body")
            return moshi.adapter(FileMeta::class.java).fromJson(json)
                ?: throw IOException("bad upload json")
        }
    }

    fun openUrl(path: String): String {
        val encoded = path.split("/").filter { it.isNotEmpty() }.joinToString("/") {
            java.net.URLEncoder.encode(it, "UTF-8").replace("+", "%20")
        }
        return "${baseUrl.trimEnd('/')}/api/open/$encoded?token=${java.net.URLEncoder.encode(token, "UTF-8")}"
    }

    fun downloadTo(path: String, dest: File) {
        val encoded = path.split("/").filter { it.isNotEmpty() }.joinToString("/") {
            java.net.URLEncoder.encode(it, "UTF-8").replace("+", "%20")
        }
        val req = authBuilder("/api/sync/download/$encoded").get().build()
        client.newCall(req).execute().use { res ->
            if (!res.isSuccessful) throw IOException("download failed: ${res.code}")
            dest.parentFile?.mkdirs()
            res.body?.byteStream()?.use { input ->
                dest.outputStream().use { output -> input.copyTo(output) }
            } ?: throw IOException("empty download")
        }
    }

    private fun authBuilder(path: String): Request.Builder =
        Request.Builder()
            .url("${baseUrl.trimEnd('/')}$path")
            .header("Authorization", "Bearer $token")
}
