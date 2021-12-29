$env:GOPATH=$PWD
$env:GO111MODULE="off"
$ret = $true
Set-Location src/raft
go test -v -run BasicAgree$
$ret = $ret -and $?
go test -v -run FailAgree$
$ret = $ret -and $?
go test -v -run FailNoAgree$
$ret = $ret -and $?
go test -v -run ConcurrentStarts$
$ret = $ret -and $?
go test -v -run Rejoin$
$ret = $ret -and $?
go test -v -run Backup$
$ret = $ret -and $?
go test -v -run Count$
$ret = $ret -and $?
Set-Location ../..
if (!$ret) {
    exit 1
}
else {
    echo "Passed Lab2"
}