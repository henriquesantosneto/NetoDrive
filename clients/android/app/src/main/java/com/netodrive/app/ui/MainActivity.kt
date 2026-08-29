package com.netodrive.app.ui

import android.Manifest
import android.content.Intent
import android.content.pm.PackageManager
import android.os.Build
import android.os.Bundle
import android.view.LayoutInflater
import android.view.View
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import androidx.core.app.ActivityCompat
import androidx.core.content.ContextCompat
import androidx.lifecycle.lifecycleScope
import androidx.recyclerview.widget.LinearLayoutManager
import com.netodrive.app.R
import com.netodrive.app.api.FileMeta
import com.netodrive.app.api.NetoDriveApi
import com.netodrive.app.cache.MediaCache
import com.netodrive.app.data.SessionStore
import com.netodrive.app.databinding.ActivityMainBinding
import com.netodrive.app.databinding.FragmentFilesBinding
import com.netodrive.app.databinding.FragmentGalleryBinding
import com.netodrive.app.databinding.FragmentSettingsBinding
import com.netodrive.app.remote.RemoteFileProvider
import com.netodrive.app.sync.GallerySyncService
import com.netodrive.app.sync.SyncResult
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

class MainActivity : AppCompatActivity() {
    private lateinit var binding: ActivityMainBinding
    private lateinit var session: SessionStore
    private lateinit var cache: MediaCache

    private var filesBinding: FragmentFilesBinding? = null
    private var galleryBinding: FragmentGalleryBinding? = null
    private var settingsBinding: FragmentSettingsBinding? = null

    private var currentPath = ""
    private lateinit var fileAdapter: FileListAdapter
    private lateinit var galleryAdapter: GalleryAdapter
    private var pendingPermissionAction: (() -> Unit)? = null

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

        fileAdapter = FileListAdapter(onClick = { item ->
            if (item.isDir) {
                browseTo(item.path)
            } else {
                openRemote(item)
            }
        })
        galleryAdapter = GalleryAdapter(
            onOpen = { openRemote(it) },
            thumbLoader = { item, imageView ->
                lifecycleScope.launch {
                    val bmp = withContext(Dispatchers.IO) { loadThumb(item) }
                    imageView.setImageBitmap(bmp)
                }
            },
        )

