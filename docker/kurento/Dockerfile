FROM kurento/kurento-media-server:7.1.1

COPY ov3endpoint_1.0.0ubuntu1_amd64.deb /tmp
COPY ov3endpoint-dbg_1.0.0ubuntu1_amd64.deb /tmp

RUN apt install -y /tmp/ov3endpoint_1.0.0ubuntu1_amd64.deb
RUN apt install -y /tmp/ov3endpoint-dbg_1.0.0ubuntu1_amd64.deb

RUN rm -f /tmp/ov3endpoint_1.0.0ubuntu1_amd64.deb && \
    rm -f /tmp/ov3endpoint-dbg_1.0.0ubuntu1_amd64.deb