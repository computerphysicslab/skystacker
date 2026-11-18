module github.com/computerphysicslab/skystacker

go 1.27

toolchain go1.27.4

require (
	github.com/computerphysicslab/skystacker/goDiagnostics v0.0.0
	github.com/computerphysicslab/skystacker/goPureRaw v0.1.1
)

replace github.com/computerphysicslab/skystacker/goDiagnostics => ./goDiagnostics

replace github.com/computerphysicslab/skystacker/goPureRaw => ./goPureRaw
