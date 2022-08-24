FROM golang:1.18 as builder
COPY . /app
ENV GOOS=linux CGO_ENABLED=0
WORKDIR /app
RUN go build && ls -la

FROM alpine:3.16
COPY --from=builder /app/upgraderr /app/
WORKDIR /app
CMD /app/upgraderr
EXPOSE 6940