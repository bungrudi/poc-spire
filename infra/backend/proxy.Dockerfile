FROM scratch
COPY backend-money backend-client backend-proxy /
USER 1000:1000
ENTRYPOINT ["/backend-client"]
