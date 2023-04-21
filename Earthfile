VERSION 0.6
FROM alpine

ARG BASE_IMAGE=quay.io/kairos/core-ubuntu-20-lts:v2.0.2
ARG IMAGE_REPOSITORY=quay.io/kairos

ARG LUET_VERSION=0.32.4
ARG GOLINT_VERSION=v1.46.2
ARG GOLANG_VERSION=1.18

ARG MICROK8S_CHANNEL=latest
ARG BASE_IMAGE_NAME=$(echo $BASE_IMAGE | grep -o [^/]*: | rev | cut -c2- | rev)
ARG BASE_IMAGE_TAG=$(echo $BASE_IMAGE | grep -o :.* | cut -c2-)

build-cosign:
    FROM gcr.io/projectsigstore/cosign:v1.13.1
    SAVE ARTIFACT /ko-app/cosign cosign

go-deps:
    FROM golang:$GOLANG_VERSION
    WORKDIR /build
    COPY go.mod  ./
    RUN go mod download
    RUN apt-get update && apt-get install -y upx
    SAVE ARTIFACT go.mod AS LOCAL go.mod
    SAVE ARTIFACT go.sum AS LOCAL go.sum

BUILD_GOLANG:
    COMMAND
    WORKDIR /build
    COPY . ./
    ARG BIN
    ARG SRC

    RUN go build -ldflags "-s -w" -o ${BIN} ./${SRC} && upx ${BIN}
    SAVE ARTIFACT ${BIN} ${BIN} AS LOCAL build/${BIN}

VERSION:
    COMMAND
    FROM alpine
    RUN apk add git

    COPY . ./

    RUN echo $(git describe --exact-match --tags || echo "v0.0.0-$(git log --oneline -n 1 | cut -d" " -f1)") > VERSION

    SAVE ARTIFACT VERSION VERSION

build-provider:
    FROM +go-deps
    DO +BUILD_GOLANG --BIN=agent-provider-microk8s --SRC=.

lint:
    FROM golang:$GOLANG_VERSION
    RUN wget -O- -nv https://raw.githubusercontent.com/golangci/golangci-lint/master/install.sh | sh -s $GOLINT_VERSION
    WORKDIR /build
    COPY . .
    RUN golangci-lint run

docker:
    DO +VERSION
    ARG VERSION=$(cat VERSION)

    FROM $BASE_IMAGE
    RUN apt-get update && apt-get autoclean  && DEBIAN_FRONTEND=noninteractive apt-get install iptables-persistent -y
    RUN snap download microk8s --channel=$MICROK8S_CHANNEL --target-directory /opt/microk8s/snaps --basename microk8s
    RUN snap download core  --target-directory /opt/microk8s/snaps --basename core
#   RUN export MICROK8S_VERSION_TAG=`snap info /opt/microk8s/snaps/microk8s.snap |grep version: |sed -e 's/^version:.*v/v/g' |awk '{ print $1}'`

    COPY +build-provider/agent-provider-microk8s /system/providers/agent-provider-microk8s
    COPY scripts/cloudinit /opt/microk8s/scripts
 #  COPY overlay/files/system/oem/* /system/oem/
    RUN chmod +x /opt/microk8s/scripts/*
    ENV OS_ID=${BASE_IMAGE_NAME}-microk8s
    ENV OS_NAME=$OS_ID:${BASE_IMAGE_TAG}
    ENV OS_REPO=${IMAGE_REPOSITORY}
    ENV OS_VERSION=v${MICROK8S_CHANNEL}_${VERSION}
    ENV OS_LABEL=${BASE_IMAGE_TAG}_v${MICROK8S_CHANNEL}_${VERSION}
    RUN envsubst >>/etc/os-release </usr/lib/os-release.tmpl

    SAVE IMAGE --push $IMAGE_REPOSITORY/${BASE_IMAGE_NAME}-microk8s:v${MICROK8S_CHANNEL}
    SAVE IMAGE --push $IMAGE_REPOSITORY/${BASE_IMAGE_NAME}-microk8s:v${MICROK8S_CHANNEL}_${VERSION}

cosign:
    ARG --required ACTIONS_ID_TOKEN_REQUEST_TOKEN
    ARG --required ACTIONS_ID_TOKEN_REQUEST_URL

    ARG --required REGISTRY
    ARG --required REGISTRY_USER
    ARG --required REGISTRY_PASSWORD

    DO +VERSION
    ARG VERSION=$(cat VERSION)

    FROM docker

    ENV ACTIONS_ID_TOKEN_REQUEST_TOKEN=${ACTIONS_ID_TOKEN_REQUEST_TOKEN}
    ENV ACTIONS_ID_TOKEN_REQUEST_URL=${ACTIONS_ID_TOKEN_REQUEST_URL}

    ENV REGISTRY=${REGISTRY}
    ENV REGISTRY_USER=${REGISTRY_USER}
    ENV REGISTRY_PASSWORD=${REGISTRY_PASSWORD}

    ENV COSIGN_EXPERIMENTAL=1
    COPY +build-cosign/cosign /usr/local/bin/

    RUN echo $REGISTRY_PASSWORD | docker login -u $REGISTRY_USER --password-stdin $REGISTRY

    RUN cosign sign $IMAGE_REPOSITORY/${BASE_IMAGE_NAME}-microk8s:v${MICROK8S_CHANNEL}
    RUN cosign sign $IMAGE_REPOSITORY/${BASE_IMAGE_NAME}-microk8s:v${MICROK8S_CHANNEL}_${VERSION}
