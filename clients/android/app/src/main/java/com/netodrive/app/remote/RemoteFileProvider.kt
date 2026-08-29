package com.netodrive.app.remote

import android.content.ContentProvider
import android.content.ContentValues
import android.database.Cursor
import android.database.MatrixCursor
import android.net.Uri
import android.os.ParcelFileDescriptor
import android.provider.OpenableColumns
import com.netodrive.app.api.NetoDriveApi
import com.netodrive.app.cache.MediaCache
import com.netodrive.app.data.SessionStore
import java.io.File

/**
 * Exposes cached/remote files as content:// URIs so other apps can open them.
 * In cache mode the blob is fetched on first open and may later be evicted.
 */
class RemoteFileProvider : ContentProvider() {
    override fun onCreate(): Boolean = true

    override fun openFile(uri: Uri, mode: String): ParcelFileDescriptor? {
        val ctx = context ?: return null
        val session = SessionStore(ctx)
        val path = uri.getQueryParameter("path") ?: return null
        val hash = uri.getQueryParameter("hash") ?: path
        val cache = MediaCache(ctx, session.cacheBudgetBytes)
        val api = NetoDriveApi(session.serverUrl, session.token, session.deviceId)

        val file: File = if (session.cacheMode) {
            cache.getIfPresent(path, hash) ?: cache.putFromDownload(path, hash) { dest ->
                api.downloadTo(path, dest)
            }
        } else {
            // Pin: keep under filesDir so it is not budget-trimmed the same way
            val pinned = File(ctx.filesDir, "pinned/${hash.take(2)}/$hash")
            if (!pinned.exists()) {
                api.downloadTo(path, pinned)
            }
            pinned
        }
        return ParcelFileDescriptor.open(file, ParcelFileDescriptor.MODE_READ_ONLY)
    }

    override fun query(
        uri: Uri,
        projection: Array<out String>?,
        selection: String?,
        selectionArgs: Array<out String>?,
        sortOrder: String?,
    ): Cursor {
        val name = uri.getQueryParameter("name") ?: "remote"
        val cols = arrayOf(OpenableColumns.DISPLAY_NAME, OpenableColumns.SIZE)
        val cursor = MatrixCursor(cols)
        cursor.addRow(arrayOf(name, 0L))
        return cursor
    }

    override fun getType(uri: Uri): String =
        uri.getQueryParameter("mime") ?: "application/octet-stream"

    override fun insert(uri: Uri, values: ContentValues?): Uri? = null
    override fun delete(uri: Uri, selection: String?, selectionArgs: Array<out String>?): Int = 0
    override fun update(
        uri: Uri,
        values: ContentValues?,
        selection: String?,
        selectionArgs: Array<out String>?,
    ): Int = 0

    companion object {
        fun buildUri(authority: String, path: String, hash: String, name: String, mime: String): Uri =
            Uri.parse("content://$authority/file").buildUpon()
                .appendQueryParameter("path", path)
                .appendQueryParameter("hash", hash)
                .appendQueryParameter("name", name)
                .appendQueryParameter("mime", mime)
                .build()
    }
}
