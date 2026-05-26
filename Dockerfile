FROM golang:1.21 AS build
WORKDIR /src
COPY go.mod ./
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 GOOS=linux go build -o /out/hermesclaw ./cmd/hermesclaw

FROM gcr.io/distroless/static-debian12:nonroot
WORKDIR /app
COPY --from=build /out/hermesclaw /app/hermesclaw
EXPOSE 8080
ENTRYPOINT ["/app/hermesclaw"]
CMD ["serve"]

