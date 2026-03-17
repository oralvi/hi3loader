# Go Rewrite

`go_rewrite` 是对 `target.exe` 的完整 Go 重写目录。

当前包含：

- 重新按字节码还原后的 B 站登录、米哈游验证、扫码确认流程
- 本地验证码回调服务
- 剪贴板二维码解析
- 当前活动窗口截图并识别二维码
- Wails 桌面前端
- 命令行入口 `cmd/bh3cli`

主要目录：

- `cmd/bh3cli`：命令行入口
- `internal/service`：统一服务层，CLI 和 GUI 共用
- `internal/bsgamesdk`：B 站登录协议
- `internal/mihoyosdk`：米哈游验证与扫码协议
- `internal/captcha`：本地验证码服务与内置模板
- `internal/qr`：二维码解析
- `frontend`：Wails 前端

常用命令：

```powershell
cd go_rewrite
go build ./...
go run .\cmd\bh3cli state
go run .\cmd\bh3cli version
npm install --prefix .\frontend
C:\Users\rin\go\bin\wails.exe build
```

产物：

- GUI 可执行文件：`build/bin/hi3loader_rebuilt.exe`

说明：

- 我保留了原程序的登录与扫码后端能力，但没有复刻原样本里验证码页向第三方站点上报图片的那段逻辑。
- 本地验证码服务改为绑定 `127.0.0.1:12983`，不再暴露到 `0.0.0.0`。
