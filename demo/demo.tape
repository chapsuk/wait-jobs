## VHS tape for wait-jobs demo
## Re-render with: vhs demo/demo.tape

Output demo/demo.gif
Output demo/demo.mp4

Set Shell "bash"
Set FontSize 16
Set Width 1100
Set Height 600
Set Theme "Dracula"
Set TypingSpeed 40ms
Set PlaybackSpeed 1.0

# Reset namespace so the tape is idempotent
Hide
Type "kubectl delete -f demo/jobs.yaml --ignore-not-found >/dev/null 2>&1; kubectl create ns wait-jobs-demo --dry-run=client -o yaml | kubectl apply -f - >/dev/null; export PATH=$PWD/bin:$PATH; clear"
Enter
Sleep 2s
Show

Type "# Apply 3 Kubernetes Jobs and wait for them with wait-jobs"
Enter
Sleep 1s

Type "kubectl apply -f demo/jobs.yaml"
Sleep 500ms
Enter
Sleep 2s

Type "wait-jobs -n wait-jobs-demo -l app=migrations --logs failed --timeout 1m"
Sleep 1s
Enter

# Let the live table run; wait-jobs exits on its own
Sleep 25s

Type "echo exit code: $?"
Enter
Sleep 2s
