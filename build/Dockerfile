FROM alpine:latest

RUN apk add --no-cache ca-certificates

COPY govital /usr/local/bin/

ENTRYPOINT ["govital"]
CMD ["--help"]
