$env:GOPATH=$PWD
$env:GO111MODULE="off"
$ret = $true
Set-Location src/raft
go test -v
$ret = $ret -and $?
Set-Location ../..
if (!$ret) {
    exit 1
}
else {
    echo "Passed All"
}