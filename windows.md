# Semaphore Windows 构建指南

本文档提供了在 Windows 环境下构建 Semaphore 项目的详细步骤和解决方案。

## 环境要求

- Windows 10/11
- Go 1.21+
- Node.js 16+
- Git
- PowerShell 或 Command Prompt

## 常见问题及解决方案

### 问题 1: `task build` 命令失败

**错误信息：**

```
bash: task: command not found
```

**解决方案：**
安装 Task 工具：

```powershell
go install github.com/go-task/task/v3/cmd/task@latest
```

### 问题 2: vue-cli-service 未找到

**错误信息：**

```
'vue-cli-service' 不是内部或外部命令，也不是可运行的程序或批处理文件。
```

**解决方案：**
安装前端依赖：

```powershell
task deps:fe
```

### 问题 3: env 命令在 Windows 中不可用

**错误信息：**

```
"env": executable file not found in $PATH
```

**解决方案：**
Windows PowerShell 不支持 `env` 命令，需要直接运行相应的构建命令。

### 问题 4: PowerShell 执行策略阻止脚本运行

**错误信息：**

```
无法加载文件，因为在此系统上禁止运行脚本
```

**解决方案：**
使用 `cmd` 包装器运行 npm 命令：

```powershell
cmd /c "npm run build"
```

## 完整构建步骤

### 1. 安装依赖

```powershell
# 安装 Task 工具
go install github.com/go-task/task/v3/cmd/task@latest

# 安装前端依赖
task deps:fe

# 安装后端依赖
task deps:be
```

### 2. 构建前端

**方法一：使用 Task（如果兼容）**

```powershell
task build:fe
```

**方法二：手动构建（推荐）**

```powershell
cd web
cmd /c "npm run build"
cd ..
```

### 3. 构建后端

**方法一：使用 Task（如果兼容）**

```powershell
task build:be
```

**方法二：手动构建（推荐）**

```powershell
go build -o bin/semaphore.exe -tags "netgo" -ldflags "-s -w" ./cli
```

### 4. 验证构建

```powershell
# 检查二进制文件
ls bin/

# 检查版本
.\bin\semaphore.exe version

# 检查前端资源
ls api/public/
```

## Windows 兼容性改进建议

### 修改 Taskfile.yml 以支持 Windows

可以考虑为 Windows 环境添加特定的构建任务：

```yaml
  build:fe:windows:
    desc: Build VueJS project (Windows compatible)
    dir: web
    cmds:
      - cmd /c "set VUE_APP_BUILD_TYPE={{ .APP_BUILD_TYPE }} && npm run build"

  build:be:windows:
    desc: Build server binary (Windows compatible)
    cmds:
      - >-
        go build -o bin/semaphore.exe
        -tags "netgo"
        -ldflags "-s -w -X {{ .IMPORT }}/util.Ver={{ .VERSION }} -X {{ .IMPORT }}/util.Commit={{ .SHA }} -X {{ .IMPORT }}/util.Date={{ .DATE }}" ./cli
```

## 快速构建脚本

创建一个 Windows 批处理文件 `build.bat`：

```batch
@echo off
echo Building Semaphore for Windows...

echo Installing dependencies...
go install github.com/go-task/task/v3/cmd/task@latest
task deps:fe
task deps:be

echo Building frontend...
cd web
cmd /c "npm run build"
cd ..

echo Building backend...
go build -o bin/semaphore.exe -tags "netgo" -ldflags "-s -w" ./cli

echo Build completed!
echo Binary location: bin/semaphore.exe
.\bin\semaphore.exe version
```

或者创建 PowerShell 脚本 `build.ps1`：

```powershell
Write-Host "Building Semaphore for Windows..." -ForegroundColor Green

Write-Host "Installing dependencies..." -ForegroundColor Yellow
go install github.com/go-task/task/v3/cmd/task@latest
task deps:fe
task deps:be

Write-Host "Building frontend..." -ForegroundColor Yellow
Set-Location web
cmd /c "npm run build"
Set-Location ..

Write-Host "Building backend..." -ForegroundColor Yellow
go build -o bin/semaphore.exe -tags "netgo" -ldflags "-s -w" ./cli

Write-Host "Build completed!" -ForegroundColor Green
Write-Host "Binary location: bin/semaphore.exe" -ForegroundColor Cyan
.\bin\semaphore.exe version
```

## 故障排除

### 1. Go 模块问题

如果遇到模块依赖问题：

```powershell
go mod tidy
go mod vendor
```

### 2. Node.js 版本问题

确保使用兼容的 Node.js 版本：

```powershell
node --version
npm --version
```

### 3. 权限问题

如果遇到权限问题，以管理员身份运行 PowerShell 或 Command Prompt。

### 4. 路径问题

确保 Go 和 Node.js 已正确添加到系统 PATH 环境变量中。

## 开发环境设置

### 1. 配置 GOPATH（如果需要）

```powershell
$env:GOPATH = "$HOME\go"
$env:PATH += ";$env:GOPATH\bin"
```

### 2. 数据库设置（可选）

如果使用 MySQL：

```sql
CREATE DATABASE semaphore;
```

如果使用 BoltDB，无需额外配置。

### 3. 运行开发服务器

```powershell
# 设置配置
.\bin\semaphore.exe setup

# 运行服务器
.\bin\semaphore.exe server --config .\config.json
```

## 注意事项

1. **路径分隔符**：Windows 使用反斜杠 `\`，在某些配置中可能需要注意。
2. **文件权限**：Windows 的文件权限系统与 Unix 系统不同。
3. **环境变量**：使用 `$env:` 前缀设置 PowerShell 环境变量。
4. **脚本执行策略**：可能需要调整 PowerShell 执行策略。

## 相关链接

- [Semaphore 官方文档](https://semaphoreui.com/)
- [Go 安装指南](https://golang.org/doc/install)
- [Node.js 安装指南](https://nodejs.org/)
- [Task 工具文档](https://taskfile.dev/)

---

**最后更新：** 2025年10月26日
**版本：** 2.16-dev
