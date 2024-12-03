#We need to setup include and library flags for gstreamer:
#export CGO_CFLAGS=`pkg-config --cflags --libs  gstreamer-1.0`
set(GO_MODULE_PATH naevatec.com/ov3endpoint)
ADD_GO_SHARED_LIBRARY(ov3endpoint ov3subscriber.go gstreamer-1.0)