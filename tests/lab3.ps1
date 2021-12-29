$env:GOPATH=$PWD
$env:GO111MODULE="off"
Set-Location src/raft
go test -v -run Persist1$
go test -v -run Persist2$
go test -v -run Persist3$
Set-Location ../..