$env:GOPATH=$PWD
$env:GO111MODULE="off"
Set-Location src/raft
go test -v -run BasicAgree$
go test -v -run FailAgree$
go test -v -run FailNoAgree$
go test -v -run ConcurrentStarts$
go test -v -run Rejoin$
go test -v -run Backup$
go test -v -run Count$
Set-Location ../..