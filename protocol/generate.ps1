$ErrorActionPreference = "Stop"
Set-Location $PSScriptRoot
protoc -I . `
  --go_out=. --go_opt=paths=source_relative `
  --go-grpc_out=. --go-grpc_opt=paths=source_relative `
  judge/v1/judge.proto
Write-Host "generated protocol/judge/v1"
