FROM registry.gitlab.com/yakshaving.art/dockerfiles/base:master

# hadolint ignore=DL3018
RUN apk --no-cache add libc6-compat

COPY hurrdurr /bin

CMD [ "/bin/hurrdurr" ]
