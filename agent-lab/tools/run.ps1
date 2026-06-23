# agent-lab Windows entry script (Makefile equivalent).
#
# Usage from repo root:
#   .\agent-lab\tools\run.ps1 fake
#   .\agent-lab\tools\run.ps1 local-web
#   .\agent-lab\tools\run.ps1 local-web -Lazy
#   .\agent-lab\tools\run.ps1 local-web -QuietPip
#   .\agent-lab\tools\run.ps1 py-openai
#   .\agent-lab\tools\run.ps1 py-chat -Msg "hello"
#   .\agent-lab\tools\run.ps1 py-web
#   .\agent-lab\tools\run.ps1 web
#   .\agent-lab\tools\run.ps1 demo-web
#   .\agent-lab\tools\run.ps1 chat -Msg "hello"
#   .\agent-lab\tools\run.ps1 chat-once -Msg "hello"
#   .\agent-lab\tools\run.ps1 demo
#   .\agent-lab\tools\run.ps1 build|test|vet|fmt|help

param(
    [Parameter(Position=0)]
    [ValidateSet("chat","chat-once","fake","local-web","py-openai","py-chat","py-web","web","demo","demo-web","build","test","vet","fmt","help")]
    [string]$Cmd = "help",

    [string]$Msg     = "hello, please introduce yourself in one sentence",
    [string]$BaseUrl = "http://127.0.0.1:18080/v1",
    [string]$ApiKey  = "sk-local",
    [string]$Profile = "L",
    [string]$WebAddr = "127.0.0.1:8090",
    [string]$PyModel = "Qwen/Qwen1.5-1.8B-Chat",
    [ValidateSet("auto","cuda","mps","cpu")]
    [string]$PyDevice = "auto",
    [switch]$Lazy,
    [switch]$SkipPyInstall,
    [switch]$QuietPip,
    [string]$TorchIndex = "https://download.pytorch.org/whl/cu128"
)

$ErrorActionPreference = "Stop"

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$AgentLab = Join-Path $RepoRoot "agent-lab"
Push-Location $RepoRoot

function Load-LocalEnv($Path) {
    if (-not (Test-Path $Path)) { return }
    $keys = New-Object System.Collections.Generic.List[string]
    Get-Content $Path | ForEach-Object {
        $line = $_.Trim()
        if ($line -eq "" -or $line.StartsWith("#")) { return }
        $idx = $line.IndexOf("=")
        if ($idx -le 0) { return }
        $name = $line.Substring(0, $idx).Trim()
        $value = $line.Substring($idx + 1).Trim().Trim('"').Trim("'")
        if ($name -match "^[A-Za-z_][A-Za-z0-9_]*$") {
            Set-Item -Path "Env:$name" -Value $value
            $keys.Add($name)
        }
    }
    $keyText = if ($keys.Count -gt 0) { $keys -join "," } else { "none" }
    Write-Host "[env] loaded local environment from $Path keys=$keyText" -ForegroundColor DarkGray
}

Load-LocalEnv (Join-Path $RepoRoot ".env.local")
Load-LocalEnv (Join-Path $AgentLab ".env.local")
if ($env:HF_TOKEN -and -not $env:HF_HUB_TOKEN) { $env:HF_HUB_TOKEN = $env:HF_TOKEN }
if ($env:HF_TOKEN -and -not $env:HUGGINGFACE_HUB_TOKEN) { $env:HUGGINGFACE_HUB_TOKEN = $env:HF_TOKEN }

function Set-AgentEnv {
    $env:OPENAI_BASE_URL  = $BaseUrl
    $env:OPENAI_API_KEY   = $ApiKey
    $env:AGENTLAB_PROFILE = $Profile
}

function Show-Help {
    Get-Content $PSCommandPath | Select-String -Pattern "^# " | ForEach-Object { $_.Line.Substring(2) }
}

