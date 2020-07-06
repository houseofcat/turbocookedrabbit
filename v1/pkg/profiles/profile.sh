# Install PProf
# Install Python v3.73 - add Python\bin to System Path/Environment Variables
#     Restart Windows machines
# Update PIP
#     python -m pip install --upgrade pip
# Install Graphiz - pip install graphiz is not working
#     Visit the Graphiz AppVeyor CI/CD: https://ci.appveyor.com/project/ellson/graphviz-pl238
#       Click a job, and pull the MSI from Artifcats.
#     Optional: On error generating svg/pdf/etc. with error stating format svg or pdf is not recognized
#     Register Dot Plugins: dot -c
# Example: go test -bench=BenchmarkPublishAndConsumeMany -benchmem -memprofile mem.prof -cpuprofile cpu.prof
go test -benchmem -run=^$ -bench "^(BenchmarkPublishConsumeAckForDuration)$" -timeout 6m -memprofile memDuration.prof -cpuprofile cpuDuration.prof
go tool pprof -svg cpuDuration.prof > cpuDuration.svg
go tool pprof -svg memDuration.prof > memDuration.svg

go test -benchmem -run=^$ -bench "^(BenchmarkGetChannelAndPublish)$" -memprofile ".\profiles\memGetChannel.prof" -cpuprofile ".\profiles\cpuGetChannel.prof"
go tool pprof -svg ".\profiles\cpuGetChannel.prof" > ".\profiles\cpuGetChannel.svg"
go tool pprof -svg ".\profiles\memGetChannel.prof" > ".\profiles\memGetChannel.svg"

#go test -benchmem -run=^$ -bench "^(BenchmarkGetChannelAndPublish)$" -memprofile ".\profiles\memGetChannel.prof" -cpuprofile ".\profiles\cpuGetChannel.prof"