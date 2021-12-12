$env:GOPATH=$PWD
$env:GO111MODULE="off"
cd src/raft
go test -run Election
cd ../..