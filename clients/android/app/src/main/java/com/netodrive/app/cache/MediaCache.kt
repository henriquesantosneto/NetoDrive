package com.netodrive.app.cache

import android.content.Context
import java.io.File
import java.security.MessageDigest

/**
 * Disk cache LRU for remote media.
 * In cache mode, files are downloaded on demand and evicted when the budget is exceeded.
 */
class MediaCache(
    context: Context,
    private var budgetBytes: Long,
) {
    private val root = File(context.cacheDir, "netodrive-media").apply { mkdirs() }
    private val lock = Any()

    fun setBudget(bytes: Long) {
        budgetBytes = bytes.coerceAtLeast(16L * 1024L * 1024L)
        trimToBudget()
    }

    fun fileFor(remotePath: String, hash: String): File {
        val key = sha1("$hash|$remotePath")
        return File(root, "${key.take(2)}/$key")
    }

    fun getIfPresent(remotePath: String, hash: String): File? {
        val f = fileFor(remotePath, hash)
        if (!f.exists()) return null
        f.setLastModified(System.currentTimeMillis())
        return f
    }

    fun putFromDownload(remotePath: String, hash: String, downloader: (File) -> Unit): File {
        synchronized(lock) {
            getIfPresent(remotePath, hash)?.let { return it }
            val dest = fileFor(remotePath, hash)
            dest.parentFile?.mkdirs()
            val tmp = File(dest.absolutePath + ".partial")
            downloader(tmp)
            if (dest.exists()) dest.delete()
            if (!tmp.renameTo(dest)) {
                tmp.copyTo(dest, overwrite = true)
                tmp.delete()
            }
            dest.setLastModified(System.currentTimeMillis())
            trimToBudget()
            return dest
        }
    }

    fun release(remotePath: String, hash: String) {
        synchronized(lock) {
            fileFor(remotePath, hash).delete()
        }
    }

    fun trimToBudget() {
        synchronized(lock) {
            val files = root.walkTopDown().filter { it.isFile && !it.name.endsWith(".partial") }.toList()
            var total = files.sumOf { it.length() }
            if (total <= budgetBytes) return
            val ordered = files.sortedBy { it.lastModified() }
            for (f in ordered) {
                if (total <= budgetBytes) break
                val len = f.length()
                if (f.delete()) total -= len
            }
        }
    }

    fun usageBytes(): Long =
        root.walkTopDown().filter { it.isFile }.sumOf { it.length() }

    private fun sha1(s: String): String {
        val dig = MessageDigest.getInstance("SHA-1").digest(s.toByteArray())
        return dig.joinToString("") { "%02x".format(it) }
    }
}
