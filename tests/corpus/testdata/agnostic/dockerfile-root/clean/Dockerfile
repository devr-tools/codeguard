FROM alpine:3.20
# USER root would run the workload as root; create an app user instead.
RUN adduser -D app
USER app
CMD ["/bin/sh"]
