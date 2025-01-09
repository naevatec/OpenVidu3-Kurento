FROM ubuntu:noble AS build

RUN apt update && apt install -y --no-install-recommends \
    build-essential \
    ca-certificates \
    cmake \
    git \
    gnupg \
    gdb

RUN apt-key adv --keyserver keyserver.ubuntu.com --recv-keys 5AFA7A83

COPY repo_cfg /tmp

RUN bash -c "source /etc/upstream-release/lsb-release 2>/dev/null || source /etc/lsb-release "
RUN tee "/etc/apt/sources.list.d/kurento.list" >/dev/null < /tmp/repo_cfg \
    && rm /tmp/repo_cfg && apt update

RUN apt install -y --no-install-recommends \
    kurento-media-server-dev

RUN apt install -y equivs devscripts wget
    
RUN wget -O /tmp/go1.23.3.linux-amd64.tar.gz https://go.dev/dl/go1.23.3.linux-amd64.tar.gz

RUN rm -rf /usr/local/go && tar -C /usr/local -xzf /tmp/go1.23.3.linux-amd64.tar.gz && rm -f /tmp/go1.23.3.linux-amd64.tar.gz

ENV PATH=$PATH:/usr/local/go/bin

WORKDIR /app

COPY --from=sources ./cmake cmake
COPY --from=sources ./debian debian
COPY --from=sources ./src src
COPY --from=sources ./tests tests
COPY --from=sources ./CMakeLists.txt .
COPY --from=sources ./config.h.cmake .
COPY --from=sources ./README.md .


RUN wget -O /app/kurento-buildpackage.sh https://github.com/Kurento/kurento/raw/refs/tags/7.1.1/ci-scripts/kurento-buildpackage.sh  && chmod a+x /app/kurento-buildpackage.sh
RUN wget -O /app/bash.conf.sh https://github.com/Kurento/kurento/raw/refs/tags/7.1.1/ci-scripts/bash.conf.sh  && chmod a+x /app/bash.conf.sh

RUN ./kurento-buildpackage.sh --release


FROM kurento/kurento-media-server:7.1.1

COPY --from=build /app/ov3endpoint_1.0.0ubuntu1_amd64.deb /tmp
COPY --from=build /app/ov3endpoint-dbg_1.0.0ubuntu1_amd64.deb /tmp

RUN apt install -y /tmp/ov3endpoint_1.0.0ubuntu1_amd64.deb
RUN apt install -y /tmp/ov3endpoint-dbg_1.0.0ubuntu1_amd64.deb

RUN rm -f /tmp/ov3endpoint_1.0.0ubuntu1_amd64.deb && \
    rm -f /tmp/ov3endpoint-dbg_1.0.0ubuntu1_amd64.deb