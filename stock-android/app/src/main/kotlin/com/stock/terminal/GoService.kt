package com.stock.terminal

import android.content.Context
import android.util.Log
import java.io.File
import java.io.FileOutputStream

class GoService(private val context: Context) {

    private var process: Process? = null
    private val BINARY_NAME = "stock-terminal"
    private val PORT = "8081"

    fun start() {
        if (process != null) return

        Thread {
            try {
                val binary = extractBinary()
                val filesDir = context.filesDir

                // 设置工作目录为 app 私有目录，数据库/日志/导出都在此
                val pb = ProcessBuilder(binary.absolutePath)
                pb.directory(filesDir)
                pb.environment()["PORT"] = PORT
                pb.environment()["FORCE_MOBILE"] = "1"
                pb.redirectErrorStream(true)

                process = pb.start()

                // 丢弃 stdout 输出（Go server 日志写入文件）
                process!!.inputStream.buffered().use { it.readBytes() }

                Log.i("GoService", "Go process exited with code: ${process!!.waitFor()}")
            } catch (e: Exception) {
                Log.e("GoService", "Failed to start Go process", e)
            } finally {
                process = null
            }
        }.start()
    }

    fun stop() {
        process?.let {
            try {
                it.destroy()
                Log.i("GoService", "Go process stopped")
            } catch (e: Exception) {
                Log.e("GoService", "Error stopping Go process", e)
            }
        }
        process = null
    }

    fun isRunning(): Boolean = process != null

    private fun extractBinary(): File {
        val outFile = File(context.filesDir, BINARY_NAME)

        // 每次启动都重新提取，确保版本一致
        context.assets.open(BINARY_NAME).use { input ->
            FileOutputStream(outFile).use { output ->
                input.copyTo(output)
            }
        }
        outFile.setExecutable(true)

        return outFile
    }
}
