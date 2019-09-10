# Install PProf
# Install Python v3.73 - add Python\bin to System Path/Environment Variables
#     Restart Windows machines
# Install Graphiz - pip install graphiz
#     Optional: On error generating svg/pdf/etc. with error stating format svg or pdf is not recognized
#     Register Dot Plugins: dot -c
go test -bench=BenchmarkPublishAndConsumeMany -benchmem -memprofile mem.prof -cpuprofile cpu.prof
go tool pprof -svg cpu.prof > cpu.svg
go tool pprof -svg mem.prof > mem.svg