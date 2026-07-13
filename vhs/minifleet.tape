Output minifleet.gif
Set Width 1500
Set Height 1000
Set Shell fish

Hide
Type "minifleet sync depado --visibility public"
Show
Enter
Sleep 10s
Hide
Type "minifleet run --language go --visibility public --include smallblog --include quokka --include minifleet --include parakeet -- 'go test ./... && golangci-lint run --allow-parallel-runners && make'"
Show
Enter
Sleep 10s
