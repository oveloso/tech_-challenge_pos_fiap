$ErrorActionPreference = 'Stop'
$projectRoot = Split-Path -Parent $PSScriptRoot
$envFile = Join-Path $projectRoot '.env'
if (Test-Path -LiteralPath $envFile) {
    Write-Output 'O arquivo .env já existe e foi preservado.'
    exit 0
}
function New-RandomHex([int]$Length) {
    $bytes = New-Object byte[] $Length
    $random = [System.Security.Cryptography.RandomNumberGenerator]::Create()
    try { $random.GetBytes($bytes) } finally { $random.Dispose() }
    return -join ($bytes | ForEach-Object { $_.ToString('x2') })
}
$settings = [ordered]@{
    POSTGRES_USER = 'oficina'
    POSTGRES_DB = 'oficina'
    POSTGRES_PASSWORD = New-RandomHex 24
    ADMIN_USERNAME = 'admin'
    ADMIN_PASSWORD = New-RandomHex 12
    JWT_SECRET = New-RandomHex 32
    DEMO_DATA = 'true'
    API_PORT = '8080'
    DB_PORT = '5432'
}
$content = ($settings.GetEnumerator() | ForEach-Object { "$($_.Key)=$($_.Value)" }) -join "`n"
[System.IO.File]::WriteAllText($envFile, $content + "`n")
Write-Output 'Arquivo .env criado. Consulte ADMIN_USERNAME e ADMIN_PASSWORD nele para autenticar na API.'
