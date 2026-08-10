param (
    [switch]$Help
)

if ($Help) {
    Write-Host "Usage:"
    Write-Host "  .\run_tests.ps1                  - Run all integration tests"
    Write-Host "  .\run_tests.ps1 -Help            - Show this help message"
    Write-Host ""
    Write-Host "To run a specific test, use the go command directly:"
    Write-Host "  go test -v -tags=integration -run TestAtomicity_SingleDeposit ./tests/"
    exit
}

Write-Host "Running Wallet Service ACID Integration Tests..." -ForegroundColor Cyan
Write-Host "Make sure the service is running locally on http://localhost:8000" -ForegroundColor Yellow

# Run the tests with integration tags, count=1 disables test caching
go test -v -tags=integration -count=1 -timeout 120s ./tests/

if ($LASTEXITCODE -eq 0) {
    Write-Host "All tests passed successfully!" -ForegroundColor Green
} else {
    Write-Host "Some tests failed. Check the output above." -ForegroundColor Red
}
