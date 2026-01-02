# VS Code Configuration for ElasticObservability

This directory contains VS Code configuration files for building and debugging the ElasticObservability project in Windows, macOS, and Linux environments.

## Prerequisites

### Required Extensions

When you open this project in VS Code, you'll be prompted to install recommended extensions. The most important one is:

- **Go** (golang.go) - Official Go extension for VS Code

Other recommended extensions will enhance your development experience.

### Go Installation

Ensure Go 1.21 or later is installed and available in your PATH:

```powershell
# Windows PowerShell
go version
```

```bash
# Linux/macOS
go version
```

### Delve Debugger (Windows)

For debugging on Windows, install the Delve debugger:

```powershell
go install github.com/go-delve/delve/cmd/dlv@latest
```

## Configuration Files

### 1. tasks.json

Defines build and run tasks that can be executed via:
- **Command Palette** → `Tasks: Run Task`
- **Terminal** → `Run Task...`
- **Keyboard shortcut**: `Ctrl+Shift+B` (default build task)

Available tasks:

#### Build Tasks
- **build** (default) - Builds the application for Windows (`.exe`)
- **build (Linux/macOS)** - Builds for Unix-like systems

#### Run Tasks
- **run** - Builds and runs the application with default config
- **clean** - Removes build artifacts and logs
- **test** - Runs all tests
- **go mod tidy** - Downloads and tidies dependencies
- **setup directories** - Creates necessary directories

### 2. launch.json

Defines debug configurations accessible via:
- **Run and Debug** panel (Ctrl+Shift+D)
- **F5** to start debugging

Available configurations:

#### Main Configurations
- **Launch ElasticObservability** - Debug main application with verbose logging
- **Launch with custom config** - Debug with custom command-line arguments

#### Testing Configurations
- **Debug Tests** - Debug all tests in the workspace
- **Debug Current Test** - Debug tests in the current file/directory

#### Advanced Configurations
- **Attach to Process** - Attach debugger to running process
- **Debug Current File** - Debug the currently open Go file
- **Debug Package** - Debug a specific package (prompts for package name)

### 3. settings.json

Workspace-specific settings for:
- Go language server configuration
- Code formatting (gofmt on save)
- Import organization
- Linting with golangci-lint
- File exclusions
- Terminal profiles for Windows

### 4. extensions.json

Lists recommended VS Code extensions for optimal development experience.

## Getting Started

### 1. Open Project in VS Code

```powershell
# Windows
cd C:\path\to\ElasticObservability
code .
```

### 2. Install Recommended Extensions

When prompted, click "Install All" or install individually from the Extensions panel.

### 3. Setup Directories

Run the setup task to create necessary directories:

1. Press `Ctrl+Shift+P` (Command Palette)
2. Type: `Tasks: Run Task`
3. Select: `setup directories`

Or via terminal:
```powershell
# Windows PowerShell
New-Item -ItemType Directory -Path logs,outputs,configs/oneTime,configs/processedOneTime,data -Force
```

### 4. Build the Project

**Option 1: Using Keyboard Shortcut**
- Press `Ctrl+Shift+B` (runs default build task)

**Option 2: Using Command Palette**
1. Press `Ctrl+Shift+P`
2. Type: `Tasks: Run Build Task`
3. Select: `build`

**Option 3: Using Terminal**
```powershell
go build -o elasticobservability.exe ./cmd/main.go
```

### 5. Run the Application

**Option 1: Using Task**
1. Press `Ctrl+Shift+P`
2. Type: `Tasks: Run Task`
3. Select: `run`

**Option 2: Using Terminal**
```powershell
.\elasticobservability.exe -config config.yaml
```

### 6. Debug the Application

**Option 1: Using F5**
1. Press `F5` to start debugging with the default configuration
2. Application starts with debugger attached

**Option 2: Using Debug Panel**
1. Click on the Run and Debug icon (Ctrl+Shift+D)
2. Select a configuration from the dropdown
3. Click the green play button or press F5

## Debugging Tips

### Setting Breakpoints

1. Click in the gutter (left of line numbers) to set a breakpoint
2. Red dot appears indicating an active breakpoint
3. When debugging, execution pauses at breakpoints

### Debug Controls

