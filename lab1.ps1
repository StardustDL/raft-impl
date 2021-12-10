$env:GOPATH=$PWD
$env:GO111MODULE="auto"
cd src/raft
go test -run Election
cd ../..