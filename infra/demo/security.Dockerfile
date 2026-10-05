FROM scratch
COPY money security-client /
USER 1000:1000
ENTRYPOINT ["/security-client"]
