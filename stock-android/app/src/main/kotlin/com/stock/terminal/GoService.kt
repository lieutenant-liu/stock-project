package com.stock.terminal

import android.content.Context
import android.util.Log
import java.io.BufferedReader
import java.io.File
import java.io.FileOutputStream
import java.io.InputStreamReader

class GoService(private val context: Context) {

    companion object {
        private const val TAG = "GoService"
        private const val BINARY_NAME = "stock-terminal"
        private const val PORT = "8081"
    }

    private var process: Process? = null
    @Volatile
    private var lastExitCode: Int? = null
    @Volatile
    private var lastErrorOutput: String = ""

    fun start() {
        if (process != null) return

        Thread {
            try {
                val binary = extractBinary()
                if (binary == null) {
                    Log.e(TAG, "Binary extraction failed")
                    return@Thread
                }

                val filesDir = context.filesDir
                Log.i(TAG, "Binary: ${binary.absolutePath}, size=${binary.length()}, exec=${binary.canExecute()}")
                Log.i(TAG, "WorkDir: ${filesDir.absolutePath}, exists=${filesDir.exists()}, writable=${filesDir.canWrite()}")

                val pb = ProcessBuilder(binary.absolutePath)
                pb.directory(filesDir)
                pb.environment()["PORT"] = PORT
                pb.environment()["FORCE_MOBILE"] = "1"
                // 不合并 stderr，单独捕获错误输出
                pb.redirectErrorStream(false)

                process = pb.start()

                // 在后台线程读取 stderr 并记录到 logcat
                val stderrThread = Thread {
                    try {
                        val reader = BufferedReader(InputStreamReader(process!!.errorStream))
                        val sb = StringBuilder()
                        var line: String?
                        while (reader.readLine().also { line = it } != null) {
                            sb.appendLine(line)
                            Log.e(TAG, "[Go STDERR] $line")
                        }
                        lastErrorOutput = sb.toString()
                    } catch (_: Exception) {}
                }
                stderrThread.isDaemon = true
                stderrThread.start()

                // 在后台线程读取 stdout（防止管道满导致进程阻塞）
                val stdoutThread = Thread {
                    try {
                        process!!.inputStream.buffered().use { it.readBytes() }
                    } catch (_: Exception) {}
                }
                stdoutThread.isDaemon = true
                stdoutThread.start()

                // 等待进程退出
                val exitCode = process!!.waitFor()
                lastExitCode = exitCode
                Log.e(TAG, "Go process exited with code: $exitCode")
                if (lastErrorOutput.isNotEmpty()) {
                    Log.e(TAG, "Go stderr output:\n$lastErrorOutput")
                }
            } catch (e: Exception) {
                Log.e(TAG, "Failed to start Go process", e)
            } finally {
                process = null
            }
        }.start()
    }

    fun stop() {
        process?.let {
            try {
                it.destroy()
                Log.i(TAG, "Go process stopped")
            } catch (e: Exception) {
                Log.e(TAG, "Error stopping Go process", e)
            }
        }
        process = null
    }

    fun isRunning(): Boolean = process != null

    fun getExitCode(): Int? = lastExitCode

    fun getErrorOutput(): String = lastErrorOutput

    private fun extractBinary(): File? {
        val outFile = File(context.filesDir, BINARY_NAME)

        try {
            // 从 assets 提取二进制文件
            context.assets.open(BINARY_NAME).use { input ->
                FileOutputStream(outFile).use { output ->
                    val bytesCopied = input.copyTo(output)
                    Log.i(TAG, "Extracted $bytesCopied bytes to ${outFile.absolutePath}")
                }
            }

            // 验证文件大小
            if (outFile.length() == 0L) {
                Log.e(TAG, "Extracted binary is empty!")
                return null
            }

            // 设置可执行权限
            val execResult = outFile.setExecutable(true, false)
            Log.i(TAG, "setExecutable result: $execResult, canExecute: ${outFile.canExecute()}")

            // 验证 ELF 头（Linux ARM 二进制的魔数是 0x7f454c46）
            val header = outFile.readBytes().take(4).toByteArray()
            val isELF = header.size >= 4 &&
                header[0] == 0x7F.toByte() &&
                header[1] == 'E'.code.toByte() &&
                header[2] == 'L'.code.toByte() &&
                header[3] == 'F'.code.toByte()
            Log.i(TAG, "ELF header check: $isELF (bytes: ${header.joinToString(" ") { "%02x".format(it) }})")

            if (!isELF) {
                Log.e(TAG, "Binary is not a valid ELF file! Asset may be corrupted.")
                return null
            }

            return outFile
        } catch (e: Exception) {
            Log.e(TAG, "Failed to extract binary", e)
            return null
        }
    }
}
