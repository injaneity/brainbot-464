# BrainBot Demo Runner (PowerShell version for Windows)
# Usage: .\run-demo.ps1

$ErrorActionPreference = "Stop"

# Colors
function Write-ColorOutput($ForegroundColor) {
    $fc = $host.UI.RawUI.ForegroundColor
    $host.UI.RawUI.ForegroundColor = $ForegroundColor
    if ($args) {
        Write-Output $args
    }
    $host.UI.RawUI.ForegroundColor = $fc
}

Write-ColorOutput Blue "â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”"
Write-ColorOutput Blue "â”ƒ      ðŸ¤– BrainBot Demo Runner          â”ƒ"
Write-ColorOutput Blue "â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”â”"
Write-Output ""

# Check for Docker
try {
    $null = docker --version
    $null = docker compose version
} catch {
    Write-ColorOutput Red "Docker or Docker Compose not found!"
    Write-ColorOutput Yellow "Please install Docker Desktop"
    exit 1
}

Write-Output ""

$SERVICES_STARTED = $false

function Cleanup {
    Write-Output ""
    Write-ColorOutput Yellow "Cleaning up services..."
    if ($SERVICES_STARTED) {
        docker compose down
    }
    Write-ColorOutput Green "Cleanup complete"
}

# Setup cleanup on Ctrl+C
$null = Register-EngineEvent -SourceIdentifier PowerShell.Exiting -Action { Cleanup }

# Check credentials
$CREATION_ENV_FILE = "creation_service\.secrets\youtube.env"
if (-not (Test-Path $CREATION_ENV_FILE)) {
    Write-ColorOutput Red "Missing $CREATION_ENV_FILE"
    Write-ColorOutput Yellow "Run: creation_service\scripts\setup_creation_service_credentials.sh"
    exit 1
}

$GEN_ENV_FILE = "generation_service\.env"

if (-not (Test-Path $GEN_ENV_FILE)) {
    Write-ColorOutput Red "Missing $GEN_ENV_FILE"
    Write-ColorOutput Yellow "Create it with GOOGLE_API_KEY and FAL_KEY, e.g.:"
    Write-Output "GOOGLE_API_KEY=your-gemini-api-key"
    Write-Output "FAL_KEY=your-fal-key"
    exit 1
}

$genEnvContent = Get-Content $GEN_ENV_FILE -Raw
if (-not ($genEnvContent -match '(?m)^GOOGLE_API_KEY=') -or -not ($genEnvContent -match '(?m)^FAL_KEY=')) {
    Write-ColorOutput Yellow "Warning: GOOGLE_API_KEY or FAL_KEY not set in $GEN_ENV_FILE"
    $response = Read-Host "Continue? (y/n)"
    if ($response -notmatch '^[Yy]$') {
        exit 1
    }
}

$ROOT_ENV_FILE = ".env"
if (-not (Test-Path $ROOT_ENV_FILE)) {
    Write-ColorOutput Red "Missing root .env file"
    Write-ColorOutput Yellow "Please create .env with S3 and Redis configuration (see .env.example)"
    exit 1
}

$rootEnvContent = Get-Content $ROOT_ENV_FILE -Raw
if (-not ($rootEnvContent -match '(?m)^S3_BUCKET=')) {
    Write-ColorOutput Yellow "Warning: S3_BUCKET not set in .env"
    $response = Read-Host "Continue? (y/n)"
    if ($response -notmatch '^[Yy]$') {
        exit 1
    }
}

# Load creation service environment variables
Get-Content $CREATION_ENV_FILE | ForEach-Object {
    if ($_ -match '^\s*([^#][^=]*)\s*=\s*(.*)$') {
        [Environment]::SetEnvironmentVariable($matches[1].Trim(), $matches[2].Trim(), "Process")
    }
}

# Check if orchestrator is already running
$ORCHESTRATOR_RUNNING = docker ps -q -f name=brainbot-orchestrator 2>$null

if ($ORCHESTRATOR_RUNNING) {
    Write-ColorOutput Green "âœ“ Orchestrator already running"
} else {
    Write-ColorOutput Blue "Building and starting services..."
    docker compose up -d --build
    $SERVICES_STARTED = $true
    Write-Output ""

    function Wait-ForService {
        param(
            [string]$Url,
            [string]$Name,
            [int]$MaxAttempts = 60
        )
        
        Write-ColorOutput Yellow "Waiting for $Name..."
        $attempt = 0
        while ($attempt -lt $MaxAttempts) {
            try {
                $response = Invoke-WebRequest -Uri $Url -UseBasicParsing -TimeoutSec 2 -ErrorAction Stop
                if ($response.StatusCode -eq 200) {
                    Write-ColorOutput Green "âœ“ $Name ready"
                    return $true
                }
            }
            catch {
                # Service not ready yet
            }
            $attempt++
            Start-Sleep -Seconds 2
        }
        Write-ColorOutput Red "âœ— $Name timeout"
        return $false
    }

    if (-not (Wait-ForService "http://localhost:8090" "Kafka UI")) { exit 1 }
    if (-not (Wait-ForService "http://localhost:8000/api/v2/heartbeat" "ChromaDB")) { exit 1 }
    Wait-ForService "http://localhost:8002/health" "Generation" | Out-Null
    if (-not (Wait-ForService "http://localhost:8080/api/health" "API")) { exit 1 }
    if (-not (Wait-ForService "http://localhost:8081/health" "Orchestrator")) { exit 1 }
}

# Run the TUI client
$env:ORCHESTRATOR_URL = "http://localhost:8081"

Write-ColorOutput Blue "Building TUI client..."
if (-not (Test-Path "bin")) {
    New-Item -ItemType Directory -Path "bin" | Out-Null
}
go build -o bin\demo-client.exe demo\main.go

Write-ColorOutput Green "Starting TUI client..."
Write-Output ""

$EXIT_CODE = 0
& .\bin\demo-client.exe --url="$env:ORCHESTRATOR_URL"
$EXIT_CODE = $LASTEXITCODE

if ($EXIT_CODE -eq 10) {
    Write-ColorOutput Yellow "Shutdown requested..."
    docker compose down
    Write-ColorOutput Green "Services stopped"
    exit 0
}

# After TUI exits, services remain running (no automatic cleanup)
Write-Output ""
Write-ColorOutput Green "TUI client exited"
Write-ColorOutput Yellow "Orchestrator is still running in the background"
Write-ColorOutput Yellow "Run this script again to reconnect, or use 'docker compose down' to stop all services"
