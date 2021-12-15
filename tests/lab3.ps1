$env:GOPATH=$PWD
$env:GO111MODULE="off"
Set-Location src/raft
go test -run Persist
Set-Location ../..