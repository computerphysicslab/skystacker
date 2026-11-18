module github.com/computerphysicslab/skystacker

go 1.27

toolchain go1.27.4

require (
	github.com/computerphysicslab/skystacker/goDiagnostics v0.0.0
	github.com/computerphysicslab/skystacker/goPureRaw v0.1.1
	github.com/skip2/go-qrcode v0.0.0-20200617195104-da1b6568686e
)

replace github.com/computerphysicslab/skystacker/goDiagnostics => ./goDiagnostics

replace github.com/computerphysicslab/skystacker/goPureRaw => ./goPureRaw
