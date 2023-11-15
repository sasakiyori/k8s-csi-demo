FROM alpine:latest
WORKDIR /
COPY k8s-csi-demo /
ENTRYPOINT ["./k8s-csi-demo"]
