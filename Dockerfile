FROM alpine:3.20
RUN apk add --no-cache ca-certificates tzdata
WORKDIR /app
# Prefer prebuilt linux/amd64 binary (Aliyun often cannot reach proxy.golang.org).
COPY waymate-api /app/waymate-api
COPY migrations /app/migrations
ENV MIGRATION_DIR=/app/migrations
EXPOSE 8080
USER nobody
ENTRYPOINT ["/app/waymate-api"]
