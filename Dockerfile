FROM registry.gitlab.com/yakshaving.art/dockerfiles/base:master

# hadolint ignore=DL3018
RUN apk --no-cache add libc6-compat && ( apk -qq --no-cache add py3-pip \
		&& pip3 install yq==2.10.1 \
		&& apk -qq del py3-pip \
		&& rm -rf /root/.cache )


COPY hurrdurr /bin

CMD [ "/bin/hurrdurr" ]
