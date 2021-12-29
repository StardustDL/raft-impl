$env:GOPATH=$PWD
$env:GO111MODULE="off"
Set-Location src/raft
go test -v -run InitialElection$
go test -v -run ReElection$
Set-Location ../..