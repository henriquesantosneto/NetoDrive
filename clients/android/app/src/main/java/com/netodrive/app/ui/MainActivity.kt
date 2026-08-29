package com.netodrive.app.ui

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import androidx.lifecycle.lifecycleScope
import androidx.recyclerview.widget.LinearLayoutManager
import com.netodrive.app.api.FileMeta
import com.netodrive.app.api.NetoDriveApi
import com.netodrive.app.cache.MediaCache
import com.netodrive.app.data.SessionStore
import com.netodrive.app.databinding.ActivityMainBinding
import com.netodrive.app.remote.RemoteFileProvider
import com.netodrive.app.sync.GallerySyncService
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

class MainActivity : AppCompatActivity() {
    private lateinit var binding: ActivityMainBinding
    private lateinit var session: SessionStore
    private lateinit var cache: MediaCache
    private lateinit var adapter: GalleryAdapter

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        session = SessionStore(this)
        if (!session.isLoggedIn()) {
            startActivity(Intent(this, LoginActivity::class.java))
            finish()
            return
        }
        binding = ActivityMainBinding.inflate(layoutInflater)
        setContentView(binding.root)

        cache = MediaCache(this, session.cacheBudgetBytes)
        binding.cacheModeSwitch.isChecked = session.cacheMode
        binding.cacheBudget.setText((session.cacheBudgetBytes / (1024 * 1024)).toString())

        adapter = GalleryAdapter(
            onOpen = { item -> openRemote(item) },
            thumbLoader = { item, imageView ->
                lifecycleScope.launch {
                    val bmp = withContext(Dispatchers.IO) {
                        loadThumb(item)
                    }
                    imageView.setImageBitmap(bmp)
                }
            },
        )
        binding.galleryList.layoutManager = LinearLayoutManager(this)
        binding.galleryList.adapter = adapter

        binding.cacheModeSwitch.setOnCheckedChangeListener { _, checked ->
            session.cacheMode = checked
            updateStatus()
        }
        binding.refreshBtn.setOnClickListener { refreshGallery() }
        binding.syncGalleryBtn.setOnClickListener {
            ensureMediaPermission {
                lifecycleScope.launch {
                    binding.statusText.text = "Sincronizando galeria…"
                    val result = withContext(Dispatchers.IO) {
                        try {
                            GallerySyncService(this@MainActivity).sync()
                        } catch (e: Exception) {
                            com.netodrive.app.sync.SyncResult(0, 0, e.message)
                        }
                    }
                    if (result.error != null) {
                        Toast.makeText(this@MainActivity, result.error, Toast.LENGTH_LONG).show()
                    } else {
                        Toast.makeText(
                            this@MainActivity,
                            "Enviados ${result.uploaded}, já no servidor ${result.skipped}",
                            Toast.LENGTH_LONG,
                        ).show()
                    }
                    refreshGallery()
                }
            }
        }

        updateStatus()
        refreshGallery()
    }

    private fun updateStatus() {
        val usedMb = cache.usageBytes() / (1024 * 1024)
        val budgetMb = session.cacheBudgetBytes / (1024 * 1024)
        val mode = if (session.cacheMode) "cache" else "fixado"
        binding.statusText.text = "Modo $mode · cache ${usedMb}/${budgetMb} MB · ${session.serverUrl}"
    }

    private fun refreshGallery() {
        val budgetMb = binding.cacheBudget.text?.toString()?.toLongOrNull() ?: 512
        session.cacheBudgetBytes = budgetMb * 1024 * 1024
        cache.setBudget(session.cacheBudgetBytes)

        lifecycleScope.launch {
            try {
                val items = withContext(Dispatchers.IO) {
                    NetoDriveApi(session.serverUrl, session.token, session.deviceId).gallery()
                }
                adapter.submit(items)
                updateStatus()
            } catch (e: Exception) {
                Toast.makeText(this@MainActivity, e.message, Toast.LENGTH_LONG).show()
            }
        }
    }

    private fun openRemote(item: FileMeta) {
        val uri = RemoteFileProvider.buildUri(
            authority = "$packageName.remote",
            path = item.path,
            hash = item.hash.ifBlank { item.path },
            name = item.name,
            mime = item.mime,
        )
        val intent = Intent(this, RemoteOpenActivity::class.java).apply {
            putExtra("path", item.path)
            putExtra("hash", item.hash)
            putExtra("name", item.name)
            putExtra("mime", item.mime)
            putExtra("content_uri", uri.toString())
        }
        startActivity(intent)
    }

    private fun loadThumb(item: FileMeta): android.graphics.Bitmap? {
        return try {
            val api = NetoDriveApi(session.serverUrl, session.token, session.deviceId)
            val file = if (session.cacheMode) {
                cache.getIfPresent(item.path, item.hash) ?: cache.putFromDownload(item.path, item.hash) { dest ->
                    api.downloadTo(item.path, dest)
                }
            } else {
                cache.putFromDownload(item.path, item.hash) { dest -> api.downloadTo(item.path, dest) }
            }
            android.graphics.BitmapFactory.decodeFile(file.absolutePath)
        } catch (_: Exception) {
            null
        }
    }

    private fun ensureMediaPermission(then: () -> Unit) {
        val permission = if (Build.VERSION.SDK_INT >= 33) {
            Manifest.permission.READ_MEDIA_IMAGES
        } else {
            Manifest.permission.READ_EXTERNAL_STORAGE
        }
        if (ContextCompat.checkSelfPermission(this, permission) == PackageManager.PERMISSION_GRANTED) {
            then()
        } else {
            ActivityCompat.requestPermissions(this, arrayOf(permission), 1001)
            pendingPermissionAction = then
        }
    }

    private var pendingPermissionAction: (() -> Unit)? = null

    override fun onRequestPermissionsResult(
        requestCode: Int,
        permissions: Array<out String>,
        grantResults: IntArray,
    ) {
        super.onRequestPermissionsResult(requestCode, permissions, grantResults)
        if (requestCode == 1001 && grantResults.firstOrNull() == PackageManager.PERMISSION_GRANTED) {
            pendingPermissionAction?.invoke()
        } else if (requestCode == 1001) {
            Toast.makeText(this, "Permissão da galeria necessária", Toast.LENGTH_LONG).show()
        }
        pendingPermissionAction = null
    }
}
