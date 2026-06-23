# agent-lab Windows entry script (Makefile equivalent).
#
# Usage from repo root:
#   .\agent-lab\tools\run.ps1 fake
#   .\agent-lab\tools\run.ps1 py-openai
#   .\agent-lab\tools\run.ps1 web
#   .\agent-lab\tools\run.ps1 demo-web
#   .\agent-lab\tools\run.ps1 chat -Msg "hello"
#   .\agent-lab\tools\run.ps1 chat-once -Msg "hello"
#   .\agent-lab\tools\run.ps1 demo
#   .\agent-lab\tools\run.ps1 build|test|vet|fmt|help

param(
    [Parameter(Position=0)]
    [ValidateSet("chat","chat-once","fake","py-openai","web","demo","demo-web","build","test","vet","fmt","help")]
    [string]$Cmd = "help",

    [string]$Msg     = "hello, please introduce yourself in one sentence",
    [string]$BaseUrl = "http://127.0.0.1:18080/v1",
    [string]$ApiKey  = "sk-local",
    [string]$Profile = "L",
    [string]$WebAddr = "127.0.0.1:8090",
    [string]$PyModel = "Qwen/Qwen1.5-1.8B-Chat",
    [ValidateSet("auto","cuda","mps","cpu")]
    [string]$PyDevice = "auto",
    [switch]$Lazy
)

$ErrorActionPreference = "Stop"

$RepoRoot = (Resolve-Path (Join-Path $PSScriptRoot "..\..")).Path
$AgentLab = Join-Path $RepoRoot "agent-lab"
Push-Location $RepoRoot

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

try {
    switch ($Cmd) {
        "help"      { Show-Help }
        "build"     { go build ./agent-lab/... }
        "test"      { go test  ./agent-lab/... -count=1 }
        "vet"       { go vet   ./agent-lab/... }
        "fmt"       { gofmt -w (Join-Path $AgentLab ".") }
        "fake"      { go run ./agent-lab/scripts/fake-openai }
        "py-openai" {
            $script = Join-Path $AgentLab "scripts\python-openai-server\main.py"
            $args = @($script, "--model", $PyModel, "--device", $PyDevice)
            if ($Lazy) { $args += "--lazy" }
            python @args
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
