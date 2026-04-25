$ErrorActionPreference = "Stop"

$ScriptDir = Split-Path -Parent $MyInvocation.MyCommand.Path
$ProjectDir = Split-Path -Parent $ScriptDir
$RepoRoot = Split-Path -Parent $ProjectDir
$VenvDir = Join-Path $ProjectDir ".venv"
$script:NativeExitCode = 0

function Invoke-Native {
    param(
        [Parameter(Mandatory = $true)]
        [string] $FilePath,
        [string[]] $ArgumentList = @()
    )

    $previousErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        $script:NativeExitCode = 1
        & $FilePath @ArgumentList
        if ($null -ne $LASTEXITCODE) {
            $script:NativeExitCode = $LASTEXITCODE
        } elseif ($?) {
            $script:NativeExitCode = 0
        }
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }
}

function Invoke-NativeQuiet {
    param(
        [Parameter(Mandatory = $true)]
        [string] $FilePath,
        [string[]] $ArgumentList = @()
    )

    $previousErrorActionPreference = $ErrorActionPreference
    $ErrorActionPreference = "Continue"
    try {
        $script:NativeExitCode = 1
        & $FilePath @ArgumentList *> $null
        if ($null -ne $LASTEXITCODE) {
            $script:NativeExitCode = $LASTEXITCODE
        } elseif ($?) {
            $script:NativeExitCode = 0
        }
    } finally {
        $ErrorActionPreference = $previousErrorActionPreference
    }
}

function Find-BasePython {
    $loraPython = Join-Path $RepoRoot "lora\venv\Scripts\python.exe"
    if (Test-Path $loraPython) {
        return $loraPython
    }

    $candidates = @("py -3.13", "py -3.12", "py -3.11", "py -3", "python")
    foreach ($candidate in $candidates) {
        $parts = $candidate -split " "
        $command = $parts[0]
        $arguments = @()
        if ($parts.Length -gt 1) {
            $arguments = $parts[1..($parts.Length - 1)]
        }

        try {
            $probe = & $command @arguments -c "import sys; print(sys.executable)" 2>$null
            if ($LASTEXITCODE -eq 0 -and $probe) {
                return $probe
            }
        } catch {
            continue
        }
    }

    throw "No usable Python interpreter found. Install Python 3.11+ or make sure python/py is on PATH."
}

function Ensure-Venv {
    $venvPython = Join-Path $VenvDir "Scripts\python.exe"
    if (Test-Path $venvPython) {
        return
    }

    $basePython = Find-BasePython
    Write-Host "[start] creating venv with: $basePython"
    Invoke-Native $basePython @("-m", "venv", $VenvDir)
    if ($script:NativeExitCode -ne 0) {
        throw "Failed to create Python virtual environment."
    }
    Invoke-NativeQuiet $venvPython @("-m", "ensurepip", "--upgrade") | Out-Null
}

Ensure-Venv

$Python = Join-Path $VenvDir "Scripts\python.exe"
$env:PYTHON = $Python

$Port = if ($env:PORT) { $env:PORT } else { "8080" }
$Url = "http://127.0.0.1:$Port/studio"

Set-Location $ProjectDir

Write-Host "[start] using python: $Python"
Write-Host "[start] checking python dependencies..."
Invoke-NativeQuiet $Python @("-c", "import pydantic, langchain_core, langgraph, torch, transformers")
if ($script:NativeExitCode -ne 0) {
    Write-Host "[start] missing deps, installing from python/requirements.txt"
    Invoke-Native $Python @("-m", "pip", "install", "-q", "-r", "python\requirements.txt")
    if ($script:NativeExitCode -ne 0) {
        throw "Failed to install Python dependencies."
    }
} else {
    Write-Host "[start] python dependencies ok"
}

$browserJob = $null
try {
    Write-Host "[start] building frontend (web-studio)..."
    Set-Location (Join-Path $ProjectDir "web-studio")
    if (-not (Test-Path "node_modules")) {
        Invoke-Native "npm" @("install")
        if ($script:NativeExitCode -ne 0) {
            throw "Failed to install frontend dependencies."
        }
    }
    Invoke-Native "npm" @("run", "build")
    if ($script:NativeExitCode -ne 0) {
        throw "Failed to build frontend."
    }

    Write-Host "[start] launching Go web server on :$Port..."
    Set-Location (Join-Path $ProjectDir "go")

    $browserJob = Start-Job -ScriptBlock {
        param($Port, $Url)

        Write-Host "[start] waiting for worker warmup (model loading) ..."
        for ($i = 0; $i -lt 240; $i++) {
            try {
                $response = Invoke-WebRequest -UseBasicParsing -Uri "http://127.0.0.1:$Port/readyz" -TimeoutSec 2
                if ($response.Content -match '"ok"') {
                    Write-Host "[start] worker ready, opening $Url"
                    Start-Process $Url
                    return
                }
            } catch {
            }

            Start-Sleep -Milliseconds 500
        }

        Write-Host "[start] worker did not become ready in 120s; open $Url manually"
    } -ArgumentList $Port, $Url

    Invoke-Native "go" @("run", "./cmd/studio")
    if ($script:NativeExitCode -ne 0) {
        throw "Go server exited with code $script:NativeExitCode."
    }
} finally {
    if ($null -ne $browserJob -and $browserJob.State -eq "Running") {
        Stop-Job $browserJob | Out-Null
    }
    if ($null -ne $browserJob) {
        Receive-Job $browserJob -ErrorAction SilentlyContinue | ForEach-Object { Write-Host $_ }
        Remove-Job $browserJob -Force -ErrorAction SilentlyContinue
    }
}
