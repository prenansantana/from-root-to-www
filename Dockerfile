FROM golang:1.26-alpine AS build
WORKDIR /src
COPY go.mod main.go ./
RUN CGO_ENABLED=0 go build -o /root-to-www .

FROM scratch
COPY --from=build /root-to-www /root-to-www
EXPOSE 1234
ENTRYPOINT ["/root-to-www"]
