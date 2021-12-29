$env:GOPATH=$PWD
$env:GO111MODULE="off"
$ret = $true
Set-Location src/raft
go test -v -run InitialElection$
$ret = $ret -and $?
go test -v -run ReElection$
$ret = $ret -and $?
Set-Location ../..
if (!$ret) {
    exit 1
}
else {
    echo "Passed Lab1"
}