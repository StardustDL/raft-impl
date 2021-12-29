$env:GOPATH=$PWD
$env:GO111MODULE="off"
$ret = $true
Set-Location src/raft
go test -v -run Persist1$
$ret = $ret -and $?
go test -v -run Persist2$
$ret = $ret -and $?
go test -v -run Persist3$
$ret = $ret -and $?
Set-Location ../..
if (!$ret) {
    exit 1
}
else {
    echo "Passed Lab3"
}