function Wait-Health($Url, $TimeoutSec = 20) {
    $deadline = (Get-Date).AddSeconds($TimeoutSec)
    while ((Get-Date) -lt $deadline) {
        try {
            Invoke-WebRequest -UseBasicParsing -TimeoutSec 1 -Uri $Url | Out-Null
            return $true
        } catch {
            if ($_.Exception.Response) { return $true }
            Start-Sleep -Milliseconds 200
        }
    }
    return $false
}

function Get-PythonExe {
    $venv = Join-Path $AgentLab ".venv"
    return Join-Path $venv "Scripts\python.exe"
}

function Invoke-PipInstall($PythonExe, $Arguments, $LogPath, $Label) {
    Write-Host "[python-env] $Label ..." -ForegroundColor Cyan
    Write-Host "[python-env] log: $LogPath" -ForegroundColor DarkGray
    $argList = @("-m", "pip") + $Arguments + @("--progress-bar", "on")
    if (-not $QuietPip) { $argList += "--verbose" }
    $logWriteFailed = $false
    & $PythonExe @argList 2>&1 | ForEach-Object {
        $line = $_.ToString()
        Write-Host $line
        if (-not $logWriteFailed) {
            try {
                Add-Content -Path $LogPath -Value $line -ErrorAction Stop
            } catch {
                $logWriteFailed = $true
                Write-Host "[python-env] log file is locked; continuing with terminal output only" -ForegroundColor Yellow
            }
        }
    }
    $code = $LASTEXITCODE
    if ($null -eq $code) { $code = 0 }
    if ($code -ne 0) {
        if (Test-Path $LogPath) { Get-Content $LogPath -Tail 60 | ForEach-Object { Write-Host "[pip] $_" -ForegroundColor Red } }
        throw "pip failed with exit code $code"
    }
    Write-Host "[python-env] $Label done" -ForegroundColor Green
}

function Test-TorchCUDA($PythonExe) {
    try {
        $out = & $PythonExe -c "import torch; print(torch.cuda.is_available()); print(torch.version.cuda or '')" 2>$null
        return (($out | Select-Object -First 1) -eq "True")
    } catch {
        return $false
    }
}

function Ensure-PythonEnv {
    $venv = Join-Path $AgentLab ".venv"
    $pythonExe = Get-PythonExe
    $requirements = Join-Path $AgentLab "scripts\python-openai-server\requirements.txt"
    $stamp = Join-Path $venv ".requirements.stamp"
    $pipLog = Join-Path $venv ("pip-install-" + $PID + ".log")
    if (-not (Test-Path $pythonExe)) {
        Write-Host "[python-env] creating virtual environment at $venv ..." -ForegroundColor Cyan
        python -m venv $venv
    }
    if (-not $SkipPyInstall) {
        $reqHash = ((Get-FileHash $requirements -Algorithm SHA256).Hash + "|torch-index=" + $TorchIndex)
        $oldHash = if (Test-Path $stamp) { Get-Content $stamp -Raw } else { "" }
        if ($reqHash -ne $oldHash.Trim()) {
            if (Test-Path $pipLog) { Remove-Item $pipLog -Force }
            Invoke-PipInstall -PythonExe $pythonExe -Arguments @("install", "--upgrade", "pip") -LogPath $pipLog -Label "upgrading pip in virtual environment"
            $torchArgs = @("install", "--index-url", $TorchIndex, "torch")
            if (-not (Test-TorchCUDA $pythonExe)) { $torchArgs = @("install", "--force-reinstall", "--index-url", $TorchIndex, "torch") }
            Invoke-PipInstall -PythonExe $pythonExe -Arguments $torchArgs -LogPath $pipLog -Label "installing CUDA torch into virtual environment"
            Invoke-PipInstall -PythonExe $pythonExe -Arguments @("install", "-r", $requirements) -LogPath $pipLog -Label "installing dependencies into virtual environment"
            Set-Content -Path $stamp -Value $reqHash
        }
    }
    return $pythonExe
}

