# Install PProf
# Install Python v3.73 - add Python\bin to System Path/Environment Variables
#     Restart Windows machines
# Install Graphiz - pip install graphiz
#     Optional: On error generating svg/pdf/etc. with error stating format svg or pdf is not recognized
#     Register Dot Plugins: dot -c
# Example: go test -bench=BenchmarkPublishAndConsumeMany -benchmem -memprofile mem.prof -cpuprofile cpu.prof
go test -benchmem -run=^$ -bench "^(BenchmarkPublishConsumeAckForDuration)$" -timeout 6m -memprofile memDuration.prof -cpuprofile cpuDuration.prof
go tool pprof -svg cpuDuration.prof > cpuDuration.svg
go tool pprof -svg memDuration.prof > memDuration.svg