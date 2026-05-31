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

        // 设置进程退出回调：立即检测崩溃，不等待超时
        goService.onProcessExited = { exitCode, errorOutput ->
            val diag = buildString {
                appendLine("引擎进程已退出 (exit=$exitCode)")
                if (errorOutput.isNotEmpty()) {
                    appendLine("--- 错误日志 ---")
                    // 只取最后 500 字符避免 UI 溢出
                    val tail = if (errorOutput.length > 500) errorOutput.takeLast(500) else errorOutput
                    append(tail)
                } else {
                    append("无错误输出，请检查 logcat [GoService] 标签")
                }
            }
            handler.post {
                statusText.text = diag
                statusText.textSize = 12f
                progressBar.visibility = View.GONE
            }
        }

        // 设置二进制文件提取失败回调
        goService.onExtractionFailed = { reason ->
            handler.post {
                statusText.text = "启动失败: $reason"
                statusText.textSize = 14f
                progressBar.visibility = View.GONE
            }
        }

        goService.start()

        // 等待 Go 服务就绪后加载页面
        waitForServer()
    }

    private fun waitForServer() {
        Thread {
            var attempts = 0
            val maxAttempts = 30 // 最多等 15 秒
            while (attempts < maxAttempts) {
                // 检查进程是否已退出
                if (goService.getExitCode() != null) {
                    val exitCode = goService.getExitCode()
                    val stderr = goService.getErrorOutput()
                    val diag = buildString {
                        appendLine("引擎进程已退出 (exit=$exitCode)")
                        if (stderr.isNotEmpty()) {
                            appendLine("--- 错误日志 ---")
                            // 只取最后 500 字符避免 UI 溢出
                            val tail = if (stderr.length > 500) stderr.takeLast(500) else stderr
                            append(tail)
                        } else {
                            append("无错误输出，请检查 logcat [GoService] 标签")
                        }
                    }
                    handler.post {
                        statusText.text = diag
                        statusText.textSize = 12f
                        progressBar.visibility = View.GONE
                    }
                    return@Thread
                }

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
            // 超时：显示诊断信息
            val stderr = goService.getErrorOutput()
            val diag = buildString {
                appendLine("引擎启动超时 (15s)")
                if (goService.isRunning()) {
                    appendLine("进程仍在运行，可能启动缓慢或端口未监听")
                } else if (goService.getExitCode() != null) {
                    appendLine("进程已退出 (exit=${goService.getExitCode()})")
                }
                if (stderr.isNotEmpty()) {
                    appendLine("--- 错误日志 ---")
                    val tail = if (stderr.length > 500) stderr.takeLast(500) else stderr
                    append(tail)
                }
            }
            handler.post {
                statusText.text = diag
                statusText.textSize = 12f
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
