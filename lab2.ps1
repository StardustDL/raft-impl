$env:GOPATH=$PWD
$env:GO111MODULE="auto"
cd src/raft
go test -run BasicAgree
go test -run FailAgree
go test -run FailNoAgree
go test -run ConcurrentStarts
go test -run Rejoin
go test -run Backup
go test -run Count
cd ../..