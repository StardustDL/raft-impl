$env:GOPATH=$PWD
$env:GO111MODULE="auto"
cd src/raft
go test -run FailNoAgree
go test -run ConcurrentStarts
go test -run Rejoin
go test -run Backup
cd ../..