- **F5** - Continue/Start debugging
- **F10** - Step over
- **F11** - Step into
- **Shift+F11** - Step out
- **Ctrl+Shift+F5** - Restart
- **Shift+F5** - Stop

### Debug Console

While debugging, use the Debug Console to:
- Evaluate expressions
- Inspect variables
- Execute Go code

Example:
```
> cluster.ClusterName
> len(types.AllClusters)
```

### Watch Expressions

Add variables to the Watch panel to monitor their values during debugging:
1. Right-click on a variable
2. Select "Add to Watch"

## Windows-Specific Notes

### PowerShell Execution Policy

If you encounter script execution errors, you may need to adjust PowerShell execution policy:

```powershell
# Check current policy
Get-ExecutionPolicy

# Set policy (as Administrator)
Set-ExecutionPolicy -ExecutionPolicy RemoteSigned -Scope CurrentUser
```

### Path Issues

Ensure Go binaries are in your PATH:
```powershell
$env:Path += ";C:\Go\bin;$env:USERPROFILE\go\bin"
```

To make it permanent, add to System Environment Variables.

### File Paths

The configuration uses `${workspaceFolder}` which automatically resolves to the correct path on Windows.

## Common Tasks

### Building for Different Platforms

**Windows (from Windows):**
```powershell
go build -o elasticobservability.exe ./cmd/main.go
```

**Linux (from Windows):**
```powershell
$env:GOOS="linux"; $env:GOARCH="amd64"; go build -o elasticobservability ./cmd/main.go
```

**macOS (from Windows):**
```powershell
$env:GOOS="darwin"; $env:GOARCH="amd64"; go build -o elasticobservability ./cmd/main.go
```

### Running Tests

**All tests:**
```powershell
go test -v ./...
```

**Specific package:**
```powershell
go test -v ./pkg/jobs
```

**With coverage:**
```powershell
go test -v -cover ./...
```

### Code Formatting

Format is automatic on save, but you can manually format:
```powershell
go fmt ./...
```

### Linting

If golangci-lint is installed:
```powershell
golangci-lint run ./...
```

## Troubleshooting

### "dlv: command not found"

Install Delve debugger:
```powershell
go install github.com/go-delve/delve/cmd/dlv@latest
```

### "go: command not found"

Ensure Go is installed and in PATH:
```powershell
# Add to PATH temporarily
$env:Path += ";C:\Go\bin"

# Verify
go version
```

### Breakpoints Not Working

1. Ensure you're building with debug symbols (default)
2. Check that Delve is properly installed
3. Try rebuilding the project
4. Restart VS Code

### Import Errors

Run go mod tidy:
```powershell
go mod tidy
```

Or use the task:
1. Ctrl+Shift+P → `Tasks: Run Task` → `go mod tidy`

### Port Already in Use

If you get "port already in use" errors:
1. Check for running instances
2. Change ports in `config.yaml`
3. Kill the process using the port:

```powershell
# Find process using port 9092
netstat -ano | findstr :9092

# Kill process (replace PID)
taskkill /PID <PID> /F
```

## Additional Resources

- [Go in VS Code](https://code.visualstudio.com/docs/languages/go)
- [Debugging Go in VS Code](https://github.com/golang/vscode-go/wiki/debugging)
- [Delve Debugger Documentation](https://github.com/go-delve/delve)
- [ElasticObservability README](../README.md)
- [Quick Start Guide](../QUICKSTART.md)

## VS Code Keyboard Shortcuts (Windows)

| Action | Shortcut |
|--------|----------|
| Command Palette | `Ctrl+Shift+P` |
| Quick Open | `Ctrl+P` |
| Build | `Ctrl+Shift+B` |
| Debug | `F5` |
| Run without Debug | `Ctrl+F5` |
| Toggle Terminal | `Ctrl+`` |
| Toggle Debug Console | `Ctrl+Shift+Y` |
| Toggle Problems | `Ctrl+Shift+M` |
| Go to Definition | `F12` |
| Find References | `Shift+F12` |
| Rename Symbol | `F2` |
| Format Document | `Shift+Alt+F` |

## Support

For issues specific to the ElasticObservability application, refer to the main [README.md](../README.md) or check the application logs in the `logs/` directory.
