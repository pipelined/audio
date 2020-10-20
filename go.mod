module pipelined.dev/audio

require (
	pipelined.dev/pipe v0.8.5-0.20201002195308-e9a6002f11e0
	pipelined.dev/signal v0.8.1-0.20200923112724-a0f620a428a7
)

go 1.13

replace (
	pipelined.dev/pipe => ../pipe
)
