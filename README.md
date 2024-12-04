# OpenVidu3-Kurento

This repository contains the components to integrate [OpenVidu 3](https://openvidu.io) with [Kurento](https://kurento.openvidu.io), the main objective is to provide the capability of adding media processing capabilities to OpenVidu via Kurento. The first goal is to provide the OpenVidu filtering API to OpenVidu 3.

It consists basically of a Kurento module: ov3endpoint. This module provides two elements:
- Ov3Subscriber. This element allows joining to an existent OpenVidu 3 session and subscribe to a participant stream with real time and low latency capabilities.
- Ov3Publisher. This elements allows joining to an existent OpenVidu 3 session to publish some media stream like any other participant stream. Again with real time and low latency capabilities.

## Ov3Endpoint

### Building
Pre-requisites
- You need to access kurento ci scripts, specifically [kurento-buildpackage.sh](https://github.com/Kurento/kurento/blob/7.1.1/ci-scripts/kurento-buildpackage.sh) script. This is needed to build the .deb adrtifact for the module
- [GoLang](https://go.dev/) this is needed to build the go parts of the module.

For building the artifact, just type:
```sh
kurento-buildpackage --release
```
Then you can deploy it over a Kurento 7.x installation just by executing:
```sh
apt install -y ov3endpoint_1.0.0ubuntu1_amd64.deb
```
Or if you want to build a docker image just issue:
```sh
cd docker/kurento
docker build -t kurento/kurento-media-server:7.1.1-ov3 .
```
