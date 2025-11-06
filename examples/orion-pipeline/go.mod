module github.com/e7canasta/orion-care-sensor/examples/orion-pipeline

go 1.23

require (
	github.com/e7canasta/orion-care-sensor/modules/framesupplier v0.1.0
	github.com/e7canasta/orion-care-sensor/modules/stream-capture v0.1.0
)

require (
	github.com/google/uuid v1.6.0 // indirect
	github.com/mattn/go-pointer v0.0.1 // indirect
	github.com/tinyzimmer/go-glib v0.0.25 // indirect
	github.com/tinyzimmer/go-gst v0.2.33 // indirect
)

replace (
	github.com/e7canasta/orion-care-sensor/modules/framesupplier => ../../modules/framesupplier
	github.com/e7canasta/orion-care-sensor/modules/stream-capture => ../../modules/stream-capture
)
