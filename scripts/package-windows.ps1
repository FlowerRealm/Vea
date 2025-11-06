Param(
    [Parameter(Mandatory = $true)]
    [string] $Version,
    [Parameter(Mandatory = $true)]
    [string] $GoOS,
    [Parameter(Mandatory = $true)]
    [string] $GoArch
)

$ErrorActionPreference = 'Stop'

$artifact = "vea-$Version-$GoOS-$GoArch"
$buildDir = Join-Path "dist" $artifact
$output = Join-Path "dist" "$artifact.zip"

Remove-Item $buildDir -Recurse -Force -ErrorAction SilentlyContinue
Remove-Item $output -Force -ErrorAction SilentlyContinue
New-Item $buildDir -ItemType Directory | Out-Null

$env:GOOS = $GoOS
$env:GOARCH = $GoArch
$env:CGO_ENABLED = "0"

go build -trimpath -ldflags "-s -w" -o (Join-Path $buildDir "vea.exe") ./cmd/server

if (Test-Path "web") {
    Copy-Item "web" (Join-Path $buildDir "web") -Recurse
}

if (Test-Path "LICENSE") {
    Copy-Item "LICENSE" (Join-Path $buildDir "LICENSE")
}

Push-Location dist
try {
    Compress-Archive -Path $artifact -DestinationPath (Split-Path -Path $output -Leaf) -Force
} finally {
    Pop-Location
}

Remove-Item $buildDir -Recurse -Force
