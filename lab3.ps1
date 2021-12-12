$env:GOPATH=$PWD
$env:GO111MODULE="off"
cd src/raft
go test -run Persist1
go test -run Persist2
go test -run Persist3
cd ../..