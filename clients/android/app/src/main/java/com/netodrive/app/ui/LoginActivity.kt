package com.netodrive.app.ui

import android.content.Intent
import android.os.Bundle
import android.widget.Toast
import androidx.appcompat.app.AppCompatActivity
import androidx.lifecycle.lifecycleScope
import com.netodrive.app.api.NetoDriveApi
import com.netodrive.app.data.SessionStore
import com.netodrive.app.databinding.ActivityLoginBinding
import kotlinx.coroutines.Dispatchers
import kotlinx.coroutines.launch
import kotlinx.coroutines.withContext

class LoginActivity : AppCompatActivity() {
    private lateinit var binding: ActivityLoginBinding
    private lateinit var session: SessionStore

    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)
        session = SessionStore(this)
        if (session.isLoggedIn()) {
            startActivity(Intent(this, MainActivity::class.java))
            finish()
            return
        }
        binding = ActivityLoginBinding.inflate(layoutInflater)
        setContentView(binding.root)

        if (session.serverUrl.isNotBlank()) binding.serverUrl.setText(session.serverUrl)
        if (session.username.isNotBlank()) binding.username.setText(session.username)

        binding.loginBtn.setOnClickListener {
            val url = binding.serverUrl.text?.toString()?.trim().orEmpty()
            val user = binding.username.text?.toString()?.trim().orEmpty()
            val pass = binding.password.text?.toString().orEmpty()
            if (url.isEmpty() || user.isEmpty()) {
                binding.errorText.text = "Preencha servidor e usuário"
                return@setOnClickListener
            }
            binding.loginBtn.isEnabled = false
            lifecycleScope.launch {
                try {
                    val api = NetoDriveApi(url, "", session.deviceId)
                    val res = withContext(Dispatchers.IO) { api.login(user, pass) }
                    session.serverUrl = url
                    session.username = user
                    session.token = res.token
                    startActivity(Intent(this@LoginActivity, MainActivity::class.java))
                    finish()
                } catch (e: Exception) {
                    binding.errorText.text = e.message ?: "Falha no login"
                    Toast.makeText(this@LoginActivity, "Login falhou", Toast.LENGTH_SHORT).show()
                } finally {
                    binding.loginBtn.isEnabled = true
                }
            }
        }
    }
}
