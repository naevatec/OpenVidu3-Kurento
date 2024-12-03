# See here for image contents: https://github.com/microsoft/vscode-dev-containers/tree/v0.194.0/containers/cpp/.devcontainer/base.Dockerfile

# [Choice] Debian / Ubuntu version: debian-11, debian-10, debian-9, ubuntu-20.04, ubuntu-18.04
ARG VARIANT="noble"
FROM mcr.microsoft.com/devcontainers/base:${VARIANT}


# [Optional] Uncomment this section to install additional packages.
# RUN apt-get update && export DEBIAN_FRONTEND=noninteractive \
#     && apt-get -y install --no-install-recommends <your-package-list-here>
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

RUN apt install -y equivs

ENV NODE_OPTIONS="--dns-result-order=ipv4first"
    
RUN curl -sL https://deb.nodesource.com/setup_18.x | bash -
    
RUN apt install -y nodejs
    
RUN npm install -g bower

RUN apt install -y wget

RUN wget -O /tmp/go1.23.3.linux-amd64.tar.gz https://go.dev/dl/go1.23.3.linux-amd64.tar.gz

RUN rm -rf /usr/local/go && tar -C /usr/local -xzf /tmp/go1.23.3.linux-amd64.tar.gz && rm -f /tmp/go1.23.3.linux-amd64.tar.gz

ENV PATH=$PATH:/usr/local/go/bin


    
    