        binding.bottomNav.setOnItemSelectedListener { item ->
            when (item.itemId) {
                R.id.nav_files -> {
                    showFiles(); true
                }
                R.id.nav_gallery -> {
                    showGallery(); true
                }
                R.id.nav_settings -> {
                    showSettings(); true
                }
                else -> false
            }
        }
        binding.bottomNav.selectedItemId = R.id.nav_files
        updateStatus()
    }

    private fun clearContent() {
        binding.content.removeAllViews()
        filesBinding = null
        galleryBinding = null
        settingsBinding = null
    }

    private fun showFiles() {
        clearContent()
        val fb = FragmentFilesBinding.inflate(LayoutInflater.from(this), binding.content, true)
        filesBinding = fb
        fb.fileList.layoutManager = LinearLayoutManager(this)
        fb.fileList.adapter = fileAdapter
        fb.upBtn.setOnClickListener {
            if (currentPath.isEmpty()) return@setOnClickListener
            val parent = currentPath.substringBeforeLast('/', missingDelimiterValue = "")
            browseTo(parent)
        }
        browseTo(currentPath)
    }

    private fun showGallery() {
        clearContent()
        val gb = FragmentGalleryBinding.inflate(LayoutInflater.from(this), binding.content, true)
        galleryBinding = gb
        gb.galleryList.layoutManager = LinearLayoutManager(this)
        gb.galleryList.adapter = galleryAdapter
        gb.syncGalleryBtn.setOnClickListener {
            ensureMediaPermission {
                lifecycleScope.launch {
                    binding.statusText.text = "Sync…"
                    val result = withContext(Dispatchers.IO) {
                        try {
                            GallerySyncService(this@MainActivity).sync()
                        } catch (e: Exception) {
                            SyncResult(0, 0, e.message)
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
                    updateStatus()
                }
            }
        }
        refreshGallery()
    }

    private fun showSettings() {
        clearContent()
        val sb = FragmentSettingsBinding.inflate(LayoutInflater.from(this), binding.content, true)
        settingsBinding = sb
        sb.cacheModeSwitch.isChecked = session.cacheMode
        sb.cacheBudget.setText((session.cacheBudgetBytes / (1024 * 1024)).toString())
        sb.cacheUsage.text = "Uso atual: ${cache.usageBytes() / (1024 * 1024)} MB"
        sb.saveCacheBtn.setOnClickListener {
            session.cacheMode = sb.cacheModeSwitch.isChecked
            val mb = sb.cacheBudget.text?.toString()?.toLongOrNull() ?: 512
            session.cacheBudgetBytes = mb * 1024 * 1024
            cache.setBudget(session.cacheBudgetBytes)
            sb.cacheUsage.text = "Uso atual: ${cache.usageBytes() / (1024 * 1024)} MB"
            Toast.makeText(this, "Cache salvo", Toast.LENGTH_SHORT).show()
            updateStatus()
        }
        sb.logoutBtn.setOnClickListener {
            session.clearAuth()
            startActivity(Intent(this, LoginActivity::class.java))
            finish()
        }
    }

    private fun browseTo(path: String) {
        currentPath = path.trim('/')
        val fb = filesBinding ?: return
        fb.pathText.text = if (currentPath.isEmpty()) "Meus arquivos" else currentPath.substringAfterLast('/')
        fb.crumbText.text = if (currentPath.isEmpty()) "NetoDrive" else "NetoDrive / ${currentPath.replace("/", " / ")}"
        fb.upBtn.visibility = if (currentPath.isEmpty()) View.GONE else View.VISIBLE
        lifecycleScope.launch {
            try {
                val items = withContext(Dispatchers.IO) {
                    NetoDriveApi(session.serverUrl, session.token, session.deviceId).listFiles(currentPath)
                }
                fileAdapter.submit(items)
            } catch (e: Exception) {
                Toast.makeText(this@MainActivity, e.message, Toast.LENGTH_LONG).show()
            }
        }
    }

    private fun refreshGallery() {
        lifecycleScope.launch {
            try {
                val items = withContext(Dispatchers.IO) {
                    NetoDriveApi(session.serverUrl, session.token, session.deviceId).gallery()
                }
                galleryAdapter.submit(items)
            } catch (e: Exception) {
                Toast.makeText(this@MainActivity, e.message, Toast.LENGTH_LONG).show()
            }
        }
    }

    private fun updateStatus() {
        val mode = if (session.cacheMode) "cache" else "fixado"
        binding.statusText.text = mode
    }

    private fun openRemote(item: FileMeta) {
        val uri = RemoteFileProvider.buildUri(
            authority = "$packageName.remote",
            path = item.path,
            hash = item.hash.ifBlank { item.path },
            name = item.name,
            mime = item.mime,
        )
        startActivity(
            Intent(this, RemoteOpenActivity::class.java).apply {
                putExtra("path", item.path)
                putExtra("hash", item.hash)
                putExtra("name", item.name)
                putExtra("mime", item.mime)
                putExtra("content_uri", uri.toString())
            },
        )
    }

    private fun loadThumb(item: FileMeta): android.graphics.Bitmap? {
        return try {
            val api = NetoDriveApi(session.serverUrl, session.token, session.deviceId)
            val file = cache.putFromDownload(item.path, item.hash) { dest ->
                api.downloadTo(item.path, dest)
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
            pendingPermissionAction = then
            ActivityCompat.requestPermissions(this, arrayOf(permission), 1001)
        }
    }

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
