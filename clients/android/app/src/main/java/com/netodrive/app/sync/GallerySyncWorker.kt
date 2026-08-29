package com.netodrive.app.sync

import android.content.Context
import androidx.work.CoroutineWorker
import androidx.work.WorkerParameters
import com.netodrive.app.data.SessionStore

class GallerySyncWorker(
    appContext: Context,
    params: WorkerParameters,
) : CoroutineWorker(appContext, params) {
    override suspend fun doWork(): Result {
        val session = SessionStore(applicationContext)
        if (!session.isLoggedIn()) return Result.success()
        return try {
            GallerySyncService(applicationContext).sync()
            Result.success()
        } catch (_: Exception) {
            Result.retry()
        }
    }
}
