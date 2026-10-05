FROM scratch
COPY backend-money backend-client backend-client-v2 /
USER 1000:1000
ENTRYPOINT ["/backend-client"]
