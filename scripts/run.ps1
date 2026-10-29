param(
    [ValidateSet('api', 'seed', 'test')][string]$Action = 'api',
    [string]$DatabaseHost = '127.0.0.1',
    [int]$DatabasePort = 0,
    [string]$DatabaseName = ''
)
$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$settings = @{}
Get-Content -LiteralPath (Join-Path $projectRoot '.env') | ForEach-Object {
    if ($_ -match '^([A-Z_]+)=(.*)$') { $settings[$matches[1]] = $matches[2] }
}
if (-not (Get-Command go -ErrorAction SilentlyContinue)) {
    throw 'Instale Go 1.26 ou superior e adicione go ao PATH.'
}
if ($DatabasePort -eq 0) { $DatabasePort = [int]$settings.DB_PORT }
if ($DatabaseName -eq '') {
    $DatabaseName = $settings.POSTGRES_DB
    if ($Action -eq 'test') { $DatabaseName += '_test' }
}
$dbUser = [Uri]::EscapeDataString($settings.POSTGRES_USER)
$dbPassword = [Uri]::EscapeDataString($settings.POSTGRES_PASSWORD)
$env:DATABASE_URL = "postgres://${dbUser}:${dbPassword}@${DatabaseHost}:${DatabasePort}/${DatabaseName}?sslmode=disable"
$env:JWT_SECRET = $settings.JWT_SECRET
$env:ADMIN_USERNAME = $settings.ADMIN_USERNAME
$env:ADMIN_PASSWORD = $settings.ADMIN_PASSWORD
$env:DEMO_DATA = $settings.DEMO_DATA
$env:HOST = '127.0.0.1'
$env:PORT = $settings.API_PORT
$env:OPENAPI_PATH = Join-Path $projectRoot 'docs/api/openapi.yaml'
$env:CORS_ORIGINS = "http://localhost:$($env:PORT),http://127.0.0.1:$($env:PORT)"
Push-Location (Join-Path $projectRoot 'backend')
try {
    if ($Action -eq 'test') {
        $env:TEST_DATABASE_URL = $env:DATABASE_URL
        go test -count=1 '-coverpkg=./...' '-coverprofile=coverage.out' ./...
    } else {
        go run "./cmd/$Action"
    }
    if ($LASTEXITCODE -ne 0) { throw "Falha ao executar $Action." }
} finally { Pop-Location }
