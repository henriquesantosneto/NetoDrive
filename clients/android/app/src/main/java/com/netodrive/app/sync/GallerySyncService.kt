package com.netodrive.app.sync

import android.content.ContentUris
import android.content.Context
import android.net.Uri
import android.provider.MediaStore
import com.netodrive.app.api.NetoDriveApi
import com.netodrive.app.data.SessionStore
import java.io.File
import java.time.Instant

data class LocalMedia(
    val galleryKey: String,
    val displayName: String,
    val album: String,
    val mime: String,
    val width: Int,
    val height: Int,
    val takenAtIso: String?,
    val uri: Uri,
    val size: Long,
)

class GalleryScanner(private val context: Context) {
    fun scanImages(limit: Int = 500): List<LocalMedia> {
        val collection = MediaStore.Images.Media.getContentUri(MediaStore.VOLUME_EXTERNAL)
        val projection = arrayOf(
            MediaStore.Images.Media._ID,
            MediaStore.Images.Media.DISPLAY_NAME,
            MediaStore.Images.Media.MIME_TYPE,
            MediaStore.Images.Media.WIDTH,
            MediaStore.Images.Media.HEIGHT,
            MediaStore.Images.Media.DATE_TAKEN,
            MediaStore.Images.Media.SIZE,
            MediaStore.Images.Media.BUCKET_DISPLAY_NAME,
        )
        val sort = "${MediaStore.Images.Media.DATE_TAKEN} DESC"
        val out = mutableListOf<LocalMedia>()
        context.contentResolver.query(collection, projection, null, null, sort)?.use { cursor ->
            val idCol = cursor.getColumnIndexOrThrow(MediaStore.Images.Media._ID)
            val nameCol = cursor.getColumnIndexOrThrow(MediaStore.Images.Media.DISPLAY_NAME)
            val mimeCol = cursor.getColumnIndexOrThrow(MediaStore.Images.Media.MIME_TYPE)
            val wCol = cursor.getColumnIndexOrThrow(MediaStore.Images.Media.WIDTH)
            val hCol = cursor.getColumnIndexOrThrow(MediaStore.Images.Media.HEIGHT)
            val takenCol = cursor.getColumnIndexOrThrow(MediaStore.Images.Media.DATE_TAKEN)
            val sizeCol = cursor.getColumnIndexOrThrow(MediaStore.Images.Media.SIZE)
            val bucketCol = cursor.getColumnIndexOrThrow(MediaStore.Images.Media.BUCKET_DISPLAY_NAME)
            while (cursor.moveToNext() && out.size < limit) {
                val id = cursor.getLong(idCol)
                val taken = cursor.getLong(takenCol)
                val bucket = cursor.getString(bucketCol)?.trim().orEmpty().ifBlank { "Camera" }
                out += LocalMedia(
                    galleryKey = "img-$id",
                    displayName = cursor.getString(nameCol) ?: "image-$id.jpg",
                    album = sanitizeAlbum(bucket),
                    mime = cursor.getString(mimeCol) ?: "image/jpeg",
                    width = cursor.getInt(wCol),
                    height = cursor.getInt(hCol),
                    takenAtIso = if (taken > 0) Instant.ofEpochMilli(taken).toString() else null,
                    uri = ContentUris.withAppendedId(collection, id),
                    size = cursor.getLong(sizeCol),
                )
            }
        }
        return out
    }

    private fun sanitizeAlbum(name: String): String {
        return name.replace(Regex("""[\\/:*?"<>|]"""), "_").trim().ifBlank { "Camera" }
    }
}

class GallerySyncService(private val context: Context) {
    private val session = SessionStore(context)

    fun sync(maxItems: Int = 200): SyncResult {
        if (!session.isLoggedIn()) return SyncResult(0, 0, "not logged in")
        val api = NetoDriveApi(session.serverUrl, session.token, session.deviceId)
        val remoteKeys = api.gallery(limit = 2000).mapNotNull { it.galleryKey }.toHashSet()
        val local = GalleryScanner(context).scanImages(maxItems)
        var uploaded = 0
        var skipped = 0
        val tmpDir = File(context.cacheDir, "gallery-upload").apply { mkdirs() }
        for (item in local) {
            if (remoteKeys.contains(item.galleryKey)) {
                skipped++
                continue
            }
            val tmp = File(tmpDir, item.displayName)
            context.contentResolver.openInputStream(item.uri)?.use { input ->
                tmp.outputStream().use { output -> input.copyTo(output) }
            } ?: continue
            api.uploadGalleryItem(
                localFile = tmp,
                album = item.album,
                remoteName = "${item.galleryKey}-${item.displayName}",
                galleryKey = item.galleryKey,
                mime = item.mime,
                width = item.width,
                height = item.height,
                takenAtIso = item.takenAtIso,
            )
            tmp.delete()
            uploaded++
        }
        return SyncResult(uploaded, skipped, null)
    }
}

data class SyncResult(val uploaded: Int, val skipped: Int, val error: String?)
