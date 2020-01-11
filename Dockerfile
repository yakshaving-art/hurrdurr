FROM alpine:3.10

RUN apk --no-cache add ca-certificates libc6-compat

COPY hurrdurr /bin
 
CMD [ "/bin/hurrdurr" ]
