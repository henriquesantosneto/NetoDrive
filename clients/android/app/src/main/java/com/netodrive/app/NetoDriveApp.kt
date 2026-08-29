package com.netodrive.app

import android.app.Application
import androidx.work.ExistingPeriodicWorkPolicy
import androidx.work.PeriodicWorkRequestBuilder
import androidx.work.WorkManager
import com.netodrive.app.sync.GallerySyncWorker
import java.util.concurrent.TimeUnit

class NetoDriveApp : Application() {
    override fun onCreate() {
        super.onCreate()
        val request = PeriodicWorkRequestBuilder<GallerySyncWorker>(6, TimeUnit.HOURS).build()
        WorkManager.getInstance(this).enqueueUniquePeriodicWork(
            "gallery-sync",
            ExistingPeriodicWorkPolicy.KEEP,
            request,
        )
    }
}
