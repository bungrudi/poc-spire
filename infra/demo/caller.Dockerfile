FROM scratch
COPY money trusted-caller untrusted-caller security-client control-init control-client /
USER 1000:1000
ENTRYPOINT ["/control-client"]
