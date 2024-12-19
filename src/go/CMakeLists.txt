
set(LK_GO_ENDPOINT_SOURCES
  appwriter.go
  ov3endpoint.go
  ov3ingress.go
  ov3publisher.go
  ov3room.go
  ov3root.go
  ov3service.go
  ov3subscriber.go
  ov3subscription.go
  ov3trackpublisher.go
)

set(LK_GO_ENDPOINT_DEPENDENCIES 
    gstreamer-1.0
)

set(OUTPUTPATH "${CMAKE_CURRENT_BINARY_DIR}/../gst-plugins/ov3endpoint")

set(GO_MODULE_PATH naevatec.com/ov3endpoint)
ADD_GO_SHARED_LIBRARY(ov3endpoint "${OUTPUTPATH}" "${LK_GO_ENDPOINT_SOURCES}" "${LK_GO_ENDPOINT_DEPENDENCIES}")