function Start-PythonOpenAI($Stdout, $Stderr) {
    $pythonExe = Ensure-PythonEnv
    $script = Join-Path $AgentLab "scripts\python-openai-server\main.py"
    $args = @($script, "--model", $PyModel, "--device", $PyDevice)
    if ($Lazy) { $args += "--lazy" }
    $spArgs = @{
        FilePath               = $pythonExe
        ArgumentList           = $args
        WorkingDirectory       = $RepoRoot
        PassThru               = $true
        WindowStyle            = "Hidden"
        RedirectStandardOutput = $Stdout
        RedirectStandardError  = $Stderr
    }
    Start-Process @spArgs
}

function Get-BaseHealthURL {
    $u = $BaseUrl.TrimEnd("/")
    $u = $u -replace "/v1$", ""
    return "$u/healthz"
}

function Stop-ListeningPort($Port, $Label) {
    $conns = Get-NetTCPConnection -LocalPort $Port -State Listen -ErrorAction SilentlyContinue
    foreach ($conn in $conns) {
        $pidToStop = $conn.OwningProcess
        if ($pidToStop -and $pidToStop -ne $PID) {
            $proc = Get-Process -Id $pidToStop -ErrorAction SilentlyContinue
            if ($proc) {
                Write-Host "[local-web] stopping stale $Label on port $Port (pid=$pidToStop, process=$($proc.ProcessName))" -ForegroundColor Yellow
                Stop-Process -Id $pidToStop -Force -ErrorAction SilentlyContinue
            }
        }
    }
}

$script:LogOffsets = @{}

function Write-NewLog($Path, $Prefix) {
    if (-not (Test-Path $Path)) { return }
    $offset = if ($script:LogOffsets.ContainsKey($Path)) { [int64]$script:LogOffsets[$Path] } else { [int64]0 }
    $fs = [System.IO.File]::Open($Path, [System.IO.FileMode]::Open, [System.IO.FileAccess]::Read, [System.IO.FileShare]::ReadWrite)
    try {
        if ($fs.Length -lt $offset) { $offset = 0 }
        $fs.Seek($offset, [System.IO.SeekOrigin]::Begin) | Out-Null
        $sr = New-Object System.IO.StreamReader($fs)
        while (-not $sr.EndOfStream) {
            $line = $sr.ReadLine()
            if ($line -ne $null -and $line.Trim() -ne "") { Write-Host "[$Prefix] $line" }
        }
        $script:LogOffsets[$Path] = $fs.Position
    } finally {
        $fs.Close()
    }
}

function Wait-HealthWithLogs($Url, $TimeoutSec, $LogSpecs) {
    $start = Get-Date
    $deadline = $start.AddSeconds($TimeoutSec)
    $nextHeartbeat = $start.AddSeconds(10)
    while ((Get-Date) -lt $deadline) {
        foreach ($spec in $LogSpecs) { Write-NewLog -Path $spec.Path -Prefix $spec.Prefix }
        try {
            Invoke-WebRequest -UseBasicParsing -TimeoutSec 1 -Uri $Url | Out-Null
            foreach ($spec in $LogSpecs) { Write-NewLog -Path $spec.Path -Prefix $spec.Prefix }
            return $true
        } catch {
            if ($_.Exception.Response) { return $true }
            $now = Get-Date
            if ($now -ge $nextHeartbeat) {
                $elapsed = [int]($now - $start).TotalSeconds
                Write-Host "[local-web] still waiting for service readiness at $Url (${elapsed}s elapsed) ..." -ForegroundColor Yellow
                $nextHeartbeat = $now.AddSeconds(10)
            }
            Start-Sleep -Milliseconds 500
        }
    }
    foreach ($spec in $LogSpecs) { Write-NewLog -Path $spec.Path -Prefix $spec.Prefix }
    return $false
}

function Get-PortFromAddr($Addr) {
    if ($Addr -match "^https?://") {
        return ([Uri]$Addr).Port
    }
    $parts = $Addr.Split(":")
    return [int]$parts[$parts.Length - 1]
}

