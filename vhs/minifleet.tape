Output minifleet.gif
Set FontSize 20
Set Margin 10
Set MarginFill "#674EFF"
Set BorderRadius 10
Set Width 1500
Set Height 1000
Set Shell fish


Type "# Discover repositories using GitHub's API"
Enter

Hide
Type "minifleet discover depado --visibility public"
Show
Enter
Sleep 3s

Type "# Sync the discovered repositories locally"
Enter

Hide
Type "minifleet sync depado"
Show
Enter
Sleep 10s

Type "# Operate on repositories in parallel"
Enter

Hide
Type "minifleet run -i smallblog -i quokka -i minifleet -i parakeet -- 'go test ./... && golangci-lint run --allow-parallel-runners && make'"
Show
Enter
Sleep 10s
