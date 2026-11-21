FROM golang:1.27.1 AS build
WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -o /agoio

FROM gcr.io/distroless/static-debian12
WORKDIR /app
COPY --from=build /agoio .
COPY static ./static
EXPOSE 8090
CMD ["/app/agoio"]