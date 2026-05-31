package com.stock.terminal

import android.annotation.SuppressLint
import android.os.Bundle
import android.os.Handler
import android.os.Looper
import android.view.View
import android.webkit.*
import android.widget.ProgressBar
import android.widget.TextView
import androidx.appcompat.app.AppCompatActivity

class MainActivity : AppCompatActivity() {

    private lateinit var webView: WebView
    private lateinit var progressBar: ProgressBar
    private lateinit var statusText: TextView
    private lateinit var goService: GoService
    private val handler = Handler(Looper.getMainLooper())
    private val PORT = "8081"

    @SuppressLint("SetJavaScriptEnabled")
    override fun onCreate(savedInstanceState: Bundle?) {
        super.onCreate(savedInstanceState)

        // 简单布局，不用 XML
        val layout = android.widget.LinearLayout(this).apply {
            orientation = android.widget.LinearLayout.VERTICAL
            setBackgroundColor(0xFF1A1A2E.toInt())
        }

        statusText = TextView(this).apply {
            text = "正在启动量化引擎..."
            setTextColor(0xFF00D2FF.toInt())
            textSize = 16f
            setPadding(32, 48, 32, 24)
        }
        layout.addView(statusText)

        progressBar = ProgressBar(this).apply {
            visibility = View.VISIBLE
        }
        layout.addView(progressBar)

        webView = WebView(this).apply {
            visibility = View.GONE
            layoutParams = android.widget.LinearLayout.LayoutParams(
                android.widget.LinearLayout.LayoutParams.MATCH_PARENT,
                android.widget.LinearLayout.LayoutParams.MATCH_PARENT
            )
        }
        layout.addView(webView)

        setContentView(layout)

        // 配置 WebView
        webView.settings.apply {
            javaScriptEnabled = true
            domStorageEnabled = true
            mixedContentMode = WebSettings.MIXED_CONTENT_ALWAYS_ALLOW
            cacheMode = WebSettings.LOAD_DEFAULT
        }

        webView.webViewClient = object : WebViewClient() {
            override fun onPageFinished(view: WebView?, url: String?) {
                super.onPageFinished(view, url)
                webView.visibility = View.VISIBLE
                progressBar.visibility = View.GONE
                statusText.visibility = View.GONE
            }

            override fun onReceivedError(view: WebView?, request: WebResourceRequest?, error: WebResourceError?) {
                // 页面加载失败时重试
                if (request?.isForMainFrame == true) {
                    handler.postDelayed({ view?.loadUrl("http://localhost:$PORT") }, 2000)
                }
            }
        }

        // 启动 Go 服务
        goService = GoService(this)
        goService.start()

        // 等待 Go 服务就绪后加载页面
        waitForServer()
    }

    private fun waitForServer() {
        Thread {
            var attempts = 0
            val maxAttempts = 30 // 最多等 15 秒
            while (attempts < maxAttempts) {
                try {
                    val conn = java.net.Socket()
                    conn.connect(java.net.InetSocketAddress("localhost", PORT.toInt()), 1000)
                    conn.close()
                    // 服务就绪
                    handler.post {
                        webView.loadUrl("http://localhost:$PORT")
                    }
                    return@Thread
                } catch (_: Exception) {
                    attempts++
                    Thread.sleep(500)
                }
            }
            // 超时
            handler.post {
                statusText.text = "引擎启动超时，请重启应用"
                progressBar.visibility = View.GONE
            }
        }.start()
    }

    override fun onBackPressed() {
        if (webView.canGoBack()) {
            webView.goBack()
        } else {
            super.onBackPressed()
        }
    }

    override fun onDestroy() {
        goService.stop()
        webView.destroy()
        super.onDestroy()
    }
}