try {
    switch ($Cmd) {
        "help"      { Show-Help }
        "build"     { go build ./agent-lab/... }
        "test"      { go test  ./agent-lab/... -count=1 }
        "vet"       { go vet   ./agent-lab/... }
        "fmt"       { gofmt -w (Join-Path $AgentLab ".") }
        "fake"      { go run ./agent-lab/scripts/fake-openai }
        "py-openai" {
            $pythonExe = Ensure-PythonEnv
            $script = Join-Path $AgentLab "scripts\python-openai-server\main.py"
            $args = @($script, "--model", $PyModel, "--device", $PyDevice)
            if ($Lazy) { $args += "--lazy" }
            & $pythonExe @args
        }
        "local-web" {
            $Profile = "S"
            Set-AgentEnv
            $env:AGENTLAB_MODEL_CHAT = "qwen1.5-1.8b-chat"
            $pyStdout = Join-Path $env:TEMP "agent-lab-py-openai.out.log"
            $pyStderr = Join-Path $env:TEMP "agent-lab-py-openai.err.log"
            $webStdout = Join-Path $env:TEMP "agent-lab-web.out.log"
            $webStderr = Join-Path $env:TEMP "agent-lab-web.err.log"
            Stop-ListeningPort -Port (Get-PortFromAddr $WebAddr) -Label "web UI"
            Stop-ListeningPort -Port (Get-PortFromAddr $BaseUrl) -Label "model server"
            Start-Sleep -Milliseconds 300
            Write-Host "[local-web] starting local model and web UI ..." -ForegroundColor Cyan
            if (-not $Lazy) { Write-Host "[local-web] preloading model before opening web UI; first run may download model files" -ForegroundColor Yellow }
            Remove-Item $pyStdout,$pyStderr,$webStdout,$webStderr -ErrorAction SilentlyContinue
            $py = Start-PythonOpenAI -Stdout $pyStdout -Stderr $pyStderr
            $pyLogs = @(@{ Path = $pyStdout; Prefix = "py" }, @{ Path = $pyStderr; Prefix = "py" })
            try {
                if (-not (Wait-HealthWithLogs -Url (Get-BaseHealthURL) -TimeoutSec 1200 -LogSpecs $pyLogs)) {
                    if (Test-Path $pyStderr) { Get-Content $pyStderr | Select-Object -Last 40 | Write-Host }
                    throw "local model server not ready"
                }
                $webArgs = @{
                    FilePath               = "go"
                    ArgumentList           = @("run","./agent-lab/cmd/web","-addr",$WebAddr)
                    WorkingDirectory       = $RepoRoot
                    PassThru               = $true
                    WindowStyle            = "Hidden"
                    RedirectStandardOutput = $webStdout
                    RedirectStandardError  = $webStderr
                }
                $web = Start-Process @webArgs
                $allLogs = @(@{ Path = $pyStdout; Prefix = "py" }, @{ Path = $pyStderr; Prefix = "py" }, @{ Path = $webStdout; Prefix = "web" }, @{ Path = $webStderr; Prefix = "web" })
                try {
                    if (-not (Wait-HealthWithLogs -Url ("http://" + $WebAddr + "/healthz") -TimeoutSec 30 -LogSpecs $allLogs)) {
                        if (Test-Path $webStderr) { Get-Content $webStderr | Select-Object -Last 40 | Write-Host }
                        throw "web UI not ready"
                    }
                    Write-Host "[local-web] open http://$WebAddr/" -ForegroundColor Green
                    while (-not $web.HasExited -and -not $py.HasExited) {
                        foreach ($spec in $allLogs) { Write-NewLog -Path $spec.Path -Prefix $spec.Prefix }
                        Start-Sleep -Milliseconds 500
                    }
                    foreach ($spec in $allLogs) { Write-NewLog -Path $spec.Path -Prefix $spec.Prefix }
                } finally {
                    Stop-Process -Id $web.Id -Force -ErrorAction SilentlyContinue
                    Remove-Item $webStdout,$webStderr -ErrorAction SilentlyContinue
                }
            } finally {
                Stop-Process -Id $py.Id -Force -ErrorAction SilentlyContinue
                Remove-Item $pyStdout,$pyStderr -ErrorAction SilentlyContinue
            }
        }
        "py-chat" {
            Set-AgentEnv
            $env:AGENTLAB_MODEL_CHAT = "qwen1.5-1.8b-chat"
            $stdout = Join-Path $env:TEMP "agent-lab-py-openai.out.log"
            $stderr = Join-Path $env:TEMP "agent-lab-py-openai.err.log"
            Write-Host "[py-chat] starting python OpenAI server on $BaseUrl ..." -ForegroundColor Cyan
            $proc = Start-PythonOpenAI -Stdout $stdout -Stderr $stderr
            try {
                if (-not (Wait-Health -Url (Get-BaseHealthURL) -TimeoutSec 30)) {
                    if (Test-Path $stderr) { Get-Content $stderr | Select-Object -Last 40 | Write-Host }
                    throw "python OpenAI server not ready"
                }
                Write-Host "[py-chat] backend ready. starting chat ..." -ForegroundColor Green
                go run ./agent-lab/cmd/chat -m "$Msg"
            } finally {
                Write-Host "[py-chat] stopping python OpenAI server (pid=$($proc.Id))" -ForegroundColor Cyan
                Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
                Remove-Item $stdout,$stderr -ErrorAction SilentlyContinue
            }
        }
        "py-web" {
            Set-AgentEnv
            $env:AGENTLAB_MODEL_CHAT = "qwen1.5-1.8b-chat"
            $pyStdout = Join-Path $env:TEMP "agent-lab-py-openai.out.log"
            $pyStderr = Join-Path $env:TEMP "agent-lab-py-openai.err.log"
            $webStdout = Join-Path $env:TEMP "agent-lab-web.out.log"
            $webStderr = Join-Path $env:TEMP "agent-lab-web.err.log"
            Write-Host "[py-web] starting python OpenAI server on $BaseUrl ..." -ForegroundColor Cyan
            $py = Start-PythonOpenAI -Stdout $pyStdout -Stderr $pyStderr
            try {
                if (-not (Wait-Health -Url (Get-BaseHealthURL) -TimeoutSec 30)) {
                    if (Test-Path $pyStderr) { Get-Content $pyStderr | Select-Object -Last 40 | Write-Host }
                    throw "python OpenAI server not ready"
                }
                Write-Host "[py-web] starting cmd/web on http://$WebAddr ..." -ForegroundColor Cyan
                $webArgs = @{
                    FilePath               = "go"
                    ArgumentList           = @("run","./agent-lab/cmd/web","-addr",$WebAddr)
                    WorkingDirectory       = $RepoRoot
                    PassThru               = $true
                    WindowStyle            = "Hidden"
                    RedirectStandardOutput = $webStdout
                    RedirectStandardError  = $webStderr
                }
                $web = Start-Process @webArgs
                try {
                    if (-not (Wait-Health -Url ("http://" + $WebAddr + "/healthz") -TimeoutSec 30)) {
                        if (Test-Path $webStderr) { Get-Content $webStderr | Select-Object -Last 40 | Write-Host }
                        throw "cmd/web not ready"
                    }
                    Write-Host "[py-web] open http://$WebAddr/ in browser. Ctrl-C here to stop." -ForegroundColor Green
                    Wait-Process -Id $web.Id
                } finally {
                    Stop-Process -Id $web.Id -Force -ErrorAction SilentlyContinue
                    Remove-Item $webStdout,$webStderr -ErrorAction SilentlyContinue
                }
            } finally {
                Write-Host "[py-web] stopping python OpenAI server (pid=$($py.Id))" -ForegroundColor Cyan
                Stop-Process -Id $py.Id -Force -ErrorAction SilentlyContinue
                Remove-Item $pyStdout,$pyStderr -ErrorAction SilentlyContinue
            }
        }
        "web" {
            Set-AgentEnv
            go run ./agent-lab/cmd/web -addr $WebAddr
        }
        "chat" {
            Set-AgentEnv
            go run ./agent-lab/cmd/chat -m "$Msg"
        }
        "chat-once" {
            Set-AgentEnv
            go run ./agent-lab/cmd/chat -m "$Msg" -no-stream
        }
        "demo" {
            Set-AgentEnv
            $stdout = Join-Path $env:TEMP "agent-lab-fake.out.log"
            $stderr = Join-Path $env:TEMP "agent-lab-fake.err.log"
            Write-Host "[demo] starting fake-openai on $BaseUrl ..." -ForegroundColor Cyan
            $spArgs = @{
                FilePath               = "go"
                ArgumentList           = @("run","./agent-lab/scripts/fake-openai")
                WorkingDirectory       = $RepoRoot
                PassThru               = $true
                WindowStyle            = "Hidden"
                RedirectStandardOutput = $stdout
                RedirectStandardError  = $stderr
            }
            $proc = Start-Process @spArgs
            try {
                $ok = Wait-Health -Url $BaseUrl -TimeoutSec 20
                if (-not $ok) { throw "fake-openai did not become ready in time" }
                Write-Host "[demo] === streaming ===" -ForegroundColor Green
                go run ./agent-lab/cmd/chat -m "$Msg" -max-tokens 64
                Write-Host ""
                Write-Host "[demo] === non-streaming ===" -ForegroundColor Green
                go run ./agent-lab/cmd/chat -m "say it again" -no-stream -max-tokens 64
            } finally {
                Write-Host "[demo] stopping fake-openai (pid=$($proc.Id))" -ForegroundColor Cyan
                Stop-Process -Id $proc.Id -Force -ErrorAction SilentlyContinue
                Remove-Item $stdout,$stderr -ErrorAction SilentlyContinue
            }
        }
        "demo-web" {
            Set-AgentEnv
            $fakeStdout = Join-Path $env:TEMP "agent-lab-fake.out.log"
            $fakeStderr = Join-Path $env:TEMP "agent-lab-fake.err.log"
            $webStdout  = Join-Path $env:TEMP "agent-lab-web.out.log"
            $webStderr  = Join-Path $env:TEMP "agent-lab-web.err.log"
            Write-Host "[demo-web] starting fake-openai on $BaseUrl ..." -ForegroundColor Cyan
            $fakeArgs = @{
                FilePath               = "go"
                ArgumentList           = @("run","./agent-lab/scripts/fake-openai")
                WorkingDirectory       = $RepoRoot
                PassThru               = $true
                WindowStyle            = "Hidden"
                RedirectStandardOutput = $fakeStdout
                RedirectStandardError  = $fakeStderr
            }
            $fake = Start-Process @fakeArgs
            try {
                if (-not (Wait-Health -Url $BaseUrl)) { throw "fake-openai not ready" }
                Write-Host "[demo-web] starting cmd/web on http://$WebAddr ..." -ForegroundColor Cyan
                $webArgs = @{
                    FilePath               = "go"
                    ArgumentList           = @("run","./agent-lab/cmd/web","-addr",$WebAddr)
                    WorkingDirectory       = $RepoRoot
                    PassThru               = $true
                    WindowStyle            = "Hidden"
                    RedirectStandardOutput = $webStdout
                    RedirectStandardError  = $webStderr
                }
                $web = Start-Process @webArgs
                try {
                    if (-not (Wait-Health -Url ("http://" + $WebAddr + "/healthz") -TimeoutSec 30)) {
                        throw "cmd/web not ready"
                    }
                    Write-Host "[demo-web] open http://$WebAddr/ in browser. Ctrl-C here to stop." -ForegroundColor Green
                    Wait-Process -Id $web.Id
                } finally {
                    Stop-Process -Id $web.Id -Force -ErrorAction SilentlyContinue
                    Remove-Item $webStdout,$webStderr -ErrorAction SilentlyContinue
                }
            } finally {
                Stop-Process -Id $fake.Id -Force -ErrorAction SilentlyContinue
                Remove-Item $fakeStdout,$fakeStderr -ErrorAction SilentlyContinue
            }
        }
    }
}
finally {
    Pop-Location
}
