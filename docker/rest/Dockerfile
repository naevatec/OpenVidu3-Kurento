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
    
RUN wget https://download.java.net/java/GA/jdk22.0.1/c7ec1332f7bb44aeba2eb341ae18aca4/8/GPL/openjdk-22.0.1_linux-x64_bin.tar.gz

RUN tar xvf openjdk-22.0.1_linux-x64_bin.tar.gz

RUN mv jdk-22.0.1 /usr/lib/jvm

RUN update-alternatives --install /usr/bin/javac javac /usr/lib/jvm/jdk-22.0.1/bin/javac 2211
RUN update-alternatives --install /usr/bin/java java /usr/lib/jvm/jdk-22.0.1/bin/java 2211

RUN rm openjdk-22.0.1_linux-x64_bin.tar.gz

WORKDIR /app

COPY --from=sources ./cmake cmake
COPY --from=sources ./src src
COPY --from=sources ./tests tests
COPY --from=sources ./CMakeLists.txt .
COPY --from=sources ./config.h.cmake .
COPY --from=sources ./rest rest

RUN cmake -DCMAKE_BUILD_TYPE=Release -DGENERATE_JAVA_CLIENT_PROJECT=TRUE .
RUN make maven_install
RUN cd rest && mvn install package



FROM ubuntu:noble

COPY --from=build /app/rest/target/ov3-kurento-filters-1.0.0.jar /

RUN apt-get update && apt-get install -y openjdk-11-jre && \
    apt-get -y install dnsutils


EXPOSE 8080/tcp

ENTRYPOINT java -jar /ov3-kurento-filters-1.0.0.jar
