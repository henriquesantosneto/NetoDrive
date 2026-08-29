package com.netodrive.app.ui

import android.content.Intent
import android.net.Uri
import android.os.Bundle
import android.view.View
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import androidx.lifecycle.lifecycleScope
import com.netodrive.app.api.NetoDriveApi
import com.netodrive.app.cache.MediaCache
import com.netodrive.app.data.SessionStore
import com.netodrive.app.databinding.ActivityRemoteOpenBinding
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

class RemoteOpenActivity : AppCompatActivity() {
    private lateinit var binding: ActivityRemoteOpenBinding

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        binding = ActivityRemoteOpenBinding.inflate(layoutInflater)
        setContentView(binding.root)

        val path = intent.getStringExtra("path") ?: return finish()
        val hash = intent.getStringExtra("hash") ?: path
        val name = intent.getStringExtra("name") ?: "arquivo"
        val mime = intent.getStringExtra("mime") ?: "application/octet-stream"
        title = name

        val session = SessionStore(this)
        val cache = MediaCache(this, session.cacheBudgetBytes)
        val api = NetoDriveApi(session.serverUrl, session.token, session.deviceId)

        lifecycleScope.launch {
            try {
                val file = withContext(Dispatchers.IO) {
                    cache.putFromDownload(path, hash) { dest -> api.downloadTo(path, dest) }
                }
                binding.progress.visibility = View.GONE
                if (mime.startsWith("image/")) {
                    binding.imageView.setImageURI(Uri.fromFile(file))
                } else {
                    // Open with external app via content provider / chooser
                    val contentUri = Uri.parse(intent.getStringExtra("content_uri"))
                    val view = Intent(Intent.ACTION_VIEW).apply {
                        setDataAndType(contentUri, mime)
                        addFlags(Intent.FLAG_GRANT_READ_URI_PERMISSION)
                    }
                    startActivity(Intent.createChooser(view, "Abrir com"))
                    finish()
                }
            } catch (e: Exception) {
                binding.progress.visibility = View.GONE
                Toast.makeText(this@RemoteOpenActivity, e.message, Toast.LENGTH_LONG).show()
                finish()
            }
        }
    }
}
